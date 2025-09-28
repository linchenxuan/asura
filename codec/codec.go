// Package codec ...
package codec

import (
	"errors"

	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	errCodecNotInit = errors.New("codec not init")

	_codec Codec = &DefaultCodec{}
)

// Codec 解码器.
type Codec interface {
	Encode(m protoreflect.ProtoMessage, b []byte) ([]byte, error)
	Decode(a any, b []byte) error
}

// Encode 打包.
func Encode(m protoreflect.ProtoMessage, b []byte) ([]byte, error) {
	if _codec == nil {
		return nil, errCodecNotInit
	}
	return _codec.Encode(m, b)
}

// Decode 解包.
func Decode(a any, b []byte) error {
	if _codec == nil {
		return errCodecNotInit
	}
	return _codec.Decode(a, b)
}

// SetCodec 设置解码器.
func SetCodec(c Codec) {
	_codec = c
}
