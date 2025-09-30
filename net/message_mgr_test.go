package net

import (
	"testing"

	"google.golang.org/protobuf/proto"
)

// TestNewMessageManager 测试创建新的消息管理器
func TestNewMessageManager(t *testing.T) {
	mgr := NewMessageManager()
	if mgr == nil {
		t.Fatal("NewMessageManager() returned nil")
	}
	if mgr.PropInfoMap == nil {
		t.Fatal("PropInfoMap not initialized")
	}
	if len(mgr.PropInfoMap) != 0 {
		t.Fatalf("Expected empty PropInfoMap, got %d items", len(mgr.PropInfoMap))
	}
}

// TestRegisterMsgInfo 测试注册完整消息协议信息
func TestRegisterMsgInfo(t *testing.T) {
	mgr := NewMessageManager()

	tests := []struct {
		name     string
		pi       *MsgProtoInfo
		wantRegs int
	}{
		{
			name:     "register valid message info",
			pi:       &MsgProtoInfo{MsgID: "test.msg", New: func() proto.Message { return &PkgHead{MsgID: "test.msg"} }},
			wantRegs: 1,
		},
		{
			name:     "register nil message info",
			pi:       nil,
			wantRegs: 0,
		},
		{
			name:     "register empty msgid",
			pi:       &MsgProtoInfo{MsgID: "", New: func() proto.Message { return &PkgHead{} }},
			wantRegs: 0,
		},
		{
			name:     "register nil New function",
			pi:       &MsgProtoInfo{MsgID: "test.msg", New: nil},
			wantRegs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialLen := len(mgr.PropInfoMap)
			mgr.RegisterMsgInfo(tt.pi)

			if len(mgr.PropInfoMap) != initialLen+tt.wantRegs {
				t.Errorf("RegisterMsgInfo() resulted in %d registrations, want %d",
					len(mgr.PropInfoMap)-initialLen, tt.wantRegs)
			}
		})
	}
}

// TestRegisterMsgInfoPreserveHandler 测试注册时保留现有的处理器和层类型
func TestRegisterMsgInfoPreserveHandler(t *testing.T) {
	mgr := NewMessageManager()

	// 先注册带处理器的信息
	originalHandler := func(msg proto.Message) error { return nil }
	originalLayerType := MsgLayerType_Stateful

	initialInfo := &MsgProtoInfo{
		MsgID:        "test.msg",
		New:          func() proto.Message { return &PkgHead{MsgID: "test.msg"} },
		MsgHandle:    originalHandler,
		MsgLayerType: originalLayerType,
	}
	mgr.RegisterMsgInfo(initialInfo)

	// 重新注册，更新其他信息但保留处理器
	newInfo := &MsgProtoInfo{
		MsgID:      "test.msg",
		New:        func() proto.Message { return &PkgHead{MsgID: "test.msg"} },
		MsgReqType: MRTReq,
		IsCS:       true,
	}
	mgr.RegisterMsgInfo(newInfo)

	// 验证处理器和层类型被保留
	protoInfo, ok := mgr.GetProtoInfo("test.msg")
	if !ok {
		t.Fatal("Message not found after re-registration")
	}

	// 注意：函数指针比较在Go中有限制，这里只检查处理器不为nil
	if protoInfo.MsgHandle == nil {
		t.Error("MsgHandle was not preserved during re-registration")
	}
	if protoInfo.MsgLayerType != originalLayerType {
		t.Error("MsgLayerType was not preserved during re-registration")
	}
	if protoInfo.MsgReqType != MRTReq {
		t.Error("MsgReqType was not updated during re-registration")
	}
	if protoInfo.IsCS != true {
		t.Error("IsCS was not updated during re-registration")
	}
}

// TestRegisterMsgHandle 测试注册消息处理器
func TestRegisterMsgHandle(t *testing.T) {
	mgr := NewMessageManager()

	tests := []struct {
		name         string
		msgid        string
		handle       any
		msgLayerType MsgLayerType
		setup        func()
		wantRegs     int
	}{
		{
			name:         "register new handler",
			msgid:        "test.msg",
			handle:       func(msg proto.Message) error { return nil },
			msgLayerType: MsgLayerType_Stateless,
			wantRegs:     1,
		},
		{
			name:         "register empty msgid",
			msgid:        "",
			handle:       func(msg proto.Message) error { return nil },
			msgLayerType: MsgLayerType_Stateless,
			wantRegs:     0,
		},
		{
			name:         "update existing handler",
			msgid:        "test.msg",
			handle:       func(msg proto.Message) error { return nil },
			msgLayerType: MsgLayerType_Stateful,
			setup: func() {
				mgr.RegisterMsgHandle("test.msg", func(msg proto.Message) error { return nil }, MsgLayerType_Stateless)
			},
			wantRegs: 0, // 不增加新注册，只是更新
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			initialLen := len(mgr.PropInfoMap)

			mgr.RegisterMsgHandle(tt.msgid, tt.handle, tt.msgLayerType)

			if len(mgr.PropInfoMap) != initialLen+tt.wantRegs {
				t.Errorf("RegisterMsgHandle() resulted in %d registrations, want %d",
					len(mgr.PropInfoMap)-initialLen, tt.wantRegs)
			}
		})
	}
}

