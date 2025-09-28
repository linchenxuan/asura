// Package net provides core networking infrastructure for Asura game server framework.

package net

import (
	"errors"
	"sync"
	"testing"
)

// Mock implementations for testing
type mockTransport struct {
	started bool
	stopped bool
	err     error
}

func (m *mockTransport) Start(to TransportOption) error {
	if m.err != nil {
		return m.err
	}
	m.started = true
	return nil
}

func (m *mockTransport) StopRecv() error {
	return nil
}

func (m *mockTransport) Stop() error {
	m.stopped = true
	return nil
}

func (m *mockTransport) SendToClient(pkg *TransSendPkg) error {
	return nil
}

type mockMsgLayerReceiver struct {
	called int
	err    error
	mu     sync.Mutex // Add mutex for thread-safe counter increment
}

func (m *mockMsgLayerReceiver) OnRecvDispatcherPkg(dd *DispatcherDelivery) error {
	m.mu.Lock()
	m.called++
	m.mu.Unlock()
	return m.err
}

// TestDispatcherDelivery tests the DispatcherDelivery structure and methods
func TestDispatcherDelivery(t *testing.T) {
	pkg := &TransRecvPkg{
		PkgHdr: &PkgHead{
			SrcActorID:    123,
			DstActorID:    456,
			MsgID:         "test.msg",
			SrcCltVersion: 789,
		},
		RouteHdr: &RouteHead{
			SrcEntityID:   999,
			SrcSetVersion: 888,
		},
	}

	td := &TransportDelivery{Pkg: pkg}
	dd := &DispatcherDelivery{
		TransportDelivery: td,
		ProtoInfo:         &MsgProtoInfo{MsgLayerType: MsgLayerType_Stateless},
		ResOpts:           []TransPkgOption{},
	}

	// Test GetReqSrcActorID
	if got := dd.GetReqSrcActorID(); got != 123 {
		t.Errorf("GetReqSrcActorID() = %v, want %v", got, 123)
	}

	// Test GetReqDstActorID
	if got := dd.GetReqDstActorID(); got != 456 {
		t.Errorf("GetReqDstActorID() = %v, want %v", got, 456)
	}

	// Test GetSrcEntityID
	if got := dd.GetSrcEntityID(); got != 999 {
		t.Errorf("GetSrcEntityID() = %v, want %v", got, 999)
	}

	// Test GetSrcSetVersion
	if got := dd.GetSrcSetVersion(); got != 888 {
		t.Errorf("GetSrcSetVersion() = %v, want %v", got, 888)
	}

	// Test GetSrcClientVersion
	if got := dd.GetSrcClientVersion(); got != 789 {
		t.Errorf("GetSrcClientVersion() = %v, want %v", got, 789)
	}
}

// TestNewDispatcher tests dispatcher creation with various configurations
func TestNewDispatcher(t *testing.T) {
	msgMgr := NewMessageManager()
	transports := []Transport{&mockTransport{}}

	tests := []struct {
		name    string
		cfg     *DispatcherConfig
		msgMgr  *MessageManager
		trans   []Transport
		wantErr bool
	}{
		{
			name:    "nil config uses defaults",
			cfg:     nil,
			msgMgr:  msgMgr,
			trans:   transports,
			wantErr: false,
		},
		{
			name: "valid config",
			cfg: &DispatcherConfig{
				RecvRateLimit: 1000,
				TokenBurst:    100,
			},
			msgMgr:  msgMgr,
			trans:   transports,
			wantErr: false,
		},
		{
			name: "invalid rate limit",
			cfg: &DispatcherConfig{
				RecvRateLimit: 0,
				TokenBurst:    100,
			},
			msgMgr:  msgMgr,
			trans:   transports,
			wantErr: true,
		},
		{
			name: "invalid token burst",
			cfg: &DispatcherConfig{
				RecvRateLimit: 1000,
				TokenBurst:    0,
			},
			msgMgr:  msgMgr,
			trans:   transports,
			wantErr: true,
		},
		{
			name:    "nil message manager",
			cfg:     nil,
			msgMgr:  nil,
			trans:   transports,
			wantErr: false, // Should handle nil msgMgr gracefully
		},
		{
			name:    "empty transports",
			cfg:     nil,
			msgMgr:  msgMgr,
			trans:   []Transport{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := NewDispatcher(tt.cfg, tt.msgMgr, tt.trans)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDispatcher() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && d == nil {
				t.Error("NewDispatcher() returned nil dispatcher")
			}
		})
	}
}

