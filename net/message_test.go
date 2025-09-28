package net

import (
	"errors"
	"testing"

	"github.com/lcx/asura/utils"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Mock message implementations for testing
type mockProtoMessage struct {
	msgID string
}

func (m *mockProtoMessage) Reset()         {}
func (m *mockProtoMessage) String() string { return m.msgID }
func (m *mockProtoMessage) ProtoMessage()  {}
func (m *mockProtoMessage) ProtoReflect() protoreflect.Message {
	return nil // Simplified implementation for testing
}

// Mock AsuraMsg implementation
type mockAsuraMsg struct {
	mockProtoMessage
	isRequest  bool
	isResponse bool
}

func (m *mockAsuraMsg) MsgID() string {
	return m.msgID
}

func (m *mockAsuraMsg) IsRequest() bool {
	return m.isRequest
}

func (m *mockAsuraMsg) IsResponse() bool {
	return m.isResponse
}

// Mock SCNtfMsg implementation
type mockSCNtfMsg struct {
	mockAsuraMsg
}

func (m *mockSCNtfMsg) SCNtfMsgID() string {
	return m.msgID
}

func (m *mockSCNtfMsg) NtfClient(sender NtfPkgSender, dstEntity uint32, dstActorID uint64, opts ...TransPkgOption) {
	// Mock implementation
}

func (m *mockSCNtfMsg) DeferNtfClient(sender DeferPkgSender, dstEntity uint32, dstActorID uint64, opts ...TransPkgOption) {
	// Mock implementation
}

// Mock SSMsg implementation
type mockSSMsg struct {
	mockAsuraMsg
}

func (m *mockSSMsg) IsSS() bool {
	return true
}

// Mock RPCReqMsg implementation
type mockRPCReqMsg struct {
	mockAsuraMsg
	resMsgID string
}

func (m *mockRPCReqMsg) ResMsgID() string {
	return m.resMsgID
}

func (m *mockRPCReqMsg) CreateResMsg() proto.Message {
	return &mockProtoMessage{msgID: m.resMsgID}
}

// Mock MsgCreator for message tests
type mockMsgCreatorForMessage struct {
	messages map[string]proto.Message
}

func (m *mockMsgCreatorForMessage) CreateMsg(msgID string) (proto.Message, error) {
	if msg, ok := m.messages[msgID]; ok {
		return msg, nil
	}
	return nil, errors.New("message not found")
}

func (m *mockMsgCreatorForMessage) ContainsMsg(msgID string) bool {
	_, ok := m.messages[msgID]
	return ok
}

// Test MsgReqType constants
func TestMsgReqTypeConstants(t *testing.T) {
	if MRTNone != 0 {
		t.Errorf("MRTNone should be 0, got %d", MRTNone)
	}
	if MRTReq != 1 {
		t.Errorf("MRTReq should be 1, got %d", MRTReq)
	}
	if MRTRes != 2 {
		t.Errorf("MRTRes should be 2, got %d", MRTRes)
	}
	if MRTNtf != 3 {
		t.Errorf("MRTNtf should be 3, got %d", MRTNtf)
	}
}

// Test MsgProtoInfo methods
func TestMsgProtoInfoMethods(t *testing.T) {
	tests := []struct {
		name     string
		pi       *MsgProtoInfo
		expected struct {
			isNtf    bool
			isSSReq  bool
			isSSRes  bool
			isReq    bool
			isRes    bool
			resMsgID string
			msgID    string
		}
	}{
		{
			name: "notification message",
			pi: &MsgProtoInfo{
				MsgID:      "test.ntf",
				MsgReqType: MRTNtf,
				IsCS:       true,
			},
			expected: struct {
				isNtf    bool
				isSSReq  bool
				isSSRes  bool
				isReq    bool
				isRes    bool
				resMsgID string
				msgID    string
			}{
				isNtf:    true,
				isSSReq:  false,
				isSSRes:  false,
				isReq:    false,
				isRes:    false,
				resMsgID: "",
				msgID:    "test.ntf",
			},
		},
		{
			name: "client-server request",
			pi: &MsgProtoInfo{
				MsgID:      "test.req",
				MsgReqType: MRTReq,
				IsCS:       true,
				ResMsgID:   "test.res",
			},
			expected: struct {
				isNtf    bool
				isSSReq  bool
				isSSRes  bool
				isReq    bool
				isRes    bool
				resMsgID string
				msgID    string
			}{
				isNtf:    false,
				isSSReq:  false,
				isSSRes:  false,
				isReq:    true,
				isRes:    false,
				resMsgID: "test.res",
				msgID:    "test.req",
			},
		},
		{
			name: "server-to-server request",
			pi: &MsgProtoInfo{
				MsgID:      "test.ssreq",
				MsgReqType: MRTReq,
				IsCS:       false,
				ResMsgID:   "test.ssres",
			},
			expected: struct {
				isNtf    bool
				isSSReq  bool
				isSSRes  bool
				isReq    bool
				isRes    bool
				resMsgID string
				msgID    string
			}{
				isNtf:    false,
				isSSReq:  true,
				isSSRes:  false,
				isReq:    true,
				isRes:    false,
				resMsgID: "test.ssres",
				msgID:    "test.ssreq",
			},
		},
		{
			name: "server-to-server response",
			pi: &MsgProtoInfo{
				MsgID:      "test.ssres",
				MsgReqType: MRTRes,
				IsCS:       false,
			},
			expected: struct {
				isNtf    bool
				isSSReq  bool
				isSSRes  bool
				isReq    bool
				isRes    bool
				resMsgID string
				msgID    string
			}{
				isNtf:    false,
				isSSReq:  false,
				isSSRes:  true,
				isReq:    false,
				isRes:    true,
				resMsgID: "",
				msgID:    "test.ssres",
			},
		},
		{
			name: "nil MsgProtoInfo",
			pi:   nil,
			expected: struct {
				isNtf    bool
				isSSReq  bool
				isSSRes  bool
				isReq    bool
				isRes    bool
				resMsgID string
				msgID    string
			}{
				isNtf:    false,
				isSSReq:  false,
				isSSRes:  false,
				isReq:    false,
				isRes:    false,
				resMsgID: "",
				msgID:    "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test IsNtf
			if got := tt.pi.IsNtf(); got != tt.expected.isNtf {
				t.Errorf("IsNtf() = %v, want %v", got, tt.expected.isNtf)
			}

			// Test IsSSReq
			if got := tt.pi.IsSSReq(); got != tt.expected.isSSReq {
				t.Errorf("IsSSReq() = %v, want %v", got, tt.expected.isSSReq)
			}

			// Test IsSSRes
			if got := tt.pi.IsSSRes(); got != tt.expected.isSSRes {
				t.Errorf("IsSSRes() = %v, want %v", got, tt.expected.isSSRes)
			}

			// Test IsReq
			if got := tt.pi.IsReq(); got != tt.expected.isReq {
				t.Errorf("IsReq() = %v, want %v", got, tt.expected.isReq)
			}

			// Test IsRes
			if got := tt.pi.IsRes(); got != tt.expected.isRes {
				t.Errorf("IsRes() = %v, want %v", got, tt.expected.isRes)
			}

			// Test GetResMsgID
			if got := tt.pi.GetResMsgID(); got != tt.expected.resMsgID {
				t.Errorf("GetResMsgID() = %v, want %v", got, tt.expected.resMsgID)
			}

			// Test GetMsgID
			if got := tt.pi.GetMsgID(); got != tt.expected.msgID {
				t.Errorf("GetMsgID() = %v, want %v", got, tt.expected.msgID)
			}
		})
	}
}

// Test MsgCreator interface
func TestMsgCreator(t *testing.T) {
	creator := &mockMsgCreatorForMessage{
		messages: map[string]proto.Message{
			"test.msg1": &mockProtoMessage{msgID: "test.msg1"},
			"test.msg2": &mockProtoMessage{msgID: "test.msg2"},
		},
	}

	tests := []struct {
		name    string
		msgID   string
		wantErr bool
	}{
		{
			name:    "existing message",
			msgID:   "test.msg1",
			wantErr: false,
		},
		{
			name:    "non-existing message",
			msgID:   "test.msg3",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := creator.CreateMsg(tt.msgID)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateMsg() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && msg == nil {
				t.Error("CreateMsg() returned nil message")
			}
		})
	}

	// Test ContainsMsg
	if !creator.ContainsMsg("test.msg1") {
		t.Error("ContainsMsg() should return true for existing message")
	}
	if creator.ContainsMsg("test.msg3") {
		t.Error("ContainsMsg() should return false for non-existing message")
	}
}

