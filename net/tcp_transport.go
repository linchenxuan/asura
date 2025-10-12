package net

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/lcx/asura/config"
	"github.com/lcx/asura/log"
	"github.com/lcx/asura/metrics"
	"github.com/lcx/asura/tracing"
)

// 修改后
type TCPTransportCfg struct {
	Tag             string `mapstructure:"tag"`
	IdleTimeout     uint32 `mapstructure:"idleTimeout"`
	Crypt           uint32 `mapstructure:"crypt"`
	Addr            string `mapstructure:"addr"`
	ConnType        string `mapstructure:"connType"`
	FrameMetaKey    string `mapstructure:"frameMetaKey"`
	SendChannelSize uint32 `mapstructure:"sendChannelSize"`
	MaxBufferSize   int    `mapstructure:"maxBufferSize"`
}

// GetName returns the configuration name for TCPTransportCfg
func (c *TCPTransportCfg) GetName() string {
	return "tcp_transport"
}

// Validate validates the TCPTransportCfg parameters
func (c *TCPTransportCfg) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("Addr cannot be empty")
	}
	if c.MaxBufferSize <= 0 {
		return fmt.Errorf("MaxBufferSize must be positive")
	}
	if c.SendChannelSize <= 0 {
		return fmt.Errorf("SendChannelSize must be positive")
	}
	if c.IdleTimeout <= 0 {
		return fmt.Errorf("IdleTimeout must be positive")
	}
	return nil
}

// TCPTransport a transport plugin based on tcp, ervey connection will be alloc a goroutine.
type TCPTransport struct {
	*TCPTransportCfg
	uidToConn map[uint64]*tcpctx
	lock      sync.RWMutex
	receiver  DispatcherReceiver
	creator   MsgCreator
	cancel    context.CancelFunc
}

// NewTCPTransportWithConfigManager creates a TCPTransport that supports configuration hot-reload.
// This constructor initializes the transport with configuration from the config manager
// and registers it as a configuration change listener for dynamic updates.
func NewTCPTransportWithConfigManager(configManager config.ConfigManager) (*TCPTransport, error) {
	if configManager == nil {
		return nil, errors.New("configManager cannot be nil")
	}

	// Load configuration from config manager
	cfg := &TCPTransportCfg{}
	if err := configManager.LoadConfig("tcp_transport", cfg); err != nil {
		return nil, fmt.Errorf("failed to load tcp_transport config: %w", err)
	}

	transport := &TCPTransport{
		TCPTransportCfg: cfg,
		uidToConn:       make(map[uint64]*tcpctx),
		lock:            sync.RWMutex{},
	}

	// Register as configuration change listener
	configManager.AddChangeListener(transport)

	return transport, nil
}

// OnConfigChanged implements the ConfigChangeListener interface for TCPTransport.
// This method is called when the TCP transport configuration is updated in the config manager.
// It handles dynamic updates to transport settings without requiring service restart.
func (t *TCPTransport) OnConfigChanged(configName string, newConfig, oldConfig config.Config) error {
	if configName != "tcp_transport" {
		return nil
	}

	newCfg, ok := newConfig.(*TCPTransportCfg)
	if !ok {
		return fmt.Errorf("invalid configuration type for TCPTransport")
	}

	// Validate the new configuration
	if err := newCfg.Validate(); err != nil {
		return fmt.Errorf("invalid TCP transport configuration: %w", err)
	}

	// Update configuration atomically
	t.lock.Lock()
	defer t.lock.Unlock()

	// Update configuration fields
	t.TCPTransportCfg = newCfg

	log.Info().Str("configName", configName).Msg("TCP transport configuration updated successfully")
	return nil
}

// GetConfigName implements the ConfigChangeListener interface for TCPTransport.
// Returns the configuration name that this listener is interested in.
func (t *TCPTransport) GetConfigName() string {
	return "tcp_transport"
}

func NewTCPTransport() (*TCPTransport, error) {
	return nil, errors.New("TCPTransportCfg cannot be nil, use NewTCPTransportWithConfig or NewTCPTransportWithConfigManager for dynamic configuration")
}

// NewTCPTransportWithConfig creates a TCPTransport with the provided configuration.
// This constructor allows for explicit configuration of the transport instance.
func NewTCPTransportWithConfig(cfg *TCPTransportCfg) *TCPTransport {
	return &TCPTransport{
		TCPTransportCfg: cfg,
		uidToConn:       make(map[uint64]*tcpctx),
		lock:            sync.RWMutex{},
	}
}

