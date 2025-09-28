package net

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/lcx/asura/log"
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

// TCPTransport a transport plugin based on tcp, ervey connection will be alloc a goroutine.
type TCPTransport struct {
	*TCPTransportCfg
	uidToConn map[uint64]*tcpctx
	lock      sync.RWMutex
	receiver  DispatcherReceiver
	creator   MsgCreator
	cancel    context.CancelFunc
}

func NewTCPTransport() *TCPTransport {
	return &TCPTransport{
		TCPTransportCfg: &TCPTransportCfg{},
		uidToConn:       make(map[uint64]*tcpctx),
		lock:            sync.RWMutex{},
	}
}

// Start CSTransport interface.
func (t *TCPTransport) Start() error {
	if t.TCPTransportCfg == nil {
		return errors.New("TCPTransportCfg is nil")
	}
	if t.Addr == "" {
		return errors.New("Addr is empty")
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", t.Addr)
	if err != nil {
		return errors.New("resolve: " + err.Error())
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return errors.New("listen fail: " + err.Error())
	}

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
		return errors.New("readUID fail")
	}

	return nil
}

// recvPkg decode fail not quit, other err need quit loop.
func (t *tcpctx) recvPkg(buf *bytes.Buffer) (quitLoop bool, _ error) {
	// 1. read prehead
	t.setReadDeadline()
	preHead, ok := t.readPreHead()
	if !ok {
		return true, errors.New("readPreHead fail")
	}

	// 2. read body
	pkgLen := preHead.HdrSize + preHead.BodySize
	buf.Reset()
	buf.Grow(int(pkgLen))
	pkgBuf := buf.Bytes()[:pkgLen]
	n, err := t.conn.Read(pkgBuf)
	if err != nil {
		return true, errors.New("is stoped by: " + err.Error())
	}

	if n != int(pkgLen) {
		return true, errors.New("read pkg: lens not match")
	}

	// 3. decode cs msg
	pkg, err := DecodeCSPkg(preHead, pkgBuf, t.transport.creator)
	if err != nil {
		return false, errors.New("decode: " + err.Error())
	}
	if pkg.PkgHdr == nil {
		return false, errors.New("pkg hdr is nil")
	}
	// check uid
	if pkg.PkgHdr.GetDstActorID() != t.uid {
		// do nothing
	}

	ctx := &TransportDelivery{
		TransSendBack: t.SendToClient,
		Pkg:           pkg,
	}

	return false, t.transport.receiver.OnRecvTransportPkg(ctx)
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