// TestRegisterMsglayer tests message layer registration
func TestRegisterMsglayer(t *testing.T) {
	msgMgr := NewMessageManager()
	d, err := NewDispatcher(nil, msgMgr, []Transport{})
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}

	handler := &mockMsgLayerReceiver{}

	tests := []struct {
		name    string
		msgType MsgLayerType
		handler MsgLayerReceiver
		wantErr bool
	}{
		{
			name:    "valid stateless layer",
			msgType: MsgLayerType_Stateless,
			handler: handler,
			wantErr: false,
		},
		{
			name:    "valid stateful layer",
			msgType: MsgLayerType_Stateful,
			handler: handler,
			wantErr: false,
		},
		{
			name:    "nil handler",
			msgType: MsgLayerType_Stateless,
			handler: nil,
			wantErr: true,
		},
		{
			name:    "invalid layer type",
			msgType: MsgLayerType_Max,
			handler: handler,
			wantErr: true,
		},
		{
			name:    "duplicate registration",
			msgType: MsgLayerType_Stateless,
			handler: handler,
			wantErr: true,
		},
	}

	// Test first registration
	err = d.RegisterMsglayer(MsgLayerType_Stateless, handler)
	if err != nil {
		t.Errorf("First RegisterMsglayer() error = %v", err)
	}

	// Test various scenarios
	for _, tt := range tests[1:] {
		t.Run(tt.name, func(t *testing.T) {
			err := d.RegisterMsglayer(tt.msgType, tt.handler)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterMsglayer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestStartServe tests dispatcher service startup
func TestStartServe(t *testing.T) {
	msgMgr := NewMessageManager()

	tests := []struct {
		name       string
		transports []Transport
		wantErr    bool
	}{
		{
			name: "successful startup",
			transports: []Transport{
				&mockTransport{},
				&mockTransport{},
			},
			wantErr: false,
		},
		{
			name: "transport startup failure",
			transports: []Transport{
				&mockTransport{},
				&mockTransport{err: errors.New("start failed")},
			},
			wantErr: true,
		},
		{
			name:       "no transports",
			transports: []Transport{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := NewDispatcher(nil, msgMgr, tt.transports)
			if err != nil {
				t.Fatalf("NewDispatcher() error = %v", err)
			}

			// Use existing mockMsgCreator from csmsgcodec_test.go
			creator := &mockMsgCreator{}
			err = d.StartServe(creator)
			if (err != nil) != tt.wantErr {
				t.Errorf("StartServe() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check transport states
			if !tt.wantErr {
				for i, trans := range tt.transports {
					if mt, ok := trans.(*mockTransport); ok && !mt.started {
						t.Errorf("Transport %d not started", i)
					}
				}
			}
		})
	}
}

// TestRegDispatcherFilter tests filter registration
func TestRegDispatcherFilter(t *testing.T) {
	msgMgr := NewMessageManager()
	d, err := NewDispatcher(nil, msgMgr, []Transport{})
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}

	// Create mock filters
	filter1 := func(dd *DispatcherDelivery, f DispatcherFilterHandleFunc) error {
		return f(dd)
	}
	filter2 := func(dd *DispatcherDelivery, f DispatcherFilterHandleFunc) error {
		return f(dd)
	}

	// Register filters
	d.RegDispatcherFilter(filter1)
	d.RegDispatcherFilter(filter2)

	// Verify filters were added (we can't easily test the internal state,
	// but we can verify no panics occurred)
	t.Log("Filters registered successfully")
}

// TestOnRecvTransportPkg tests message processing through the dispatcher
func TestOnRecvTransportPkg(t *testing.T) {
	msgMgr := NewMessageManager()

	// Add a protocol info for testing
	protoInfo := &MsgProtoInfo{
		MsgID:        "test.msg",
		MsgLayerType: MsgLayerType_Stateless,
	}
	msgMgr.PropInfoMap["test.msg"] = protoInfo

	d, err := NewDispatcher(nil, msgMgr, []Transport{})
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}

	// Register a message layer
	handler := &mockMsgLayerReceiver{}
	err = d.RegisterMsglayer(MsgLayerType_Stateless, handler)
	if err != nil {
		t.Fatalf("RegisterMsglayer() error = %v", err)
	}

	// Create test transport delivery
	td := &TransportDelivery{
		Pkg: &TransRecvPkg{
			PkgHdr: &PkgHead{
				MsgID: "test.msg",
			},
		},
	}

	// Process message
	err = d.OnRecvTransportPkg(td)
	if err != nil {
		t.Errorf("OnRecvTransportPkg() error = %v", err)
	}

	// Verify handler was called
	if handler.called != 1 {
		t.Errorf("Handler called %d times, want 1", handler.called)
	}
}

// TestDispatcherWithFilters tests dispatcher with custom filters
func TestDispatcherWithFilters(t *testing.T) {
	msgMgr := NewMessageManager()
	d, err := NewDispatcher(nil, msgMgr, []Transport{})
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}

	// Register message layer
	handler := &mockMsgLayerReceiver{}
	d.RegisterMsglayer(MsgLayerType_Stateless, handler)

	// Register custom filter
	filter := func(dd *DispatcherDelivery, f DispatcherFilterHandleFunc) error {
		t.Log("Custom filter called")
		return f(dd)
	}
	d.RegDispatcherFilter(filter)

	// Add protocol info
	protoInfo := &MsgProtoInfo{
		MsgID:        "test.msg",
		MsgLayerType: MsgLayerType_Stateless,
	}
	msgMgr.PropInfoMap["test.msg"] = protoInfo

	// Create and process message
	td := &TransportDelivery{
		Pkg: &TransRecvPkg{
			PkgHdr: &PkgHead{
				MsgID: "test.msg",
			},
		},
	}

	err = d.OnRecvTransportPkg(td)
	if err != nil {
		t.Errorf("OnRecvTransportPkg() error = %v", err)
	}

	if handler.called != 1 {
		t.Error("Handler was not called after filter processing")
	}
}

// TestDispatcherErrorHandling tests error handling in various scenarios
func TestDispatcherErrorHandling(t *testing.T) {
	msgMgr := NewMessageManager()
	d, err := NewDispatcher(nil, msgMgr, []Transport{})
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}

	// Test with no registered message layer - should not panic
	td := &TransportDelivery{
		Pkg: &TransRecvPkg{
			PkgHdr: &PkgHead{
				MsgID: "unknown.msg",
			},
		},
	}

	// Add protocol info for unknown message
	protoInfo := &MsgProtoInfo{
		MsgID:        "unknown.msg",
		MsgLayerType: MsgLayerType_Stateless,
	}
	msgMgr.PropInfoMap["unknown.msg"] = protoInfo

	err = d.OnRecvTransportPkg(td)
	if err == nil {
		t.Error("Expected error for unknown message layer, got nil")
	}

	// Test handler error
	handler := &mockMsgLayerReceiver{err: errors.New("handler error")}
	d.RegisterMsglayer(MsgLayerType_Stateless, handler)

	// Update to use known message
	td.Pkg.PkgHdr.MsgID = "test.msg"
	protoInfo2 := &MsgProtoInfo{
		MsgID:        "test.msg",
		MsgLayerType: MsgLayerType_Stateless,
	}
	msgMgr.PropInfoMap["test.msg"] = protoInfo2

	err = d.OnRecvTransportPkg(td)
	if err == nil {
		t.Error("Expected handler error, got nil")
	}
}

// TestDispatcherConcurrency tests concurrent message processing
func TestDispatcherConcurrency(t *testing.T) {
	msgMgr := NewMessageManager()
	d, err := NewDispatcher(nil, msgMgr, []Transport{})
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}

	// Register message layer
	handler := &mockMsgLayerReceiver{}
	d.RegisterMsglayer(MsgLayerType_Stateless, handler)

	// Add protocol info
	protoInfo := &MsgProtoInfo{
		MsgID:        "test.msg",
		MsgLayerType: MsgLayerType_Stateless,
	}
	msgMgr.PropInfoMap["test.msg"] = protoInfo

	// Process messages concurrently
	var wg sync.WaitGroup
	numGoroutines := 10
	messagesPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				td := &TransportDelivery{
					Pkg: &TransRecvPkg{
						PkgHdr: &PkgHead{
							MsgID: "test.msg",
						},
					},
				}
				if err := d.OnRecvTransportPkg(td); err != nil {
					t.Errorf("Concurrent OnRecvTransportPkg() error = %v", err)
				}
			}
		}()
	}

	wg.Wait()

	// Verify handler was called the correct number of times
	expectedCalls := numGoroutines * messagesPerGoroutine
	if handler.called != expectedCalls {
		t.Errorf("Handler called %d times, want %d", handler.called, expectedCalls)
	}
}

