package db

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/lcx/asura/log"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	_BlobKind = map[protoreflect.Kind]struct{}{
		protoreflect.BytesKind:   {},
		protoreflect.MessageKind: {},
		protoreflect.GroupKind:   {},
	}
	_IncreaseAbleKind = map[protoreflect.Kind]struct{}{
		protoreflect.Int32Kind:  {},
		protoreflect.Uint32Kind: {},
		protoreflect.Int64Kind:  {},
		protoreflect.Sint64Kind: {},
		protoreflect.Uint64Kind: {},
	}
	_marshalOptions        = &proto.MarshalOptions{}
	_unmarshalMergeOptions = &proto.UnmarshalOptions{
		Merge: true,
	}
)

// MarshalToMap converts a protobuf message into a map keyed by field name.
// Scalar values are string encoded, while blob-eligible fields keep their wire form.
func MarshalToMap(msg proto.Message, fields []string) (map[string]any, error) {
	rf := msg.ProtoReflect()
	desc := rf.Descriptor()
	rawStrMap := map[string]string{}
	rf.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if !IsFdMarshalToBlob(fd) {
			rawStrMap[string(fd.Name())] = marshalScalar(fd, v)
		}
		return true
	})

	fdFilter := DirtyFilter(desc, fields)
	fds := desc.Fields()
	wireMap := map[string]any{}
	var cur int32
	marshalBytes, err := _marshalOptions.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal to map: %w", err)
	}
	for int(cur) < len(marshalBytes) {
		num, _, n := protowire.ConsumeField(marshalBytes[cur:])
		if n < 0 {
			return nil, fmt.Errorf("wire consume field: ret=%d", n)
		}
		next := cur + int32(n)
		fd := fds.ByNumber(num)
		if fd == nil {
			return nil, fmt.Errorf("unknow field: num=%d", num)
		}
		fdName := string(fd.Name())
		if fdFilter != nil {
			if _, exist := fdFilter[fdName]; !exist {
				cur = next
				continue
			}
		}
		rstr, exist1 := rawStrMap[fdName]
		if exist1 {
			wireMap[fdName] = rstr
		} else {
			cb, exist2 := wireMap[fdName]
			if !exist2 {
				wireMap[fdName] = marshalBytes[cur:next]
			} else {
				wireMap[fdName] = append(cb.([]byte), marshalBytes[cur:next]...) //nolint:forcetypeassert
			}
		}
		cur = next
	}
	return wireMap, nil
}

// DirtyFilter creates a filter set for the provided field names and truncates nested paths.
func DirtyFilter(desc protoreflect.MessageDescriptor,
	fields []string) map[string]struct{} {
	if len(fields) == 0 {
		return nil
	}
	fds := desc.Fields()
	retFilter := map[string]struct{}{}
	for _, fs := range fields {
		retKey := fs
		fd := fds.ByName(protoreflect.Name(fs))
		if fd == nil {
			ir := strings.Index(fs, ".")
			if ir < 0 {
				log.Info().Str("dirty", fs).Str("msg", string(desc.FullName())).Msg("not in msg")
				continue
			}
			retKey = fs[:ir]
			fd = fds.ByName(protoreflect.Name(retKey))
			if fd == nil {
				log.Info().Str("dirty", fs).Str("msg", string(desc.FullName())).Msg("not in msg")
				continue
			}
		}
		retFilter[retKey] = struct{}{}
	}
	return retFilter
}

// UnmarshalFromMap decodes a map produced by MarshalToMap back into the message.
func UnmarshalFromMap(msg proto.Message, bytesMap map[string]string) (err error) {
	buf := bytes.Buffer{}
	rf := msg.ProtoReflect()
	desc := rf.Descriptor()
	fds := desc.Fields()
	for k, s := range bytesMap {
		if strings.HasPrefix(k, "_") {
			continue
		}
		fd := fds.ByName(protoreflect.Name(k))
		if fd == nil {
			log.Info().Str("filedNum", k).
				Str("type", string(rf.Descriptor().FullName())).
				Msg("cannot find filedNum")
			continue
		}
		if IsFdMarshalToBlob(fd) {
			_, e := buf.WriteString(s)
			if e != nil {
				err = fmt.Errorf("merge buf: %w", e)
				return
			}
		} else {
			v, e1 := unmarshalScalarByStr(fd, s)
			if e1 != nil {
				err = e1
				return
			}
			rf.Set(fd, v)
		}
	}
	err = _unmarshalMergeOptions.Unmarshal(buf.Bytes(), msg)
	return
}

