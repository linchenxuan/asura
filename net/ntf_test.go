package net

import (
	"errors"
	"testing"
)

// mockMsgManagerForNtf 模拟 MessageManager 的行为
type mockMsgManagerForNtf struct {
	PropInfoMap map[string]*MsgProtoInfo
}

func newMockMsgManagerForNtf() *mockMsgManagerForNtf {
	return &mockMsgManagerForNtf{
		PropInfoMap: make(map[string]*MsgProtoInfo),
	}
}

func (m *mockMsgManagerForNtf) IsCSNtfMsg(msgID string) bool {
	info, ok := m.PropInfoMap[msgID]
	return ok && info.MsgReqType == MRTNtf && info.IsCS
}

func (m *mockMsgManagerForNtf) IsNtfMsg(msgID string) bool {
	info, ok := m.PropInfoMap[msgID]
	return ok && info.MsgReqType == MRTNtf
}

func (m *mockMsgManagerForNtf) GetProtoInfo(msgID string) (*MsgProtoInfo, bool) {
	info, ok := m.PropInfoMap[msgID]
	return info, ok
}

// mockCSTransportForNtf 模拟 CSTransport 接口
type mockCSTransportForNtf struct {
	lastSentPkg *TransSendPkg
	sendError   error
}

func (m *mockCSTransportForNtf) Start(opts TransportOption) error {
	return nil
}

func (m *mockCSTransportForNtf) StopRecv() error {
	return nil
}

func (m *mockCSTransportForNtf) Stop() error {
	return nil
}

func (m *mockCSTransportForNtf) SendToClient(pkg TransSendPkg) error {
	m.lastSentPkg = &pkg
	return m.sendError
}

// mockSSTransportForNtf 模拟 SSTransport 接口
type mockSSTransportForNtf struct {
	lastSentPkg *TransSendPkg
	sendError   error
}

func (m *mockSSTransportForNtf) Start(opts TransportOption) error {
	return nil
}

func (m *mockSSTransportForNtf) StopRecv() error {
	return nil
}

func (m *mockSSTransportForNtf) Stop() error {
	return nil
}

func (m *mockSSTransportForNtf) SendToServer(pkg TransSendPkg) error {
	m.lastSentPkg = &pkg
	return m.sendError
}

func TestNewNtfSender(t *testing.T) {
	msgManager := &MessageManager{}
	ntfSender := NewNtfSender(msgManager)

	if ntfSender == nil {
		t.Fatal("NewNtfSender should not return nil")
	}

	if ntfSender.msgManager != msgManager {
		t.Error("msgManager not properly set")
	}
}

func TestNtfClient_Success(t *testing.T) {
	msgManager := &MessageManager{
		PropInfoMap: map[string]*MsgProtoInfo{
			"test_csntf": {
				MsgID:      "test_csntf",
				MsgReqType: MRTNtf,
				IsCS:       true,
			},
		},
	}

	ntfSender := NewNtfSender(msgManager)
	csTransport := &mockCSTransportForNtf{}

	pkg := TransSendPkg{
		PkgHdr: &PkgHead{
			MsgID: "test_csntf",
		},
	}

	err := ntfSender.NtfClient(pkg, 123, csTransport)
	if err != nil {
		t.Errorf("NtfClient() unexpected error = %v", err)
	}
	if csTransport.lastSentPkg == nil {
		t.Error("Expected package to be sent")
	}
	if csTransport.lastSentPkg != nil && csTransport.lastSentPkg.PkgHdr.DstActorID != 123 {
		t.Errorf("DstActorID = %v, want %v", csTransport.lastSentPkg.PkgHdr.DstActorID, 123)
	}
}

func TestNtfClient_ZeroActorID(t *testing.T) {
	msgManager := &MessageManager{
		PropInfoMap: map[string]*MsgProtoInfo{
			"test_csntf": {
				MsgID:      "test_csntf",
				MsgReqType: MRTNtf,
				IsCS:       true,
			},
		},
	}

	ntfSender := NewNtfSender(msgManager)
	csTransport := &mockCSTransportForNtf{}

	pkg := TransSendPkg{
		PkgHdr: &PkgHead{
			MsgID:      "test_csntf",
			DstActorID: 50, // Initial value
		},
	}

	err := ntfSender.NtfClient(pkg, 0, csTransport)
	if err != nil {
		t.Errorf("NtfClient() unexpected error = %v", err)
	}
	if csTransport.lastSentPkg == nil {
		t.Error("Expected package to be sent")
	}
	if csTransport.lastSentPkg != nil && csTransport.lastSentPkg.PkgHdr.DstActorID != 50 {
		t.Errorf("DstActorID should not be changed when aid is 0, got %v", csTransport.lastSentPkg.PkgHdr.DstActorID)
	}
}