// Test NewPkgHead
func TestNewPkgHead(t *testing.T) {
	msgID := "test.message"
	head := NewPkgHead(msgID)

	if head == nil {
		t.Fatal("NewPkgHead() returned nil")
	}
	if head.MsgID != msgID {
		t.Errorf("NewPkgHead() MsgID = %v, want %v", head.MsgID, msgID)
	}
}

// Test AsuraMsg interface
func TestAsuraMsgInterface(t *testing.T) {
	msg := &mockAsuraMsg{
		mockProtoMessage: mockProtoMessage{msgID: "test.msg"},
		isRequest:        true,
		isResponse:       false,
	}

	if msg.MsgID() != "test.msg" {
		t.Errorf("MsgID() = %v, want %v", msg.MsgID(), "test.msg")
	}
	if !msg.IsRequest() {
		t.Error("IsRequest() should return true")
	}
	if msg.IsResponse() {
		t.Error("IsResponse() should return false")
	}
}

// Test SCNtfMsg interface
func TestSCNtfMsgInterface(t *testing.T) {
	msg := &mockSCNtfMsg{
		mockAsuraMsg: mockAsuraMsg{
			mockProtoMessage: mockProtoMessage{msgID: "test.ntf"},
			isRequest:        false,
			isResponse:       false,
		},
	}

	if msg.SCNtfMsgID() != "test.ntf" {
		t.Errorf("SCNtfMsgID() = %v, want %v", msg.SCNtfMsgID(), "test.ntf")
	}

	// Test that methods can be called without panic
	msg.NtfClient(nil, 1, 100)
	msg.DeferNtfClient(nil, 1, 100)
}