// FindPrimaryKey returns the comma-separated primary key extension as an ordered slice.
func FindPrimaryKey(desc protoreflect.Descriptor) []string {
	primKey, ok := proto.GetExtension(desc.Options(), E_PrimaryKey).(string)
	if !ok || len(primKey) == 0 {
		return nil
	}
	return strings.Split(primKey, ",")
}

// FindFds looks up field descriptors for the provided key names while preserving order.
func FindFds(msgDesc protoreflect.MessageDescriptor, keyNames []string) []protoreflect.FieldDescriptor {
	fds := make([]protoreflect.FieldDescriptor, len(keyNames))
	fields := msgDesc.Fields()
	for i, key := range keyNames {
		fds[i] = fields.ByTextName(key)
	}
	return fds
}

//nolint:gocognit,gocyclo
func unmarshalScalarByStr(fd protoreflect.FieldDescriptor, str string) (protoreflect.Value, error) {
	const b32 int = 32
	const b64 int = 64
	const base10 = 10
	kind := fd.Kind()
	switch kind {
	case protoreflect.StringKind:
		return protoreflect.ValueOfString(str), nil

	case protoreflect.BoolKind:
		switch str {
		case "true", "1":
			return protoreflect.ValueOfBool(true), nil
		case "false", "0", "":
			return protoreflect.ValueOfBool(false), nil
		}

	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		if str == "" {
			return protoreflect.ValueOfInt32(0), nil
		}
		if n, err := strconv.ParseInt(str, base10, b32); err == nil {
			return protoreflect.ValueOfInt32(int32(n)), nil
		}

	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		if str == "" {
			return protoreflect.ValueOfInt64(0), nil
		}
		if n, err := strconv.ParseInt(str, base10, b64); err == nil {
			return protoreflect.ValueOfInt64(n), nil
		}

	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		if str == "" {
			return protoreflect.ValueOfUint32(0), nil
		}
		if n, err := strconv.ParseUint(str, base10, b32); err == nil {
			return protoreflect.ValueOfUint32(uint32(n)), nil
		}

	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		if str == "" {
			return protoreflect.ValueOfUint64(0), nil
		}
		if n, err := strconv.ParseUint(str, base10, b64); err == nil {
			return protoreflect.ValueOfUint64(n), nil
		}
	case protoreflect.DoubleKind:
		if str == "" {
			return protoreflect.ValueOfFloat64(0), nil
		}
		if n, err := strconv.ParseFloat(str, b64); err == nil {
			return protoreflect.ValueOfFloat64(n), nil
		}
	case protoreflect.FloatKind:
		if str == "" {
			return protoreflect.ValueOfFloat32(0), nil
		}
		if n, err := strconv.ParseFloat(str, b64); err == nil {
			return protoreflect.ValueOfFloat32(float32(n)), nil
		}
	case protoreflect.EnumKind:
		if str == "" {
			return protoreflect.ValueOfEnum(0), nil
		}
		if n, err := strconv.ParseInt(str, base10, b32); err == nil {
			return protoreflect.ValueOfEnum(protoreflect.EnumNumber(n)), nil
		}
	}

	return protoreflect.Value{}, fmt.Errorf(
		"invalid value for: fd=%s value=%s kind=%d",
		string(fd.Name()), str, int8(kind),
	)
}

func marshalScalar(fd protoreflect.FieldDescriptor, v protoreflect.Value) (retstr string) {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		if v.Bool() {
			retstr = "1"
		} else {
			retstr = "0"
		}
	default:
		retstr = v.String()
	}
	return
}

// IsFdMarshalToBlob reports whether the field is marshalled in raw wire format.
func IsFdMarshalToBlob(fd protoreflect.FieldDescriptor) bool {
	if fd.Cardinality() == protoreflect.Repeated {
		return true
	}
	_, exist := _BlobKind[fd.Kind()]
	return exist
}

// BuildKeyFieldsMap builds a lookup table containing all primary key fields.
func BuildKeyFieldsMap(desc protoreflect.MessageDescriptor) map[string]struct{} {
	ret := map[string]struct{}{}
	keys := FindPrimaryKey(desc)
	for _, key := range keys {
		ret[key] = struct{}{}
	}
	return ret
}

// BuildIncreaseableFieldsMap returns the set of integer fields eligible for increments.
func BuildIncreaseableFieldsMap(desc protoreflect.MessageDescriptor) map[string]struct{} {
	ret := map[string]struct{}{}
	keys := BuildKeyFieldsMap(desc)

	fields := desc.Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if fd.Cardinality() == protoreflect.Repeated {
			continue
		}
		if _, exist := keys[string(fd.Name())]; exist {
			continue
		}
		if _, exist := _IncreaseAbleKind[fd.Kind()]; !exist {
			continue
		}
		ret[string(fd.Name())] = struct{}{}
	}
	return ret
}
