// Copyright 2017 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package protostruct supports operations on the protocol buffer Struct message.
package protostruct

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/golang/protobuf/proto"
	pb "github.com/golang/protobuf/ptypes/struct"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestEncodeDecodeStruct(t *testing.T) {
	unittest.InitGinkgo(t, "Protobuf Struct to map")
}

var (
	alwaysEqual = cmp.Comparer(func(_, _ interface{}) bool { return true })

	defaultCmpOptions = []cmp.Option{
		// Use proto.Equal for protobufs
		cmp.Comparer(proto.Equal),
		// Use big.Rat.Cmp for big.Rats
		cmp.Comparer(func(x, y *big.Rat) bool {
			if x == nil || y == nil {
				return x == y
			}
			return x.Cmp(y) == 0
		}),
		// NaNs compare equal
		cmp.FilterValues(func(x, y float64) bool {
			return math.IsNaN(x) && math.IsNaN(y)
		}, alwaysEqual),
		cmp.FilterValues(func(x, y float32) bool {
			return math.IsNaN(float64(x)) && math.IsNaN(float64(y))
		}, alwaysEqual),
	}
)
var _ = Describe("Protobuf Struct to map", func() {
	It("should successfully Encode/Decode", func() {
		g := NewWithT(GinkgoT())

		var pbstsruct *pb.Struct = nil
		var imap = (map[string]interface{}(nil))

		decoded := DecodeToMap(pbstsruct)
		g.Expect(decoded).To(Equal(imap), fmt.Sprintf("DecodeToMap(nil) = %v, want nil", decoded))

		nullv := &pb.Value{Kind: &pb.Value_NullValue{}}
		stringv := &pb.Value{Kind: &pb.Value_StringValue{StringValue: "x"}}
		boolv := &pb.Value{Kind: &pb.Value_BoolValue{BoolValue: true}}
		numberv := &pb.Value{Kind: &pb.Value_NumberValue{NumberValue: 2.7}}
		_int32 := &pb.Value{Kind: &pb.Value_NumberValue{NumberValue: 32}}
		_int64 := &pb.Value{Kind: &pb.Value_NumberValue{NumberValue: 64}}
		_int := &pb.Value{Kind: &pb.Value_NumberValue{NumberValue: 32}}
		pbstsruct = &pb.Struct{Fields: map[string]*pb.Value{
			"n":     nullv,
			"s":     stringv,
			"b":     boolv,
			"f":     numberv,
			"int32": _int32,
			"int64": _int64,
			"int":   _int,
			"l": {Kind: &pb.Value_ListValue{ListValue: &pb.ListValue{
				Values: []*pb.Value{nullv, stringv, boolv, numberv},
			}}},
			"S": {Kind: &pb.Value_StructValue{StructValue: &pb.Struct{Fields: map[string]*pb.Value{
				"n1": nullv,
				"b1": boolv,
			}}}},
		}}

		imap = map[string]interface{}{
			"n":     nil,
			"s":     "x",
			"b":     true,
			"f":     2.7,
			"int32": float64(32),
			"int64": float64(64),
			"int":   float64(32),
			"l":     []interface{}{nil, "x", true, 2.7},
			"S":     map[string]interface{}{"n1": nil, "b1": true},
		}

		decoded = DecodeToMap(pbstsruct)
		encoded := EncodeToStruct(imap)

		g.Expect(cmp.Diff(encoded, pbstsruct, defaultCmpOptions...)).To(BeEmpty(), "2 pb.structs should be equal")
		g.Expect(cmp.Diff(imap, decoded, defaultCmpOptions...)).To(BeEmpty(), "2 maps should be equal")
	})

})