// Test SSMsg interface
func TestSSMsgInterface(t *testing.T) {
	msg := &mockSSMsg{
		mockAsuraMsg: mockAsuraMsg{
			mockProtoMessage: mockProtoMessage{msgID: "test.ss"},
			isRequest:        true,
			isResponse:       false,
		},
	}

	if !msg.IsSS() {
		t.Error("IsSS() should return true")
	}
}

// Test RPCReqMsg interface
func TestRPCReqMsgInterface(t *testing.T) {
	msg := &mockRPCReqMsg{
		mockAsuraMsg: mockAsuraMsg{
			mockProtoMessage: mockProtoMessage{msgID: "test.rpc"},
			isRequest:        true,
			isResponse:       false,
		},
		resMsgID: "test.rpc.res",
	}

	if msg.ResMsgID() != "test.rpc.res" {
		t.Errorf("ResMsgID() = %v, want %v", msg.ResMsgID(), "test.rpc.res")
	}

	resMsg := msg.CreateResMsg()
	if resMsg == nil {
		t.Fatal("CreateResMsg() returned nil")
	}
	// Use type assertion to access the String method
	if mockMsg, ok := resMsg.(*mockProtoMessage); ok {
		if mockMsg.String() != "test.rpc.res" {
			t.Errorf("CreateResMsg() message ID = %v, want %v", mockMsg.String(), "test.rpc.res")
		}
	} else {
		t.Error("CreateResMsg() should return *mockProtoMessage")
	}
}

