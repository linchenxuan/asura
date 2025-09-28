package codec

import (
	"encoding/json"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// DefaultCodec ...
type DefaultCodec struct{}

// Encode ...
func (c *DefaultCodec) Encode(m protoreflect.ProtoMessage, b []byte) ([]byte, error) {
	return proto.MarshalOptions{}.MarshalAppend(b, m)
}

// Decode ...
func (c *DefaultCodec) Decode(a any, b []byte) error {
	return json.Unmarshal(b, a)
}