func TestNtfClient_NilPkgHeader(t *testing.T) {
	msgManager := &MessageManager{}
	csTransport := &mockCSTransportForNtf{}
	ntfSender := NewNtfSender(msgManager)

	pkg := TransSendPkg{} // PkgHdr is nil

	err := ntfSender.NtfClient(pkg, 100, csTransport)
	if err == nil {
		t.Error("NtfClient() expected error for nil PkgHdr")
	}
	if err != nil && err.Error() != "ntf pkg header is nil" {
		t.Errorf("NtfClient() error = %v, want %v", err, "ntf pkg header is nil")
	}
}

func TestNtfClient_NilCSTransport(t *testing.T) {
	msgManager := &MessageManager{
		PropInfoMap: map[string]*MsgProtoInfo{
			"test_csntf": {
				MsgID:      "test_csntf",
				MsgReqType: MRTNtf,
				IsCS:       true,
			},
		},
	}

	ntfSender := NewNtfSender(msgManager)

	pkg := TransSendPkg{
		PkgHdr: &PkgHead{
			MsgID: "test_csntf",
		},
	}

	err := ntfSender.NtfClient(pkg, 100, nil)
	if err == nil {
		t.Error("NtfClient() expected error for nil CSTransport")
	}
	expectedErr := "MsgID:test_csntf ntf CSTransport nil"
	if err != nil && err.Error() != expectedErr {
		t.Errorf("NtfClient() error = %v, want %v", err, expectedErr)
	}
}

func TestNtfClient_NoProtoInfo(t *testing.T) {
	msgManager := &MessageManager{
		PropInfoMap: map[string]*MsgProtoInfo{}, // No proto info
	}

	csTransport := &mockCSTransportForNtf{}
	ntfSender := NewNtfSender(msgManager)

	pkg := TransSendPkg{
		PkgHdr: &PkgHead{
			MsgID: "unknown_msg",
		},
	}

	err := ntfSender.NtfClient(pkg, 100, csTransport)
	if err == nil {
		t.Error("NtfClient() expected error for no proto info")
	}
	expectedErr := "MsgID:unknown_msg ntf no proto info"
	if err != nil && err.Error() != expectedErr {
		t.Errorf("NtfClient() error = %v, want %v", err, expectedErr)
	}
}

func TestNtfClient_NotCSNtfMsg(t *testing.T) {
	msgManager := &MessageManager{
		PropInfoMap: map[string]*MsgProtoInfo{
			"test_req": {
				MsgID:      "test_req",
				MsgReqType: MRTReq,
				IsCS:       true,
			},
		},
	}

	csTransport := &mockCSTransportForNtf{}
	ntfSender := NewNtfSender(msgManager)

	pkg := TransSendPkg{
		PkgHdr: &PkgHead{
			MsgID: "test_req",
		},
	}

	err := ntfSender.NtfClient(pkg, 100, csTransport)
	if err == nil {
		t.Error("NtfClient() expected error for non-CS notification message")
	}
	expectedErr := "MsgID:test_req ntf is not csntf msg"
	if err != nil && err.Error() != expectedErr {
		t.Errorf("NtfClient() error = %v, want %v", err, expectedErr)
	}
}

func TestNtfClient_SendFailed(t *testing.T) {
	msgManager := &MessageManager{
		PropInfoMap: map[string]*MsgProtoInfo{
			"test_csntf": {
				MsgID:      "test_csntf",
				MsgReqType: MRTNtf,
				IsCS:       true,
			},
		},
	}

	expectedSendError := errors.New("send to client failed")
	csTransport := &mockCSTransportForNtf{
		sendError: expectedSendError,
	}

	ntfSender := NewNtfSender(msgManager)

	pkg := TransSendPkg{
		PkgHdr: &PkgHead{
			MsgID: "test_csntf",
		},
	}

	err := ntfSender.NtfClient(pkg, 100, csTransport)
	if err != expectedSendError {
		t.Errorf("NtfClient() error = %v, want %v", err, expectedSendError)
	}
}

func TestNtfServer_Success(t *testing.T) {
	msgManager := &MessageManager{
		PropInfoMap: map[string]*MsgProtoInfo{
			"test_ntf": {
				MsgID:      "test_ntf",
				MsgReqType: MRTNtf,
			},
		},
	}

	ssTransport := &mockSSTransportForNtf{}
	ntfSender := NewNtfSender(msgManager)

	pkg := TransSendPkg{
		PkgHdr: &PkgHead{
			MsgID: "test_ntf",
		},
	}

	err := ntfSender.NtfServer(pkg, ssTransport)
	if err != nil {
		t.Errorf("NtfServer() unexpected error = %v", err)
	}
	if ssTransport.lastSentPkg == nil {
		t.Error("Expected package to be sent")
	}
}

