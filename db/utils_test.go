package db

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

var (
	testFileOnce sync.Once
	testFileDesc protoreflect.FileDescriptor
	testFileErr  error
)

func strptr(v string) *string {
	return &v
}

func int32ptr(v int32) *int32 {
	return &v
}

func testRecordDescriptor(t testing.TB) protoreflect.MessageDescriptor {
	t.Helper()

	testFileOnce.Do(func() {
		msgOptions := &descriptorpb.MessageOptions{}
		proto.SetExtension(msgOptions, E_PrimaryKey, "id")
		proto.SetExtension(msgOptions, E_Index, []string{"name"})

		nested := &descriptorpb.DescriptorProto{
			Name: strptr("Nested"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name:   strptr("value"),
					Number: int32ptr(1),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
				},
			},
		}

		message := &descriptorpb.DescriptorProto{
			Name:    strptr("TestRecord"),
			Options: msgOptions,
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name:   strptr("id"),
					Number: int32ptr(1),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
				},
				{
					Name:   strptr("name"),
					Number: int32ptr(2),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				},
				{
					Name:   strptr("counter"),
					Number: int32ptr(3),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
				},
				{
					Name:   strptr("hits"),
					Number: int32ptr(4),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_UINT64.Enum(),
				},
				{
					Name:   strptr("active"),
					Number: int32ptr(5),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum(),
				},
				{
					Name:   strptr("score"),
					Number: int32ptr(6),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum(),
				},
				{
					Name:   strptr("payload"),
					Number: int32ptr(7),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum(),
				},
				{
					Name:   strptr("labels"),
					Number: int32ptr(8),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				},
				{
					Name:     strptr("details"),
					Number:   int32ptr(9),
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
					TypeName: strptr(".dbtest.Nested"),
				},
			},
		}

		fileProto := &descriptorpb.FileDescriptorProto{
			Syntax:      strptr("proto3"),
			Name:        strptr("db_test_record.proto"),
			Package:     strptr("dbtest"),
			MessageType: []*descriptorpb.DescriptorProto{message, nested},
		}

		fd, err := protodesc.NewFile(fileProto, nil)
		if err != nil {
			testFileErr = err
			return
		}
		testFileDesc = fd
	})

	require.NoError(t, testFileErr)
	return testFileDesc.Messages().ByName("TestRecord")
}

func newTestRecord(t testing.TB) *dynamicpb.Message {
	t.Helper()

	desc := testRecordDescriptor(t)
	msg := dynamicpb.NewMessage(desc)
	fields := desc.Fields()

	msg.Set(fields.ByName("id"), protoreflect.ValueOfInt64(42))
	msg.Set(fields.ByName("name"), protoreflect.ValueOfString("primary"))
	msg.Set(fields.ByName("counter"), protoreflect.ValueOfInt64(7))
	msg.Set(fields.ByName("hits"), protoreflect.ValueOfUint64(11))
	msg.Set(fields.ByName("active"), protoreflect.ValueOfBool(true))
	msg.Set(fields.ByName("score"), protoreflect.ValueOfFloat64(123.5))
	msg.Set(fields.ByName("payload"), protoreflect.ValueOfBytes([]byte("blob-data")))

	labelsField := fields.ByName("labels")
	labels := msg.Mutable(labelsField).List()
	labels.Append(protoreflect.ValueOfString("alpha"))
	labels.Append(protoreflect.ValueOfString("beta"))

	detailsField := fields.ByName("details")
	details := msg.Mutable(detailsField).Message()
	detailsDesc := detailsField.Message()
	details.Set(detailsDesc.Fields().ByName("value"), protoreflect.ValueOfInt32(99))

	return msg
}

func mapToStringMap(src map[string]any) map[string]string {
	out := make(map[string]string, len(src))
	for k, v := range src {
		switch val := v.(type) {
		case string:
			out[k] = val
		case []byte:
			out[k] = string(val)
		default:
			panic("unexpected map value type")
		}
	}
	return out
}

func TestFieldsToStrings(t *testing.T) {
	t.Parallel()

	require.Nil(t, FieldsToStrings(nil))

	fields := []Field{"one", "two"}
	require.Equal(t, []string{"one", "two"}, FieldsToStrings(fields))
}