// Start CSTransport interface.
func (t *TCPTransport) Start(opt TransportOption) error {
	// 记录Start方法调用的指标
	metrics.IncrCounterWithGroup("net", "transport_start_total", 1)

	// 从TransportOption中获取receiver和creator
	t.receiver = opt.Handler
	t.creator = opt.Creator

	if t.TCPTransportCfg == nil {
		metrics.IncrCounterWithDimGroup("net", "transport_start_error_total", 1, map[string]string{"error_type": "nil_config"})
		return errors.New("TCPTransportCfg is nil")
	}
	if t.Addr == "" {
		metrics.IncrCounterWithDimGroup("net", "transport_start_error_total", 1, map[string]string{"error_type": "empty_addr"})
		return errors.New("Addr is empty")
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", t.Addr)
	if err != nil {
		metrics.IncrCounterWithDimGroup("net", "transport_start_error_total", 1, map[string]string{"error_type": "resolve"})
		return errors.New("resolve: " + err.Error())
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		metrics.IncrCounterWithDimGroup("net", "transport_start_error_total", 1, map[string]string{"error_type": "listen"})
		return errors.New("listen fail: " + err.Error())
	}

	// 记录成功启动的指标
	metrics.IncrCounterWithDimGroup("net", "transport_start_success_total", 1, map[string]string{"transport_type": "tcp"})

	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	go t.serve(ctx, listener)
	return nil
}

// Stop CSTransport interface.
func (t *TCPTransport) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

// StopRecv 停止收包.
func (t *TCPTransport) StopRecv() error {
	return errors.New("tcp transport not support stop recv")
}

// OnGracefulExitStart TCPTransport interface.当服务器开始优雅退出时调用.
func (t *TCPTransport) OnGracefulExitStart() {}

//nolint:nestif
func (t *TCPTransport) serve(ctx context.Context, listener *net.TCPListener) { //nolint:revive

	var once sync.Once
	closeListener := func() {
		if err := listener.Close(); err != nil {
		}
	}
	defer once.Do(closeListener)

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			var e net.Error
			if errors.As(err, &e) && e.Timeout() {
				continue
			}
			return
		}

		if err = conn.SetReadBuffer(t.MaxBufferSize); err != nil {
			log.Error().Int("BufSize", t.MaxBufferSize).Err(err).Msg("Set read buffer err")
			if err = conn.Close(); err != nil {
				log.Error().Err(err).Msg("Set read buffer, close err")
			}
			continue
		}
		if err = conn.SetWriteBuffer(int(t.MaxBufferSize)); err != nil {
			log.Error().Int("BufSize", t.MaxBufferSize).Err(err).Msg("Set write buffer err")
			if err = conn.Close(); err != nil {
				log.Error().Err(err).Msg("Set write buffer, close err")
			}
			continue
		}

		cancelCtx, cancel := context.WithCancel(ctx)
		tctx := &tcpctx{
			ctx:        ctx,
			cancelCtx:  cancelCtx,
			cancel:     cancel,
			conn:       conn,
			localAddr:  conn.LocalAddr(),
			remoteAddr: conn.RemoteAddr(),
			sendCh:     make(chan *TransSendPkg, t.SendChannelSize),
			transport:  t,
		}

		tctx.serve()
	}
}

// SendToClient CSTransport interface.
// Only stateful runtime can call this, stateless runtime will call tcpctx.SendToClient.
func (t *TCPTransport) SendToClient(pkg *TransSendPkg) error {
	t.lock.RLock()
	defer t.lock.RUnlock()
	tctx, ok := t.uidToConn[pkg.PkgHdr.GetDstActorID()]

	if !ok {
		return errors.New("tcp transport SendToClient not found uid: " + strconv.FormatUint(pkg.PkgHdr.GetDstActorID(), 10))
	}

	if err := tctx.SendToClient(pkg); err != nil {
		return err
	}

	return nil
}

// SendToServer transport send context to server.
func (t *TCPTransport) SendToServer(pkg *TransSendPkg) error {
	return t.SendToClient(pkg)
}

// CloseConn transport close connection.
func (t *TCPTransport) CloseConn(uid uint64) error {
	t.lock.RLock()
	tctx, ok := t.uidToConn[uid]
	t.lock.RUnlock()
	if !ok {
		return errors.New("tcp transport CloseConn not found uid: " + strconv.FormatUint(uid, 10))
	}
	tctx.close()
	return nil
}

func (t *TCPTransport) removeConn(uid uint64) {
	t.lock.Lock()
	defer t.lock.Unlock()

	delete(t.uidToConn, uid)
}

func (t *TCPTransport) addConn(uid uint64, tcx *tcpctx) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.uidToConn[uid] = tcx
}