// Test NewResPkg with valid parameters
func TestNewResPkgValid(t *testing.T) {
	// Setup server address for testing
	_ = utils.SetupServerAddr("1.1.1.1")

	reqPkg := &TransRecvPkg{
		PkgHdr: &PkgHead{
			SrcActorID: 100,
			DstActorID: 200,
		},
		RouteHdr: &RouteHead{
			SrcEntityID: 300,
		},
	}

	resMsgID := "test.response"
	retCode := int32(0)
	resBody := &mockProtoMessage{msgID: resMsgID}

	resPkg, err := NewResPkg(reqPkg, resMsgID, retCode, resBody)
	if err != nil {
		t.Fatalf("NewResPkg() error = %v", err)
	}

	if resPkg == nil {
		t.Fatal("NewResPkg() returned nil package")
	}

	// Verify response message ID
	if resPkg.PkgHdr.MsgID != resMsgID {
		t.Errorf("PkgHdr.MsgID = %v, want %v", resPkg.PkgHdr.MsgID, resMsgID)
	}

	// Verify return code
	if resPkg.PkgHdr.RetCode != retCode {
		t.Errorf("PkgHdr.RetCode = %v, want %v", resPkg.PkgHdr.RetCode, retCode)
	}

	// Verify actor ID swapping
	if resPkg.PkgHdr.DstActorID != reqPkg.PkgHdr.GetSrcActorID() {
		t.Errorf("PkgHdr.DstActorID = %v, want %v", resPkg.PkgHdr.DstActorID, reqPkg.PkgHdr.GetSrcActorID())
	}
	if resPkg.PkgHdr.SrcActorID != reqPkg.PkgHdr.GetDstActorID() {
		t.Errorf("PkgHdr.SrcActorID = %v, want %v", resPkg.PkgHdr.SrcActorID, reqPkg.PkgHdr.GetDstActorID())
	}

	// Verify routing information
	if resPkg.RouteHdr.GetSrcEntityID() == 0 {
		t.Error("RouteHdr.SrcEntityID should not be 0")
	}
	if resPkg.RouteHdr.GetMsgID() != resMsgID {
		t.Errorf("RouteHdr.MsgID = %v, want %v", resPkg.RouteHdr.GetMsgID(), resMsgID)
	}

	// Verify P2P routing
	p2pRoute, ok := resPkg.RouteHdr.RouteType.(*RouteHead_P2P)
	if !ok {
		t.Fatal("RouteType should be P2P")
	}
	if p2pRoute.P2P.GetDstEntityID() != reqPkg.RouteHdr.GetSrcEntityID() {
		t.Errorf("P2P.DstEntityID = %v, want %v", p2pRoute.P2P.GetDstEntityID(), reqPkg.RouteHdr.GetSrcEntityID())
	}

	// Verify body
	if resPkg.Body != resBody {
		t.Error("Body should match the provided response body")
	}
}