func TestIndexConversions(t *testing.T) {
	t.Parallel()

	indexes := []Index{"p_idx", "s_idx"}
	require.Equal(t, []string{"p_idx", "s_idx"}, IndexsToStrings(indexes))

	strs := []string{"foo", "bar"}
	require.Equal(t, []Index{"foo", "bar"}, StringsToIndexs(strs))
}

func TestDirtyFilter(t *testing.T) {
	t.Parallel()

	desc := testRecordDescriptor(t)
	filter := DirtyFilter(desc, []string{"name", "details.value", "missing"})
	require.Equal(t, map[string]struct{}{
		"name":    {},
		"details": {},
	}, filter)
}

func TestMarshalToMap_AllFields(t *testing.T) {
	t.Parallel()

	msg := newTestRecord(t)
	wireMap, err := MarshalToMap(msg, nil)
	require.NoError(t, err)

	require.Equal(t, "42", wireMap["id"])
	require.Equal(t, "primary", wireMap["name"])
	require.Equal(t, "7", wireMap["counter"])
	require.Equal(t, "11", wireMap["hits"])
	require.Equal(t, "1", wireMap["active"])
	require.Equal(t, "123.5", wireMap["score"])

	require.IsType(t, []byte{}, wireMap["payload"])
	require.IsType(t, []byte{}, wireMap["labels"])
	require.IsType(t, []byte{}, wireMap["details"])
}

func TestMarshalToMap_FieldFilter(t *testing.T) {
	t.Parallel()

	msg := newTestRecord(t)
	wireMap, err := MarshalToMap(msg, []string{"name", "details.value"})
	require.NoError(t, err)

	require.Equal(t, 2, len(wireMap))
	require.Equal(t, "primary", wireMap["name"])
	require.IsType(t, []byte{}, wireMap["details"])
	_, ok := wireMap["counter"]
	assert.False(t, ok)
}

func TestUnmarshalFromMap_RoundTrip(t *testing.T) {
	t.Parallel()

	msg := newTestRecord(t)
	wireMap, err := MarshalToMap(msg, nil)
	require.NoError(t, err)

	stringMap := mapToStringMap(wireMap)
	desc := testRecordDescriptor(t)
	restored := dynamicpb.NewMessage(desc)
	require.NoError(t, UnmarshalFromMap(restored, stringMap))

	require.True(t, proto.Equal(msg, restored))
}

func TestFindPrimaryKey(t *testing.T) {
	t.Parallel()

	desc := testRecordDescriptor(t)
	require.Equal(t, []string{"id"}, FindPrimaryKey(desc))
}

func TestFindFds(t *testing.T) {
	t.Parallel()

	desc := testRecordDescriptor(t)
	fds := FindFds(desc, []string{"id", "name", "counter"})
	require.Len(t, fds, 3)
	require.Equal(t, "id", string(fds[0].Name()))
	require.Equal(t, "name", string(fds[1].Name()))
	require.Equal(t, "counter", string(fds[2].Name()))
}

func TestBuildKeyFieldsMap(t *testing.T) {
	t.Parallel()

	desc := testRecordDescriptor(t)
	require.Equal(t, map[string]struct{}{"id": {}}, BuildKeyFieldsMap(desc))
}

func TestBuildIncreaseableFieldsMap(t *testing.T) {
	t.Parallel()

	desc := testRecordDescriptor(t)
	require.Equal(t, map[string]struct{}{
		"counter": {},
		"hits":    {},
	}, BuildIncreaseableFieldsMap(desc))
}

func TestIsFdMarshalToBlob(t *testing.T) {
	t.Parallel()

	desc := testRecordDescriptor(t)
	fields := desc.Fields()

	require.False(t, IsFdMarshalToBlob(fields.ByName("name")))
	require.True(t, IsFdMarshalToBlob(fields.ByName("payload")))
	require.True(t, IsFdMarshalToBlob(fields.ByName("labels")))
	require.True(t, IsFdMarshalToBlob(fields.ByName("details")))
}
