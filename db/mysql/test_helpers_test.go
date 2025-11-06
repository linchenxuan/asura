package mysql

import (
	"sync"
	"testing"

	"github.com/lcx/asura/db"
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

func strPtr(v string) *string {
	return &v
}

func int32Ptr(v int32) *int32 {
	return &v
}

func testRecordDescriptor(t testing.TB) protoreflect.MessageDescriptor {
	t.Helper()

	testFileOnce.Do(func() {
		msgOptions := &descriptorpb.MessageOptions{}
		proto.SetExtension(msgOptions, db.E_PrimaryKey, "id")
		proto.SetExtension(msgOptions, db.E_Index, []string{"name"})

		nested := &descriptorpb.DescriptorProto{
			Name: strPtr("Nested"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name:   strPtr("value"),
					Number: int32Ptr(1),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
				},
			},
		}

		message := &descriptorpb.DescriptorProto{
			Name:    strPtr("TestRecord"),
			Options: msgOptions,
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name:   strPtr("id"),
					Number: int32Ptr(1),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
				},
				{
					Name:   strPtr("name"),
					Number: int32Ptr(2),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				},
				{
					Name:   strPtr("counter"),
					Number: int32Ptr(3),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
				},
				{
					Name:   strPtr("hits"),
					Number: int32Ptr(4),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_UINT64.Enum(),
				},
				{
					Name:   strPtr("active"),
					Number: int32Ptr(5),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum(),
				},
				{
					Name:   strPtr("score"),
					Number: int32Ptr(6),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum(),
				},
				{
					Name:   strPtr("payload"),
					Number: int32Ptr(7),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum(),
				},
				{
					Name:   strPtr("labels"),
					Number: int32Ptr(8),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				},
				{
					Name:     strPtr("details"),
					Number:   int32Ptr(9),
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
					TypeName: strPtr(".dbtest.Nested"),
				},
			},
		}

		fileProto := &descriptorpb.FileDescriptorProto{
			Syntax:      strPtr("proto3"),
			Name:        strPtr("db_test_record.proto"),
			Package:     strPtr("dbtest"),
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

func resetMetaCache() {
	_locker.Lock()
	_metaMap = map[protoreflect.FullName]*DBProtoMeta{}
	_locker.Unlock()
}