// getCurrentConnCount returns the current number of connections
func (t *TCPTransport) getCurrentConnCount() int {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return len(t.uidToConn)
}

type tcpctx struct {
	uid           uint64
	actorMetaKey  uint64
	secureKey     uint64
	ctx           context.Context
	cancelCtx     context.Context
	cancel        context.CancelFunc
	lastReadTime  time.Time
	lastWriteTime time.Time
	conn          net.Conn
	localAddr     net.Addr
	remoteAddr    net.Addr
	closeOnce     sync.Once
	sendCh        chan *TransSendPkg
	transport     *TCPTransport
}

//nolint:revive
func (t *tcpctx) close() {
	t.closeOnce.Do(func() {

		t.transport.removeConn(t.uid)
		// 记录连接关闭的指标
		metrics.IncrCounterWithGroup("net", "connection_close_total", 1)
		// 更新当前连接数的指标
		currentConnCount := t.transport.getCurrentConnCount()
		metrics.UpdateGaugeWithGroup("net", "current_connections", metrics.Value(currentConnCount))

		// notify recv goroutine to exit

		t.cancel()
		// close socket
		_ = t.conn.Close()
	})
}

func (t *tcpctx) serve() {
	go t.serveSend()
	go t.serveRecv()
}

const (
	_uidLen       = 8
	_metaKeyLen   = 8
	_secureKeyLen = 8
)

func (t *tcpctx) readUID() bool {
	t.setReadDeadline()

	firstBuf := make([]byte, _uidLen+_metaKeyLen+_secureKeyLen)
	n, err := t.conn.Read(firstBuf)
	if err != nil {
		return false
	}

	if n != _uidLen+_metaKeyLen+_secureKeyLen {
		return false
	}

	t.uid = binary.LittleEndian.Uint64(firstBuf[:8])
	t.actorMetaKey = binary.LittleEndian.Uint64(firstBuf[8:16])
	t.secureKey = binary.LittleEndian.Uint64(firstBuf[16:])

	t.transport.CloseConn(t.uid)

	t.transport.addConn(t.uid, t)

	return true
}

func (t *tcpctx) readPreHead() (*PreHead, bool) {
	preHeadBuf := make([]byte, PRE_HEAD_SIZE)
	n, err := t.conn.Read(preHeadBuf)
	if err != nil {
		return nil, false
	}

	if n != PRE_HEAD_SIZE {
		return nil, false
	}

	preHead, err := DecodePreHead(preHeadBuf)
	if err != nil {
		return nil, false
	}

	if preHead.HdrSize+preHead.BodySize > uint32(t.transport.MaxBufferSize) {
		return nil, false
	}
	return preHead, true
}

func (t *tcpctx) beginServeRecv() error {
	// First package fix to a uid of 8 bytes and a metakey of 4 bytes
	t.setReadDeadline()
	ok := t.readUID()
	if !ok {
		metrics.IncrCounterWithGroup("net", "connection_auth_failure_total", 1)
		return errors.New("readUID fail")
	}

	// 记录连接成功的指标
	metrics.IncrCounterWithGroup("net", "connection_success_total", 1)
	// 更新当前连接数的指标
	metrics.UpdateGaugeWithGroup("net", "current_connections", metrics.Value(t.transport.getCurrentConnCount()))

	return nil
}

