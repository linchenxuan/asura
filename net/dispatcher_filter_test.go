package net

import (
	"errors"
	"fmt"
	"testing"
)

// MockTransport implements Transport interface for testing
type MockTransport struct {
	started bool
}

func (t *MockTransport) Start(opt TransportOption) error {
	t.started = true
	return nil
}

func (t *MockTransport) StopRecv() error {
	return nil
}

func (t *MockTransport) Stop() error {
	t.started = false
	return nil
}

// MockMsgCreator implements MsgCreator interface for testing
type MockMsgCreator struct{}

func (c *MockMsgCreator) CreateMsg(msgID string) (interface{}, error) {
	return struct{}{}, nil
}

// TestDispatcherFilterChain tests the filter chain processing logic
func TestDispatcherFilterChain(t *testing.T) {
	tests := []struct {
		name          string
		filters       DispatcherFilterChain
		expectedCalls []string
		expectedError bool
		handlerError  error
	}{
		{
			name:          "Empty chain should call handler directly",
			filters:       DispatcherFilterChain{},
			expectedCalls: []string{"handler"},
			expectedError: false,
		},
		{
			name: "Single filter should call filter then handler",
			filters: DispatcherFilterChain{
				func(dd *DispatcherDelivery, f DispatcherFilterHandleFunc) error {
					return f(dd)
				},
			},
			expectedCalls: []string{"handler"},
			expectedError: false,
		},
		{
			name: "Multiple filters should chain correctly",
			filters: DispatcherFilterChain{
				func(dd *DispatcherDelivery, f DispatcherFilterHandleFunc) error {
					return f(dd)
				},
				func(dd *DispatcherDelivery, f DispatcherFilterHandleFunc) error {
					return f(dd)
				},
			},
			expectedCalls: []string{"handler"},
			expectedError: false,
		},
		{
			name: "Filter returning error should stop chain",
			filters: DispatcherFilterChain{
				func(dd *DispatcherDelivery, f DispatcherFilterHandleFunc) error {
					return errors.New("filter error")
				},
			},
			expectedCalls: []string{},
			expectedError: true,
		},
		{
			name:          "Handler returning error should propagate",
			filters:       DispatcherFilterChain{},
			expectedCalls: []string{"handler"},
			expectedError: true,
			handlerError:  errors.New("handler error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := []string{}
			handler := func(dd *DispatcherDelivery) error {
				calls = append(calls, "handler")
				return tt.handlerError
			}

			err := tt.filters.Handle(&DispatcherDelivery{}, handler)

			if tt.expectedError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestMsgFilterPluginCfg tests message filter configuration
func TestMsgFilterPluginCfg(t *testing.T) {
	cfg := &MsgFilterPluginCfg{
		MsgFilter: []string{"test.msg1", "test.msg2", "test.msg3"},
	}

	if len(cfg.MsgFilter) != 3 {
		t.Errorf("Expected 3 message filters, got %d", len(cfg.MsgFilter))
	}

	expectedFilters := []string{"test.msg1", "test.msg2", "test.msg3"}
	for i, expected := range expectedFilters {
		if cfg.MsgFilter[i] != expected {
			t.Errorf("Expected filter %s at index %d, got %s", expected, i, cfg.MsgFilter[i])
		}
	}
}

// TestReloadMsgFilterCfg tests reloading message filter configuration
func TestReloadMsgFilterCfg(t *testing.T) {
	msgMgr := NewMessageManager()
	transport := &MockTransport{}
	dispatcher, err := NewDispatcher(&DispatcherConfig{
		RecvRateLimit: 20000,
		TokenBurst:    1000,
	}, msgMgr, []Transport{transport})
	if err != nil {
		t.Fatalf("Failed to create dispatcher: %v", err)
	}

	// Test initial empty configuration
	if len(dispatcher.msgFilterMap) != 0 {
		t.Errorf("Expected empty filter map, got %d entries", len(dispatcher.msgFilterMap))
	}

	// Test reloading with configuration
	cfg := &MsgFilterPluginCfg{
		MsgFilter: []string{"msg1", "msg2", "msg3"},
	}
	dispatcher.reloadMsgFilterCfg(cfg)

	if len(dispatcher.msgFilterMap) != 3 {
		t.Errorf("Expected 3 filter entries, got %d", len(dispatcher.msgFilterMap))
	}

	// Verify all messages are in filter map
	for _, msgID := range cfg.MsgFilter {
		if _, ok := dispatcher.msgFilterMap[msgID]; !ok {
			t.Errorf("Expected message %s to be in filter map", msgID)
		}
	}

	// Test reloading with additional messages
	cfg2 := &MsgFilterPluginCfg{
		MsgFilter: []string{"msg4", "msg5"},
	}
	dispatcher.reloadMsgFilterCfg(cfg2)

	if len(dispatcher.msgFilterMap) != 5 {
		t.Errorf("Expected 5 filter entries after reload, got %d", len(dispatcher.msgFilterMap))
	}
}

// TestMsgFilter tests the message filtering functionality
func TestMsgFilter(t *testing.T) {
	msgMgr := NewMessageManager()

	// Set up protocol info for test messages
	reqProtoInfo := &MsgProtoInfo{
		MsgID:      "test.req",
		ResMsgID:   "test.res",
		MsgReqType: MRTReq,
	}
	msgMgr.PropInfoMap["test.req"] = reqProtoInfo

	resProtoInfo := &MsgProtoInfo{
		MsgID:      "test.res",
		MsgReqType: MRTRes,
	}
	msgMgr.PropInfoMap["test.res"] = resProtoInfo

	transport := &MockTransport{}
	dispatcher, err := NewDispatcher(&DispatcherConfig{
		RecvRateLimit: 20000,
		TokenBurst:    1000,
	}, msgMgr, []Transport{transport})
	if err != nil {
		t.Fatalf("Failed to create dispatcher: %v", err)
	}

	// Add message to filter
	dispatcher.reloadMsgFilterCfg(&MsgFilterPluginCfg{
		MsgFilter: []string{"test.req", "unknown.msg"},
	})

	tests := []struct {
		name          string
		msgID         string
		isReq         bool
		expectFilter  bool
		setupDelivery func() *DispatcherDelivery
		expectError   bool
	}{
		{
			name:         "Filtered request message should return nil without calling handler",
			msgID:        "test.req",
			isReq:        true,
			expectFilter: true,
			setupDelivery: func() *DispatcherDelivery {
				return &DispatcherDelivery{
					TransportDelivery: &TransportDelivery{
						Pkg: &TransRecvPkg{
							PkgHdr: &PkgHead{
								MsgID:      "test.req",
								SrcActorID: 1001,
								DstActorID: 2001,
								SvrPkgSeq:  123,
								CltPkgSeq:  456,
							},
						},
						TransSendBack: func(pkg *TransSendPkg) error {
							return nil
						},
					},
				}
			},
			expectError: false,
		},
		{
			name:         "Non-filtered message should call handler",
			msgID:        "test.other",
			isReq:        true,
			expectFilter: false,
			setupDelivery: func() *DispatcherDelivery {
				return &DispatcherDelivery{
					TransportDelivery: &TransportDelivery{
						Pkg: &TransRecvPkg{
							PkgHdr: &PkgHead{
								MsgID: "test.other",
							},
						},
					},
				}
			},
			expectError: false,
		},
		{
			name:         "Response message should not be filtered even if in filter list",
			msgID:        "test.res",
			isReq:        false,
			expectFilter: false,
			setupDelivery: func() *DispatcherDelivery {
				return &DispatcherDelivery{
					TransportDelivery: &TransportDelivery{
						Pkg: &TransRecvPkg{
							PkgHdr: &PkgHead{
								MsgID: "test.res",
							},
						},
					},
				}
			},
			expectError: false,
		},
		{
			name:  "Missing protocol info should return error",
			msgID: "unknown.msg",
			setupDelivery: func() *DispatcherDelivery {
				return &DispatcherDelivery{
					TransportDelivery: &TransportDelivery{
						Pkg: &TransRecvPkg{
							PkgHdr: &PkgHead{
								MsgID: "unknown.msg",
							},
						},
						TransSendBack: func(pkg *TransSendPkg) error {
							return nil
						},
					},
				}
			},
			expectFilter: true, // This message should be filtered first
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delivery := tt.setupDelivery()
			handlerCalled := false

			handler := func(dd *DispatcherDelivery) error {
				handlerCalled = true
				return nil
			}

			err := dispatcher.msgFilter(delivery, handler)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectFilter && handlerCalled {
				t.Errorf("Expected handler not to be called for filtered message")
			}
			if !tt.expectFilter && !handlerCalled {
				t.Log(tt.msgID)
				t.Errorf("Expected handler to be called for non-filtered message")
			}
		})
	}
}

// TestMsgFilterWithSendBack tests message filtering with response sending
func TestMsgFilterWithSendBack(t *testing.T) {
	msgMgr := NewMessageManager()

	// Set up protocol info for request/response pair
	reqProtoInfo := &MsgProtoInfo{
		MsgID:      "game.join",
		ResMsgID:   "game.join.res",
		MsgReqType: MRTReq,
	}
	msgMgr.PropInfoMap["game.join"] = reqProtoInfo

	transport := &MockTransport{}
	dispatcher, err := NewDispatcher(&DispatcherConfig{
		RecvRateLimit: 20000,
		TokenBurst:    1000,
	}, msgMgr, []Transport{transport})
	if err != nil {
		t.Fatalf("Failed to create dispatcher: %v", err)
	}

	// Add message to filter
	dispatcher.reloadMsgFilterCfg(&MsgFilterPluginCfg{
		MsgFilter: []string{"game.join"},
	})

	// Track sent responses
	var sentResponses []*TransSendPkg

	delivery := &DispatcherDelivery{
		TransportDelivery: &TransportDelivery{
			Pkg: &TransRecvPkg{
				PkgHdr: &PkgHead{
					MsgID:      "game.join",
					SrcActorID: 1001,
					DstActorID: 2001,
					SvrPkgSeq:  789,
					CltPkgSeq:  012,
				},
			},
			TransSendBack: func(pkg *TransSendPkg) error {
				sentResponses = append(sentResponses, pkg)
				return nil
			},
		},
	}

	handler := func(dd *DispatcherDelivery) error {
		t.Errorf("Handler should not be called for filtered message")
		return nil
	}

	err = dispatcher.msgFilter(delivery, handler)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify response was sent
	if len(sentResponses) != 1 {
		t.Fatalf("Expected 1 response to be sent, got %d", len(sentResponses))
	}

	response := sentResponses[0]
	if response.PkgHdr.MsgID != "game.join.res" {
		t.Errorf("Expected response MsgID to be 'game.join.res', got '%s'", response.PkgHdr.MsgID)
	}
	if response.PkgHdr.SrcActorID != 2001 {
		t.Errorf("Expected response SrcActorID to be 2001, got %d", response.PkgHdr.SrcActorID)
	}
	if response.PkgHdr.DstActorID != 1001 {
		t.Errorf("Expected response DstActorID to be 1001, got %d", response.PkgHdr.DstActorID)
	}
	if response.PkgHdr.SvrPkgSeq != 789 {
		t.Errorf("Expected response SvrPkgSeq to be 789, got %d", response.PkgHdr.SvrPkgSeq)
	}
	if response.PkgHdr.CltPkgSeq != 012 {
		t.Errorf("Expected response CltPkgSeq to be 012, got %d", response.PkgHdr.CltPkgSeq)
	}
}

// TestMsgFilterWithoutSendBack tests message filtering when TransSendBack is nil
func TestMsgFilterWithoutSendBack(t *testing.T) {
	msgMgr := NewMessageManager()

	// Set up protocol info
	reqProtoInfo := &MsgProtoInfo{
		MsgID:      "test.req",
		ResMsgID:   "test.res",
		MsgReqType: MRTReq,
	}
	msgMgr.PropInfoMap["test.req"] = reqProtoInfo

	transport := &MockTransport{}
	dispatcher, err := NewDispatcher(&DispatcherConfig{
		RecvRateLimit: 20000,
		TokenBurst:    1000,
	}, msgMgr, []Transport{transport})
	if err != nil {
		t.Fatalf("Failed to create dispatcher: %v", err)
	}

	// Add message to filter
	dispatcher.reloadMsgFilterCfg(&MsgFilterPluginCfg{
		MsgFilter: []string{"test.req"},
	})

	// Create delivery without TransSendBack function
	delivery := &DispatcherDelivery{
		TransportDelivery: &TransportDelivery{
			Pkg: &TransRecvPkg{
				PkgHdr: &PkgHead{
					MsgID: "test.req",
				},
			},
			TransSendBack: nil, // No send back function
		},
	}

	handler := func(dd *DispatcherDelivery) error {
		t.Errorf("Handler should not be called for filtered message")
		return nil
	}

	// Should not panic and should return nil
	err = dispatcher.msgFilter(delivery, handler)
	if err != nil {
		t.Errorf("Expected nil error when TransSendBack is nil, got: %v", err)
	}
}

// TestDispatcherFilterChainWithMultipleFilters tests complex filter chains
func TestDispatcherFilterChainWithMultipleFilters(t *testing.T) {
	callOrder := []string{}

	filter1 := func(dd *DispatcherDelivery, f DispatcherFilterHandleFunc) error {
		callOrder = append(callOrder, "filter1-before")
		err := f(dd)
		callOrder = append(callOrder, "filter1-after")
		return err
	}

	filter2 := func(dd *DispatcherDelivery, f DispatcherFilterHandleFunc) error {
		callOrder = append(callOrder, "filter2-before")
		err := f(dd)
		callOrder = append(callOrder, "filter2-after")
		return err
	}

	filter3 := func(dd *DispatcherDelivery, f DispatcherFilterHandleFunc) error {
		callOrder = append(callOrder, "filter3-before")
		err := f(dd)
		callOrder = append(callOrder, "filter3-after")
		return err
	}

	chain := DispatcherFilterChain{filter1, filter2, filter3}

	handler := func(dd *DispatcherDelivery) error {
		callOrder = append(callOrder, "handler")
		return nil
	}

	err := chain.Handle(&DispatcherDelivery{}, handler)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expectedOrder := []string{
		"filter1-before",
		"filter2-before",
		"filter3-before",
		"handler",
		"filter3-after",
		"filter2-after",
		"filter1-after",
	}

	if len(callOrder) != len(expectedOrder) {
		t.Fatalf("Expected %d calls, got %d", len(expectedOrder), len(callOrder))
	}

	for i, expected := range expectedOrder {
		if callOrder[i] != expected {
			t.Errorf("Expected call order[%d] to be '%s', got '%s'", i, expected, callOrder[i])
		}
	}
}

// BenchmarkDispatcherFilterChain benchmarks filter chain performance
func BenchmarkDispatcherFilterChain(b *testing.B) {
	chain := DispatcherFilterChain{
		func(dd *DispatcherDelivery, f DispatcherFilterHandleFunc) error {
			return f(dd)
		},
		func(dd *DispatcherDelivery, f DispatcherFilterHandleFunc) error {
			return f(dd)
		},
		func(dd *DispatcherDelivery, f DispatcherFilterHandleFunc) error {
			return f(dd)
		},
	}

	handler := func(dd *DispatcherDelivery) error {
		return nil
	}

	delivery := &DispatcherDelivery{
		TransportDelivery: &TransportDelivery{
			Pkg: &TransRecvPkg{
				PkgHdr: &PkgHead{
					MsgID: "benchmark.msg",
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = chain.Handle(delivery, handler)
	}
}

// BenchmarkMsgFilter benchmarks message filter performance
func BenchmarkMsgFilter(b *testing.B) {
	msgMgr := NewMessageManager()

	// Set up many protocol infos
	for i := 0; i < 100; i++ {
		msgMgr.PropInfoMap[fmt.Sprintf("msg.%d", i)] = &MsgProtoInfo{
			MsgID:      fmt.Sprintf("msg.%d", i),
			ResMsgID:   fmt.Sprintf("msg.%d.res", i),
			MsgReqType: MRTReq,
		}
	}

	transport := &MockTransport{}
	dispatcher, _ := NewDispatcher(&DispatcherConfig{
		RecvRateLimit: 20000,
		TokenBurst:    1000,
	}, msgMgr, []Transport{transport})

	// Add many messages to filter
	var filterMsgs []string
	for i := 0; i < 50; i++ {
		filterMsgs = append(filterMsgs, fmt.Sprintf("msg.%d", i))
	}
	dispatcher.reloadMsgFilterCfg(&MsgFilterPluginCfg{MsgFilter: filterMsgs})

	delivery := &DispatcherDelivery{
		TransportDelivery: &TransportDelivery{
			Pkg: &TransRecvPkg{
				PkgHdr: &PkgHead{
					MsgID: "msg.25", // This is in the filter list
				},
			},
			TransSendBack: func(pkg *TransSendPkg) error {
				return nil
			},
		},
	}

	handler := func(dd *DispatcherDelivery) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dispatcher.msgFilter(delivery, handler)
	}
}