// Test NewResPkg error cases
func TestNewResPkgErrors(t *testing.T) {
	tests := []struct {
		name     string
		reqPkg   *TransRecvPkg
		resMsgID string
		wantErr  bool
	}{
		{
			name:     "nil request package",
			reqPkg:   nil,
			resMsgID: "test.res",
			wantErr:  true,
		},
		{
			name:     "empty response message ID",
			reqPkg:   &TransRecvPkg{},
			resMsgID: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resBody := &mockProtoMessage{}
			_, err := NewResPkg(tt.reqPkg, tt.resMsgID, 0, resBody)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewResPkg() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test NewSCNtfPkg
func TestNewSCNtfPkg(t *testing.T) {
	// Setup server address for testing
	_ = utils.SetupServerAddr("1.1.1.1")

	msg := &mockSCNtfMsg{
		mockAsuraMsg: mockAsuraMsg{
			mockProtoMessage: mockProtoMessage{msgID: "test.ntf"},
		},
	}
	dstEntity := uint32(100)
	dstActorID := uint64(200)

	pkg := NewSCNtfPkg(msg, dstEntity, dstActorID)

	// Verify package header
	if pkg.PkgHdr.MsgID != msg.MsgID() {
		t.Errorf("PkgHdr.MsgID = %v, want %v", pkg.PkgHdr.MsgID, msg.MsgID())
	}
	if pkg.PkgHdr.GetDstActorID() != dstActorID {
		t.Errorf("PkgHdr.DstActorID = %v, want %v", pkg.PkgHdr.GetDstActorID(), dstActorID)
	}

	// Verify body
	if pkg.Body != msg {
		t.Error("Body should be the notification message")
	}

	// Test cross-entity routing
	if pkg.RouteHdr == nil {
		t.Fatal("RouteHdr should not be nil for cross-entity routing")
	}
	if pkg.RouteHdr.GetSrcEntityID() == 0 {
		t.Error("RouteHdr.SrcEntityID should not be 0")
	}

	p2pRoute, ok := pkg.RouteHdr.RouteType.(*RouteHead_P2P)
	if !ok {
		t.Fatal("RouteType should be P2P")
	}
	if p2pRoute.P2P.GetDstEntityID() != dstEntity {
		t.Errorf("P2P.DstEntityID = %v, want %v", p2pRoute.P2P.GetDstEntityID(), dstEntity)
	}
}

// Test NewPostLocalPkg
func TestNewPostLocalPkg(t *testing.T) {
	// Setup server address for testing
	_ = utils.SetupServerAddr("1.1.1.1")

	msg := &mockAsuraMsg{
		mockProtoMessage: mockProtoMessage{msgID: "test.local"},
	}

	pkg := NewPostLocalPkg(msg)

	// Verify package header
	if pkg.PkgHdr.MsgID != msg.MsgID() {
		t.Errorf("PkgHdr.MsgID = %v, want %v", pkg.PkgHdr.MsgID, msg.MsgID())
	}

	// Verify body
	if pkg.Body != msg {
		t.Error("Body should be the local message")
	}

	// Verify routing information
	if pkg.RouteHdr == nil {
		t.Fatal("RouteHdr should not be nil")
	}
	if pkg.RouteHdr.GetSrcEntityID() != 0 {
		t.Error("RouteHdr.SrcEntityID should be 0 for local messages")
	}
	if pkg.RouteHdr.GetMsgID() != msg.MsgID() {
		t.Errorf("RouteHdr.MsgID = %v, want %v", pkg.RouteHdr.GetMsgID(), msg.MsgID())
	}

	p2pRoute, ok := pkg.RouteHdr.RouteType.(*RouteHead_P2P)
	if !ok {
		t.Fatal("RouteType should be P2P")
	}
	if p2pRoute.P2P.GetDstEntityID() == 0 {
		t.Error("P2P.DstEntityID should not be 0 for local messages")
	}
}

// Test NewP2PPkg
func TestNewP2PPkg(t *testing.T) {
	// Setup server address for testing
	_ = utils.SetupServerAddr("1.1.1.1")

	msg := &mockSSMsg{
		mockAsuraMsg: mockAsuraMsg{
			mockProtoMessage: mockProtoMessage{msgID: "test.ss"},
		},
	}
	dstID := uint32(500)

	pkg := NewP2PPkg(msg, dstID)

	// Verify package header
	if pkg.PkgHdr.MsgID != msg.MsgID() {
		t.Errorf("PkgHdr.MsgID = %v, want %v", pkg.PkgHdr.MsgID, msg.MsgID())
	}

	// Verify body
	if pkg.Body != msg {
		t.Error("Body should be the server-to-server message")
	}

	// Verify routing information
	if pkg.RouteHdr == nil {
		t.Fatal("RouteHdr should not be nil")
	}
	if pkg.RouteHdr.GetSrcEntityID() == 0 {
		t.Error("RouteHdr.SrcEntityID should not be 0")
	}
	if pkg.RouteHdr.GetMsgID() != msg.MsgID() {
		t.Errorf("RouteHdr.MsgID = %v, want %v", pkg.RouteHdr.GetMsgID(), msg.MsgID())
	}

	p2pRoute, ok := pkg.RouteHdr.RouteType.(*RouteHead_P2P)
	if !ok {
		t.Fatal("RouteType should be P2P")
	}
	if p2pRoute.P2P.GetDstEntityID() != dstID {
		t.Errorf("P2P.DstEntityID = %v, want %v", p2pRoute.P2P.GetDstEntityID(), dstID)
	}
}

// Test NewP2PPkgActor
func TestNewP2PPkgActor(t *testing.T) {
	msg := &mockSSMsg{
		mockAsuraMsg: mockAsuraMsg{
			mockProtoMessage: mockProtoMessage{msgID: "test.ss.actor"},
		},
	}
	dstEntityID := uint32(600)
	dstActorID := uint64(700)

	pkg := NewP2PPkgActor(msg, dstEntityID, dstActorID)

	// Verify package header
	if pkg.PkgHdr.MsgID != msg.MsgID() {
		t.Errorf("PkgHdr.MsgID = %v, want %v", pkg.PkgHdr.MsgID, msg.MsgID())
	}
	if pkg.PkgHdr.GetDstActorID() != dstActorID {
		t.Errorf("PkgHdr.DstActorID = %v, want %v", pkg.PkgHdr.GetDstActorID(), dstActorID)
	}

	// Verify routing information
	if pkg.RouteHdr == nil {
		t.Fatal("RouteHdr should not be nil")
	}
	if pkg.RouteHdr.GetSrcEntityID() == 0 {
		t.Error("RouteHdr.SrcEntityID should not be 0")
	}
	if pkg.RouteHdr.GetMsgID() != msg.MsgID() {
		t.Errorf("RouteHdr.MsgID = %v, want %v", pkg.RouteHdr.GetMsgID(), msg.MsgID())
	}

	p2pRoute, ok := pkg.RouteHdr.RouteType.(*RouteHead_P2P)
	if !ok {
		t.Fatal("RouteType should be P2P")
	}
	if p2pRoute.P2P.GetDstEntityID() != dstEntityID {
		t.Errorf("P2P.DstEntityID = %v, want %v", p2pRoute.P2P.GetDstEntityID(), dstEntityID)
	}
}

// Benchmark tests
func BenchmarkNewResPkg(b *testing.B) {
	reqPkg := &TransRecvPkg{
		PkgHdr: &PkgHead{
			SrcActorID: 100,
			DstActorID: 200,
		},
		RouteHdr: &RouteHead{
			SrcEntityID: 300,
		},
	}
	resBody := &mockProtoMessage{msgID: "bench.res"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewResPkg(reqPkg, "bench.res", 0, resBody)
		if err != nil {
			b.Fatalf("NewResPkg() error = %v", err)
		}
	}
}

func BenchmarkNewSCNtfPkg(b *testing.B) {
	msg := &mockSCNtfMsg{
		mockAsuraMsg: mockAsuraMsg{
			mockProtoMessage: mockProtoMessage{msgID: "bench.ntf"},
		},
	}
	dstEntity := uint32(100)
	dstActorID := uint64(200)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewSCNtfPkg(msg, dstEntity, dstActorID)
	}
}

func BenchmarkNewP2PPkg(b *testing.B) {
	msg := &mockSSMsg{
		mockAsuraMsg: mockAsuraMsg{
			mockProtoMessage: mockProtoMessage{msgID: "bench.ss"},
		},
	}
	dstID := uint32(500)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewP2PPkg(msg, dstID)
	}
}