func TestNtfServer_NilPkgHeader(t *testing.T) {
	msgManager := &MessageManager{}
	ssTransport := &mockSSTransportForNtf{}
	ntfSender := NewNtfSender(msgManager)

	pkg := TransSendPkg{} // PkgHdr is nil

	err := ntfSender.NtfServer(pkg, ssTransport)
	if err == nil {
		t.Error("NtfServer() expected error for nil PkgHdr")
	}
	if err != nil && err.Error() != "ntf pkg header is nil" {
		t.Errorf("NtfServer() error = %v, want %v", err, "ntf pkg header is nil")
	}
}

func TestNtfServer_NoProtoInfo(t *testing.T) {
	msgManager := &MessageManager{
		PropInfoMap: map[string]*MsgProtoInfo{}, // No proto info
	}

	ssTransport := &mockSSTransportForNtf{}
	ntfSender := NewNtfSender(msgManager)

	pkg := TransSendPkg{
		PkgHdr: &PkgHead{
			MsgID: "unknown_msg",
		},
	}

	err := ntfSender.NtfServer(pkg, ssTransport)
	if err == nil {
		t.Error("NtfServer() expected error for no proto info")
	}
	expectedErr := "MsgID:unknown_msg ntf no proto info"
	if err != nil && err.Error() != expectedErr {
		t.Errorf("NtfServer() error = %v, want %v", err, expectedErr)
	}
}

func TestNtfServer_NotNtfMsg(t *testing.T) {
	msgManager := &MessageManager{
		PropInfoMap: map[string]*MsgProtoInfo{
			"test_req": {
				MsgID:      "test_req",
				MsgReqType: MRTReq,
			},
		},
	}

	ssTransport := &mockSSTransportForNtf{}
	ntfSender := NewNtfSender(msgManager)

	pkg := TransSendPkg{
		PkgHdr: &PkgHead{
			MsgID: "test_req",
		},
	}

	err := ntfSender.NtfServer(pkg, ssTransport)
	if err == nil {
		t.Error("NtfServer() expected error for non-notification message")
	}
	expectedErr := "MsgID:test_req ntf is not ntf"
	if err != nil && err.Error() != expectedErr {
		t.Errorf("NtfServer() error = %v, want %v", err, expectedErr)
	}
}

func TestNtfServer_SendFailed(t *testing.T) {
	msgManager := &MessageManager{
		PropInfoMap: map[string]*MsgProtoInfo{
			"test_ntf": {
				MsgID:      "test_ntf",
				MsgReqType: MRTNtf,
			},
		},
	}

	expectedSendError := errors.New("send to server failed")
	ssTransport := &mockSSTransportForNtf{
		sendError: expectedSendError,
	}

	ntfSender := NewNtfSender(msgManager)

	pkg := TransSendPkg{
		PkgHdr: &PkgHead{
			MsgID: "test_ntf",
		},
	}

	err := ntfSender.NtfServer(pkg, ssTransport)
	if err != expectedSendError {
		t.Errorf("NtfServer() error = %v, want %v", err, expectedSendError)
	}
}

func BenchmarkNtfClient(b *testing.B) {
	msgManager := &MessageManager{
		PropInfoMap: map[string]*MsgProtoInfo{
			"bench_csntf": {
				MsgID:      "bench_csntf",
				MsgReqType: MRTNtf,
				IsCS:       true,
			},
		},
	}

	ntfSender := NewNtfSender(msgManager)
	csTransport := &mockCSTransportForNtf{}

	pkg := TransSendPkg{
		PkgHdr: &PkgHead{
			MsgID: "bench_csntf",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ntfSender.NtfClient(pkg, 123, csTransport)
	}
}

func BenchmarkNtfServer(b *testing.B) {
	msgManager := &MessageManager{
		PropInfoMap: map[string]*MsgProtoInfo{
			"bench_ntf": {
				MsgID:      "bench_ntf",
				MsgReqType: MRTNtf,
			},
		},
	}

	ntfSender := NewNtfSender(msgManager)
	ssTransport := &mockSSTransportForNtf{}

	pkg := TransSendPkg{
		PkgHdr: &PkgHead{
			MsgID: "bench_ntf",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ntfSender.NtfServer(pkg, ssTransport)
	}
}
