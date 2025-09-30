package net

import (
	"bytes"
	"errors"
	"testing"

	"github.com/lcx/asura/codec"
	"google.golang.org/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

// mockMsgCreator 用于测试的模拟消息创建器
type mockMsgCreator struct {
	shouldFail bool
}

func (m *mockMsgCreator) CreateMsg(msgID string) (proto.Message, error) {
	if m.shouldFail {
		return nil, errors.New("create message failed")
	}
	return &PkgHead{MsgID: msgID}, nil
}

func (m *mockMsgCreator) ContainsMsg(msgID string) bool {
	return !m.shouldFail
}

// testProtobufCodec 测试专用的protobuf编解码器
type testProtobufCodec struct{}

func (c *testProtobufCodec) Encode(m protoreflect.ProtoMessage, b []byte) ([]byte, error) {
	return proto.MarshalOptions{}.MarshalAppend(b, m)
}

func (c *testProtobufCodec) Decode(a any, b []byte) error {
	// 确保目标是指针类型
	msg, ok := a.(proto.Message)
	if !ok {
		return errors.New("target is not a proto.Message")
	}
	return proto.Unmarshal(b, msg)
}

func init() {
	// 设置测试专用的protobuf编解码器
	codec.SetCodec(&testProtobufCodec{})
}

func Test_EncodeCSMsg(t *testing.T) {
	tests := []struct {
		name    string
		pkg     *TransSendPkg
		wantErr bool
	}{
		{
			name: "正常编码",
			pkg: &TransSendPkg{
				RouteHdr: &RouteHead{},
				PkgHdr:   &PkgHead{MsgID: "test_msg"},
				Body:     &PkgHead{MsgID: "test_msg"},
			},
			wantErr: false,
		},
		{
			name: "空消息体",
			pkg: &TransSendPkg{
				RouteHdr: &RouteHead{},
				PkgHdr:   &PkgHead{MsgID: "empty_msg"},
				Body:     &PkgHead{MsgID: "empty_msg"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encodedBuf, err := EncodeCSMsg(tt.pkg)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncodeCSMsg() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && encodedBuf == nil {
				t.Errorf("EncodeCSMsg() returned nil buffer without error")
			}
		})
	}
}

func Test_PackCSMsg(t *testing.T) {
	tests := []struct {
		name    string
		pkg     *TransSendPkg
		wantErr bool
	}{
		{
			name: "正常打包",
			pkg: &TransSendPkg{
				RouteHdr: &RouteHead{},
				PkgHdr:   &PkgHead{MsgID: "test_msg"},
				Body:     &PkgHead{MsgID: "test_msg"},
			},
			wantErr: false,
		},
		{
			name: "空消息体",
			pkg: &TransSendPkg{
				RouteHdr: &RouteHead{},
				PkgHdr:   &PkgHead{MsgID: "test_msg"},
				Body:     nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := PackCSMsg(tt.pkg, buf)
			if (err != nil) != tt.wantErr {
				t.Errorf("PackCSMsg() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && buf.Len() == 0 {
				t.Errorf("PackCSMsg() produced empty buffer")
			}
		})
	}
}

func Test_PackCSPkg(t *testing.T) {
	tests := []struct {
		name    string
		pkg     *TransSendPkg
		wantErr bool
	}{
		{
			name: "正常打包",
			pkg: &TransSendPkg{
				RouteHdr: &RouteHead{},
				PkgHdr:   &PkgHead{MsgID: "test_msg"},
				Body:     &PkgHead{MsgID: "test_msg"},
			},
			wantErr: false,
		},
		{
			name: "空消息体",
			pkg: &TransSendPkg{
				RouteHdr: &RouteHead{},
				PkgHdr:   &PkgHead{MsgID: "test_msg"},
				Body:     nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			preHeadBuf, err := PackCSPkg(tt.pkg, buf)
			if (err != nil) != tt.wantErr {
				t.Errorf("PackCSPkg() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if buf.Len() == 0 {
					t.Errorf("PackCSPkg() produced empty body buffer")
				}
				if len(preHeadBuf) == 0 {
					t.Errorf("PackCSPkg() produced empty prehead buffer")
				}
			}
		})
	}
}

func Test_DecodeCSMsg(t *testing.T) {
	// 首先创建一个有效的消息包用于测试解码
	pkg := &TransSendPkg{
		RouteHdr: &RouteHead{},
		PkgHdr:   &PkgHead{MsgID: "test_msg"},
		Body:     &PkgHead{MsgID: "test_msg"},
	}

	buf := &bytes.Buffer{}
	err := PackCSMsg(pkg, buf)
	if err != nil {
		t.Fatalf("Failed to pack test message: %v", err)
	}

	tests := []struct {
		name      string
		data      []byte
		creator   MsgCreator
		wantErr   bool
		errString string
	}{
		{
			name:    "正常解码",
			data:    buf.Bytes(),
			creator: &mockMsgCreator{},
			wantErr: false,
		},
		{
			name:      "空创建器",
			data:      buf.Bytes(),
			creator:   nil,
			wantErr:   true,
			errString: "nil MsgCreateor",
		},
		{
			name:      "数据过短",
			data:      []byte{1, 2, 3},
			creator:   &mockMsgCreator{},
			wantErr:   true,
			errString: "data too short for PreHead",
		},
		{
			name: "创建消息失败",
			data: buf.Bytes(),
			creator: &mockMsgCreator{
				shouldFail: true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recvPkg, err := DecodeCSMsg(tt.data, tt.creator)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeCSMsg() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errString != "" && err.Error() != tt.errString {
				t.Errorf("DecodeCSMsg() error = %v, want error containing %v", err, tt.errString)
			}
			if !tt.wantErr && recvPkg == nil {
				t.Errorf("DecodeCSMsg() returned nil package without error")
			}
		})
	}
}

func Test_DecodeCSPkg(t *testing.T) {
	// 创建测试数据
	pkg := &TransSendPkg{
		RouteHdr: &RouteHead{},
		PkgHdr:   &PkgHead{MsgID: "test_msg"},
		Body:     &PkgHead{MsgID: "test_msg"},
	}

	buf := &bytes.Buffer{}
	preHeadBuf, err := PackCSPkg(pkg, buf)
	if err != nil {
		t.Fatalf("Failed to pack test message: %v", err)
	}

	preHead, err := DecodePreHead(preHeadBuf)
	if err != nil {
		t.Fatalf("Failed to decode prehead: %v", err)
	}

	tests := []struct {
		name      string
		preHead   *PreHead
		data      []byte
		creator   MsgCreator
		wantErr   bool
		errString string
	}{
		{
			name:    "正常解码",
			preHead: preHead,
			data:    buf.Bytes(),
			creator: &mockMsgCreator{},
			wantErr: false,
		},
		{
			name: "数据长度不匹配",
			preHead: &PreHead{
				HdrSize:  100, // 设置不匹配的长度
				BodySize: 200,
			},
			data:      buf.Bytes(),
			creator:   &mockMsgCreator{},
			wantErr:   true,
			errString: "data length mismatch with PreHead size, body size and tail size",
		},
		{
			name:      "空创建器",
			preHead:   preHead,
			data:      buf.Bytes(),
			creator:   nil,
			wantErr:   true,
			errString: "nil MsgCreateor",
		},
		{
			name:    "创建消息失败",
			preHead: preHead,
			data:    buf.Bytes(),
			creator: &mockMsgCreator{
				shouldFail: true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recvPkg, err := DecodeCSPkg(tt.preHead, tt.data, tt.creator)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeCSPkg() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errString != "" && err.Error() != tt.errString {
				t.Errorf("DecodeCSPkg() error = %v, want error containing %v", err, tt.errString)
			}
			if !tt.wantErr && recvPkg == nil {
				t.Errorf("DecodeCSPkg() returned nil package without error")
			}
		})
	}
}

func Test_EncodeDecode_RoundTrip(t *testing.T) {
	// 测试完整的编码解码流程
	originalPkg := &TransSendPkg{
		RouteHdr: &RouteHead{
			SrcEntityID: 123,
			RouteType: &RouteHead_P2P{
				P2P: &P2PRoute{DstEntityID: 456},
			},
			MsgID: "test_roundtrip",
		},
		PkgHdr: &PkgHead{
			MsgID:      "test_roundtrip",
			SrcActorID: 789,
			DstActorID: 101112,
			RetCode:    0,
		},
		Body: &PkgHead{MsgID: "test_roundtrip"},
	}

	// 编码
	encodedBuf, err := EncodeCSMsg(originalPkg)
	if err != nil {
		t.Fatalf("Failed to encode message: %v", err)
	}

	// 打包
	fullData := &bytes.Buffer{}
	for _, buf := range encodedBuf {
		fullData.Write(buf)
	}

	// 解码
	creator := &mockMsgCreator{}
	decodedPkg, err := DecodeCSMsg(fullData.Bytes(), creator)
	if err != nil {
		t.Fatalf("Failed to decode message: %v", err)
	}

	// 验证解码结果
	if decodedPkg.PkgHdr.MsgID != originalPkg.PkgHdr.MsgID {
		t.Errorf("MsgID mismatch: got %v, want %v", decodedPkg.PkgHdr.MsgID, originalPkg.PkgHdr.MsgID)
	}
	if decodedPkg.PkgHdr.SrcActorID != originalPkg.PkgHdr.SrcActorID {
		t.Errorf("SrcActorID mismatch: got %v, want %v", decodedPkg.PkgHdr.SrcActorID, originalPkg.PkgHdr.SrcActorID)
	}
	if decodedPkg.PkgHdr.DstActorID != originalPkg.PkgHdr.DstActorID {
		t.Errorf("DstActorID mismatch: got %v, want %v", decodedPkg.PkgHdr.DstActorID, originalPkg.PkgHdr.DstActorID)
	}
}

func Test_PreHead_EncodeDecode(t *testing.T) {
	originalPreHead := &PreHead{
		HdrSize:  100,
		BodySize: 200,
	}

	// 编码
	encoded := EncodePreHead(originalPreHead)
	if len(encoded) != PRE_HEAD_SIZE {
		t.Errorf("Encoded prehead has wrong size: got %d, want %d", len(encoded), PRE_HEAD_SIZE)
	}

	// 解码
	decoded, err := DecodePreHead(encoded)
	if err != nil {
		t.Fatalf("Failed to decode prehead: %v", err)
	}

	// 验证
	if decoded.HdrSize != originalPreHead.HdrSize {
		t.Errorf("HdrSize mismatch: got %v, want %v", decoded.HdrSize, originalPreHead.HdrSize)
	}
	if decoded.BodySize != originalPreHead.BodySize {
		t.Errorf("BodySize mismatch: got %v, want %v", decoded.BodySize, originalPreHead.BodySize)
	}
}

func Test_PreHead_DecodeErrors(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantErr   bool
		errString string
	}{
		{
			name:      "数据过短",
			data:      []byte{1, 2, 3},
			wantErr:   true,
			errString: "buff too small",
		},
		{
			name: "零头部大小",
			data: func() []byte {
				head := &PreHead{HdrSize: 0, BodySize: 100}
				return EncodePreHead(head)
			}(),
			wantErr:   true,
			errString: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodePreHead(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodePreHead() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errString != "" && err.Error() != tt.errString {
				t.Errorf("DecodePreHead() error = %v, want error containing %v", err, tt.errString)
			}
		})
	}
}