// TestGetProtoInfo 测试获取协议信息
func TestGetProtoInfo(t *testing.T) {
	mgr := NewMessageManager()

	// 注册测试消息
	testInfo := &MsgProtoInfo{
		MsgID:      "test.msg",
		New:        func() proto.Message { return &PkgHead{MsgID: "test.msg"} },
		MsgReqType: MRTReq,
	}
	mgr.RegisterMsgInfo(testInfo)

	tests := []struct {
		name    string
		msgID   string
		wantOk  bool
		checkFn func(*MsgProtoInfo)
	}{
		{
			name:   "get existing message info",
			msgID:  "test.msg",
			wantOk: true,
			checkFn: func(info *MsgProtoInfo) {
				if info.MsgID != "test.msg" {
					t.Error("Incorrect MsgID")
				}
				if info.MsgReqType != MRTReq {
					t.Error("Incorrect MsgReqType")
				}
			},
		},
		{
			name:   "get non-existing message info",
			msgID:  "nonexistent.msg",
			wantOk: false,
		},
		{
			name:   "get empty msgid",
			msgID:  "",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, ok := mgr.GetProtoInfo(tt.msgID)
			if ok != tt.wantOk {
				t.Errorf("GetProtoInfo() ok = %v, want %v", ok, tt.wantOk)
			}
			if tt.checkFn != nil && ok {
				tt.checkFn(info)
			}
		})
	}
}

// TestCreateMsg 测试创建消息实例
func TestCreateMsg(t *testing.T) {
	mgr := NewMessageManager()

	// 注册带工厂函数的消息
	mgr.RegisterMsgInfo(&MsgProtoInfo{
		MsgID: "test.msg",
		New:   func() proto.Message { return &PkgHead{MsgID: "test.msg"} },
	})

	tests := []struct {
		name    string
		msgID   string
		wantErr bool
		checkFn func(proto.Message)
	}{
		{
			name:    "create existing message",
			msgID:   "test.msg",
			wantErr: false,
			checkFn: func(msg proto.Message) {
				if msg == nil {
					t.Error("Created message is nil")
				}
				if pkgHead, ok := msg.(*PkgHead); ok {
					if pkgHead.MsgID != "test.msg" {
						t.Error("Incorrect message content")
					}
				} else {
					t.Error("Incorrect message type")
				}
			},
		},
		{
			name:    "create non-existing message",
			msgID:   "nonexistent.msg",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := mgr.CreateMsg(tt.msgID)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateMsg() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.checkFn != nil && !tt.wantErr {
				tt.checkFn(msg)
			}
		})
	}
}

