// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.24.0-devel
// 	protoc        v3.9.1
// source: pkg/samsahai/rpc

package rpc

import (
	reflect "reflect"

	proto "github.com/golang/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

var File_pkg_samsahai_rpc protoreflect.FileDescriptor

var file_pkg_samsahai_rpc_rawDesc = []byte{
	0x0a, 0x10, 0x70, 0x6b, 0x67, 0x2f, 0x73, 0x61, 0x6d, 0x73, 0x61, 0x68, 0x61, 0x69, 0x2f, 0x72,
	0x70, 0x63,
}

var file_pkg_samsahai_rpc_goTypes = []interface{}{}
var file_pkg_samsahai_rpc_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_pkg_samsahai_rpc_init() }
func file_pkg_samsahai_rpc_init() {
	if File_pkg_samsahai_rpc != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pkg_samsahai_rpc_rawDesc,
			NumEnums:      0,
			NumMessages:   0,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pkg_samsahai_rpc_goTypes,
		DependencyIndexes: file_pkg_samsahai_rpc_depIdxs,
	}.Build()
	File_pkg_samsahai_rpc = out.File
	file_pkg_samsahai_rpc_rawDesc = nil
	file_pkg_samsahai_rpc_goTypes = nil
	file_pkg_samsahai_rpc_depIdxs = nil
}