// recvPkg decode fail not quit, other err need quit loop.
func (t *tcpctx) recvPkg(buf *bytes.Buffer) (quitLoop bool, _ error) {
	// Create span for package reception
	ctx := context.Background()
	spanName := fmt.Sprintf("tcp.recv_pkg.%d", t.uid)
	ctx, span := tracing.StartSpanFromContext(ctx, spanName)
	defer span.End()

	// Set span tags
	span.SetTag("component", "tcp_transport")
	span.SetTag("connection.uid", t.uid)
	span.SetTag("connection.remote_addr", t.remoteAddr.String())
	span.SetTag("connection.local_addr", t.localAddr.String())

	// 1. read prehead
	t.setReadDeadline()
	preHead, ok := t.readPreHead()
	if !ok {
		span.SetTag("error", true)
		span.SetTag("error.type", "read_prehead")
		span.LogKV("error.message", "readPreHead fail")
		return true, errors.New("readPreHead fail")
	}

	// Set prehead info in span
	span.SetTag("message.hdr_size", preHead.HdrSize)
	span.SetTag("message.body_size", preHead.BodySize)

	// 2. read body
	pkgLen := preHead.HdrSize + preHead.BodySize
	buf.Reset()
	buf.Grow(int(pkgLen))
	pkgBuf := buf.Bytes()[:pkgLen]
	n, err := t.conn.Read(pkgBuf)
	if err != nil {
		span.SetTag("error", true)
		span.SetTag("error.type", "read_body")
		span.LogKV("error.message", "is stoped by: "+err.Error())
		return true, errors.New("is stoped by: " + err.Error())
	}

	if n != int(pkgLen) {
		span.SetTag("error", true)
		span.SetTag("error.type", "length_mismatch")
		span.LogKV("error.message", "read pkg: lens not match")
		return true, errors.New("read pkg: lens not match")
	}

	// 3. decode cs msg
	pkg, err := DecodeCSPkg(preHead, pkgBuf, t.transport.creator)
	if err != nil {
		span.SetTag("error", true)
		span.SetTag("error.type", "decode")
		span.LogKV("error.message", "decode: "+err.Error())
		return false, errors.New("decode: " + err.Error())
	}
	if pkg.PkgHdr == nil {
		span.SetTag("error", true)
		span.SetTag("error.type", "nil_header")
		span.LogKV("error.message", "pkg hdr is nil")
		return false, errors.New("pkg hdr is nil")
	}

	// Set message info in span
	span.SetTag("message.id", pkg.PkgHdr.GetMsgID())
	span.SetTag("message.dst_actor_id", pkg.PkgHdr.GetDstActorID())
	span.SetTag("message.src_actor_id", pkg.PkgHdr.GetSrcActorID())

	// check uid
	if pkg.PkgHdr.GetDstActorID() != t.uid {
		span.SetTag("error", true)
		span.SetTag("error.type", "uid_mismatch")
		span.LogKV("error.message", fmt.Sprintf("UID mismatch: expected %d, got %d", t.uid, pkg.PkgHdr.GetDstActorID()))
		// do nothing
	}

	// Log successful package reception
	span.LogFields(
		tracing.LogField{Key: "event", Value: "package_received"},
		tracing.LogField{Key: "package_size", Value: int(pkgLen)},
	)

	delivery := &TransportDelivery{
		TransSendBack: t.SendToClient,
		Pkg:           pkg,
	}

	err = t.transport.receiver.OnRecvTransportPkg(delivery)
	if err != nil {
		span.SetTag("error", true)
		span.SetTag("error.type", "dispatch")
		span.LogKV("error.message", err.Error())
	}

	return false, err
}

func (t *tcpctx) serveRecv() {
	defer t.close()

	if err := t.beginServeRecv(); err != nil {
		return
	}

	// TODO 池
	buf := &bytes.Buffer{}

	// Then enter loop of recv package cyclically
	for {
		// reset buffer pos to recv business package
		select {
		case <-t.ctx.Done():
			return
		case <-t.cancelCtx.Done():
			return
		default:
			quit, err := t.recvPkg(buf)
			if err != nil {
				// do nothing
			}
			if quit {
				return
			}
		}

	}
}

func (t *tcpctx) serveSend() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-t.cancelCtx.Done():
			return
		case pkg := <-t.sendCh:
			err := t.send(pkg)
			if err != nil {
				return
			}
		}
	}
}

func (t *tcpctx) send(pkg *TransSendPkg) error {
	buf := &bytes.Buffer{}

	p, err := PackCSPkg(pkg, buf)
	if err != nil {
		return errors.New(err.Error())
	}

	t.setWriteDeadline()
	_, err = t.conn.Write(p)
	if err != nil {
		return errors.New("SendToClient prehead fail: " + err.Error())
	}

	_, err = t.conn.Write(buf.Bytes())
	if err != nil {
		return errors.New("SendToClient pkg fail: " + err.Error())
	}
	return nil
}

func (t *tcpctx) setReadDeadline() {
	// timeout control, refer to the practice of trpc
	if t.transport.IdleTimeout > 0 {
		n := time.Now()
		if n.Sub(t.lastReadTime) > 5*time.Second {
			t.lastReadTime = n
			_ = t.conn.SetReadDeadline(n.Add(time.Duration(t.transport.IdleTimeout)))
		}
	}
}

func (t *tcpctx) setWriteDeadline() {
	if t.transport.IdleTimeout > 0 {
		n := time.Now()
		if n.Sub(t.lastWriteTime) > 5*time.Second {
			t.lastWriteTime = n
			_ = t.conn.SetWriteDeadline(n.Add(time.Duration(t.transport.IdleTimeout)))
		}
	}
}

// SendToClient tcp context send pkg to client.
func (t *tcpctx) SendToClient(pkg *TransSendPkg) error {
	if pkg.PkgHdr == nil {
		return errors.New("tcp SendToClient PkgHdr==nil")
	}

	select {
	case t.sendCh <- pkg:
		return nil
	default:
		return errors.New("send channel is full")
	}
}