// TestGetDefaultConfig tests default configuration
func TestGetDefaultConfig(t *testing.T) {
	cfg := getDefaultConfig()
	if cfg == nil {
		t.Fatal("getDefaultConfig() returned nil")
	}
	if cfg.RecvRateLimit != 20000 {
		t.Errorf("Default RecvRateLimit = %v, want %v", cfg.RecvRateLimit, 20000)
	}
	if cfg.TokenBurst != 1000 {
		t.Errorf("Default TokenBurst = %v, want %v", cfg.TokenBurst, 1000)
	}
}

// TestCheckParamValid tests parameter validation
func TestCheckParamValid(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *DispatcherConfig
		wantErr bool
	}{
		{
			name: "valid parameters",
			cfg: &DispatcherConfig{
				RecvRateLimit: 1000,
				TokenBurst:    100,
			},
			wantErr: false,
		},
		{
			name: "zero rate limit",
			cfg: &DispatcherConfig{
				RecvRateLimit: 0,
				TokenBurst:    100,
			},
			wantErr: true,
		},
		{
			name: "zero token burst",
			cfg: &DispatcherConfig{
				RecvRateLimit: 1000,
				TokenBurst:    0,
			},
			wantErr: true,
		},
		{
			name: "negative rate limit",
			cfg: &DispatcherConfig{
				RecvRateLimit: -1,
				TokenBurst:    100,
			},
			wantErr: true,
		},
		{
			name: "negative token burst",
			cfg: &DispatcherConfig{
				RecvRateLimit: 1000,
				TokenBurst:    -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkParamValid(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkParamValid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// BenchmarkDispatcherMessageProcessing benchmarks message processing performance
func BenchmarkDispatcherMessageProcessing(b *testing.B) {
	msgMgr := NewMessageManager()
	d, err := NewDispatcher(nil, msgMgr, []Transport{})
	if err != nil {
		b.Fatalf("NewDispatcher() error = %v", err)
	}

	// Register message layer
	handler := &mockMsgLayerReceiver{}
	d.RegisterMsglayer(MsgLayerType_Stateless, handler)

	// Add protocol info
	protoInfo := &MsgProtoInfo{
		MsgID:        "bench.msg",
		MsgLayerType: MsgLayerType_Stateless,
	}
	msgMgr.PropInfoMap["bench.msg"] = protoInfo

	// Create test message
	td := &TransportDelivery{
		Pkg: &TransRecvPkg{
			PkgHdr: &PkgHead{
				MsgID: "bench.msg",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := d.OnRecvTransportPkg(td); err != nil {
			b.Errorf("OnRecvTransportPkg() error = %v", err)
		}
	}
}

// BenchmarkDispatcherConcurrentMessageProcessing benchmarks concurrent message processing
func BenchmarkDispatcherConcurrentMessageProcessing(b *testing.B) {
	msgMgr := NewMessageManager()
	d, err := NewDispatcher(nil, msgMgr, []Transport{})
	if err != nil {
		b.Fatalf("NewDispatcher() error = %v", err)
	}

	// Register message layer
	handler := &mockMsgLayerReceiver{}
	d.RegisterMsglayer(MsgLayerType_Stateless, handler)

	// Add protocol info
	protoInfo := &MsgProtoInfo{
		MsgID:        "bench.msg",
		MsgLayerType: MsgLayerType_Stateless,
	}
	msgMgr.PropInfoMap["bench.msg"] = protoInfo

	// Create test message
	td := &TransportDelivery{
		Pkg: &TransRecvPkg{
			PkgHdr: &PkgHead{
				MsgID: "bench.msg",
			},
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := d.OnRecvTransportPkg(td); err != nil {
				b.Errorf("OnRecvTransportPkg() error = %v", err)
			}
		}
	})
}