// TestContainsMsg 测试检查消息是否存在
func TestContainsMsg(t *testing.T) {
	mgr := NewMessageManager()

	// 注册测试消息
	mgr.RegisterMsgInfo(&MsgProtoInfo{
		MsgID: "test.msg",
		New:   func() proto.Message { return &PkgHead{MsgID: "test.msg"} },
	})

	tests := []struct {
		name  string
		msgID string
		want  bool
	}{
		{
			name:  "check existing message",
			msgID: "test.msg",
			want:  true,
		},
		{
			name:  "check non-existing message",
			msgID: "nonexistent.msg",
			want:  false,
		},
		{
			name:  "check empty msgid",
			msgID: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mgr.ContainsMsg(tt.msgID); got != tt.want {
				t.Errorf("ContainsMsg() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestMessageTypeChecks 测试各种消息类型判断函数
func TestMessageTypeChecks(t *testing.T) {
	mgr := NewMessageManager()

	// 注册不同类型的消息
	mgr.RegisterMsgInfo(&MsgProtoInfo{
		MsgID:      "test.req",
		New:        func() proto.Message { return &PkgHead{MsgID: "test.req"} },
		MsgReqType: MRTReq,
	})

	mgr.RegisterMsgInfo(&MsgProtoInfo{
		MsgID:      "test.res",
		New:        func() proto.Message { return &PkgHead{MsgID: "test.res"} },
		MsgReqType: MRTRes,
		IsCS:       true, // 客户端-服务器消息
	})

	mgr.RegisterMsgInfo(&MsgProtoInfo{
		MsgID:      "test.ntf",
		New:        func() proto.Message { return &PkgHead{MsgID: "test.ntf"} },
		MsgReqType: MRTNtf,
	})

	mgr.RegisterMsgInfo(&MsgProtoInfo{
		MsgID:      "test.ssreq",
		New:        func() proto.Message { return &PkgHead{MsgID: "test.ssreq"} },
		MsgReqType: MRTReq,
		IsCS:       false, // 服务器间消息
	})

	mgr.RegisterMsgInfo(&MsgProtoInfo{
		MsgID:      "test.ssres",
		New:        func() proto.Message { return &PkgHead{MsgID: "test.ssres"} },
		MsgReqType: MRTRes,
		IsCS:       false, // 服务器间消息
	})

	mgr.RegisterMsgInfo(&MsgProtoInfo{
		MsgID:      "test.csntf",
		New:        func() proto.Message { return &PkgHead{MsgID: "test.csntf"} },
		MsgReqType: MRTNtf,
		IsCS:       true, // 客户端-服务器消息
	})

	tests := []struct {
		name    string
		msgID   string
		checkFn func(string) bool
		want    bool
	}{
		{
			name:    "IsRequestMsg - request type",
			msgID:   "test.req",
			checkFn: mgr.IsRequestMsg,
			want:    true,
		},
		{
			name:    "IsRequestMsg - non-request type",
			msgID:   "test.ntf",
			checkFn: mgr.IsRequestMsg,
			want:    false,
		},
		{
			name:    "IsNtfMsg - notification type",
			msgID:   "test.ntf",
			checkFn: mgr.IsNtfMsg,
			want:    true,
		},
		{
			name:    "IsNtfMsg - non-notification type",
			msgID:   "test.req",
			checkFn: mgr.IsNtfMsg,
			want:    false,
		},
		{
			name:    "IsSSRequestMsg - server-to-server request",
			msgID:   "test.ssreq",
			checkFn: mgr.IsSSRequestMsg,
			want:    true,
		},
		{
			name:    "IsSSRequestMsg - client-server request",
			msgID:   "cs_req",
			checkFn: mgr.IsSSRequestMsg,
			want:    false, // CS request should return false for IsSSRequestMsg
		},
		{
			name:    "IsSSResMsg - server-to-server response",
			msgID:   "test.ssres",
			checkFn: mgr.IsSSResMsg,
			want:    true,
		},
		{
			name:    "IsSSResMsg - client-server response",
			msgID:   "test.res",
			checkFn: mgr.IsSSResMsg,
			want:    false,
		},
		{
			name:    "IsCSNtfMsg - client-server notification",
			msgID:   "test.csntf",
			checkFn: mgr.IsCSNtfMsg,
			want:    true,
		},
		{
			name:    "IsCSNtfMsg - server-to-server notification",
			msgID:   "test.ntf",
			checkFn: mgr.IsCSNtfMsg,
			want:    false,
		},
		{
			name:    "check non-existing message",
			msgID:   "nonexistent.msg",
			checkFn: mgr.IsRequestMsg,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.checkFn(tt.msgID); got != tt.want {
				t.Errorf("%s() = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// TestGetAllMsgList 测试获取消息列表
func TestGetAllMsgList(t *testing.T) {
	mgr := NewMessageManager()

	// 注册多个消息
	msgIDs := []string{"msg1", "msg2", "msg3", "msg4", "msg5"}
	for _, msgID := range msgIDs {
		mgr.RegisterMsgInfo(&MsgProtoInfo{
			MsgID:      msgID,
			New:        func() proto.Message { return &PkgHead{MsgID: msgID} },
			MsgReqType: MRTReq,
			IsCS:       true, // 客户端-服务器消息
		})
	}

	// 注册一些特殊类型的消息
	mgr.RegisterMsgInfo(&MsgProtoInfo{
		MsgID:      "ntf1",
		New:        func() proto.Message { return &PkgHead{MsgID: "ntf1"} },
		MsgReqType: MRTNtf,
		IsCS:       true, // 客户端-服务器消息
	})

	mgr.RegisterMsgInfo(&MsgProtoInfo{
		MsgID:      "ssreq1",
		New:        func() proto.Message { return &PkgHead{MsgID: "ssreq1"} },
		MsgReqType: MRTReq,
		IsCS:       false, // 服务器间消息
	})

	tests := []struct {
		name      string
		checkFunc func(*MsgProtoInfo) bool
		wantLen   int
		wantIDs   []string
	}{
		{
			name:      "get all messages",
			checkFunc: func(*MsgProtoInfo) bool { return true },
			wantLen:   7, // 6个普通消息 + 1个通知消息
		},
		{
			name: "get only request messages",
			checkFunc: func(pi *MsgProtoInfo) bool {
				return pi.MsgReqType == MRTReq
			},
			wantLen: 6, // 5个普通请求 + 1个服务器间请求
		},
		{
			name: "get only notification messages",
			checkFunc: func(pi *MsgProtoInfo) bool {
				return pi.MsgReqType == MRTNtf
			},
			wantLen: 1,
		},
		{
			name: "get only server-to-server messages",
			checkFunc: func(pi *MsgProtoInfo) bool {
				return pi.IsSSReq() || pi.IsSSRes()
			},
			wantLen: 1, // 只有ssreq1是服务器间消息
		},
		{
			name: "get messages with specific ID prefix",
			checkFunc: func(pi *MsgProtoInfo) bool {
				return len(pi.MsgID) >= 3 && pi.MsgID[:3] == "msg"
			},
			wantLen: 5,
		},
		{
			name:      "get messages with no matches",
			checkFunc: func(pi *MsgProtoInfo) bool { return false },
			wantLen:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mgr.GetAllMsgList(tt.checkFunc)
			if len(result) != tt.wantLen {
				t.Errorf("GetAllMsgList() returned %d messages, want %d", len(result), tt.wantLen)
			}
		})
	}
}

// TestMessageManagerConcurrency 测试消息管理器的并发安全性
func TestMessageManagerConcurrency(t *testing.T) {
	mgr := NewMessageManager()
	done := make(chan bool)

	// 并发注册消息
	go func() {
		for i := 0; i < 100; i++ {
			msgID := "msg1"
			mgr.RegisterMsgInfo(&MsgProtoInfo{
				MsgID: msgID,
				New:   func() proto.Message { return &PkgHead{MsgID: msgID} },
			})
		}
		done <- true
	}()

	// 并发读取消息信息
	go func() {
		for i := 0; i < 100; i++ {
			mgr.ContainsMsg("msg1")
			mgr.GetProtoInfo("msg1")
		}
		done <- true
	}()

	// 等待两个goroutine完成
	for i := 0; i < 2; i++ {
		<-done
	}

	// 验证最终状态
	if !mgr.ContainsMsg("msg1") {
		t.Error("Message should exist after concurrent operations")
	}
}

// BenchmarkRegisterMsgInfo 基准测试注册消息信息
func BenchmarkRegisterMsgInfo(b *testing.B) {
	mgr := NewMessageManager()
	pi := &MsgProtoInfo{
		MsgID: "bench.msg",
		New:   func() proto.Message { return &PkgHead{MsgID: "bench.msg"} },
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.RegisterMsgInfo(pi)
	}
}

// BenchmarkGetProtoInfo 基准测试获取协议信息
func BenchmarkGetProtoInfo(b *testing.B) {
	mgr := NewMessageManager()
	mgr.RegisterMsgInfo(&MsgProtoInfo{
		MsgID: "bench.msg",
		New:   func() proto.Message { return &PkgHead{MsgID: "bench.msg"} },
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.GetProtoInfo("bench.msg")
	}
}

// BenchmarkCreateMsg 基准测试创建消息
func BenchmarkCreateMsg(b *testing.B) {
	mgr := NewMessageManager()
	mgr.RegisterMsgInfo(&MsgProtoInfo{
		MsgID: "bench.msg",
		New:   func() proto.Message { return &PkgHead{MsgID: "bench.msg"} },
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.CreateMsg("bench.msg")
	}
}

// BenchmarkContainsMsg 基准测试检查消息存在
func BenchmarkContainsMsg(b *testing.B) {
	mgr := NewMessageManager()
	mgr.RegisterMsgInfo(&MsgProtoInfo{
		MsgID: "bench.msg",
		New:   func() proto.Message { return &PkgHead{MsgID: "bench.msg"} },
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.ContainsMsg("bench.msg")
	}
}

// BenchmarkGetAllMsgList 基准测试获取消息列表
func BenchmarkGetAllMsgList(b *testing.B) {
	mgr := NewMessageManager()

	// 预注册一些消息
	for i := 0; i < 100; i++ {
		msgID := "msg" + string(rune('0'+i%10))
		mgr.RegisterMsgInfo(&MsgProtoInfo{
			MsgID:      msgID,
			New:        func() proto.Message { return &PkgHead{MsgID: msgID} },
			MsgReqType: MRTReq,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.GetAllMsgList(func(pi *MsgProtoInfo) bool {
			return pi.MsgReqType == MRTReq
		})
	}
}
