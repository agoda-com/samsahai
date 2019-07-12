package protostruct

import (
	"fmt"
	"reflect"

	pb "github.com/golang/protobuf/ptypes/struct"
)

// EncodeToStruct converts a map[string]interface{} to a ptypes.Struct
func EncodeToStruct(v map[string]interface{}) *pb.Struct {
	size := len(v)
	if size == 0 {
		return nil
	}
	fields := make(map[string]*pb.Value, size)
	for k, v := range v {
		fields[k] = ToValue(v)
	}
	return &pb.Struct{
		Fields: fields,
	}
}

// ToValue converts an interface{} to a ptypes.Value
func ToValue(v interface{}) *pb.Value {
	switch v := v.(type) {
	case nil:
		return &pb.Value{
			Kind: &pb.Value_NullValue{
				NullValue: pb.NullValue_NULL_VALUE,
			},
		}
	case bool:
		return &pb.Value{
			Kind: &pb.Value_BoolValue{
				BoolValue: v,
			},
		}
	case int:
		return &pb.Value{
			Kind: &pb.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case int8:
		return &pb.Value{
			Kind: &pb.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case int32:
		return &pb.Value{
			Kind: &pb.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case int64:
		return &pb.Value{
			Kind: &pb.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case uint:
		return &pb.Value{
			Kind: &pb.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case uint8:
		return &pb.Value{
			Kind: &pb.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case uint32:
		return &pb.Value{
			Kind: &pb.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case uint64:
		return &pb.Value{
			Kind: &pb.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case float32:
		return &pb.Value{
			Kind: &pb.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case float64:
		return &pb.Value{
			Kind: &pb.Value_NumberValue{
				NumberValue: v,
			},
		}
	case string:
		return &pb.Value{
			Kind: &pb.Value_StringValue{
				StringValue: v,
			},
		}
	case error:
		return &pb.Value{
			Kind: &pb.Value_StringValue{
				StringValue: v.Error(),
			},
		}
	default:
		// Fallback to reflection for other types
		return toValue(reflect.ValueOf(v))
	}
}

func toValue(v reflect.Value) *pb.Value {
	switch v.Kind() {
	case reflect.Bool:
		return &pb.Value{
			Kind: &pb.Value_BoolValue{
				BoolValue: v.Bool(),
			},
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &pb.Value{
			Kind: &pb.Value_NumberValue{
				NumberValue: float64(v.Int()),
			},
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return &pb.Value{
			Kind: &pb.Value_NumberValue{
				NumberValue: float64(v.Uint()),
			},
		}
	case reflect.Float32, reflect.Float64:
		return &pb.Value{
			Kind: &pb.Value_NumberValue{
				NumberValue: v.Float(),
			},
		}
	case reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		return toValue(reflect.Indirect(v))
	case reflect.Array, reflect.Slice:
		size := v.Len()
		if size == 0 {
			return nil
		}
		values := make([]*pb.Value, size)
		for i := 0; i < size; i++ {
			values[i] = toValue(v.Index(i))
		}
		return &pb.Value{
			Kind: &pb.Value_ListValue{
				ListValue: &pb.ListValue{
					Values: values,
				},
			},
		}
	case reflect.Struct:
		t := v.Type()
		size := v.NumField()
		if size == 0 {
			return nil
		}
		fields := make(map[string]*pb.Value, size)
		for i := 0; i < size; i++ {
			name := t.Field(i).Name
			// Better way?
			if len(name) > 0 && 'A' <= name[0] && name[0] <= 'Z' {
				fields[name] = toValue(v.Field(i))
			}
		}
		if len(fields) == 0 {
			return nil
		}
		return &pb.Value{
			Kind: &pb.Value_StructValue{
				StructValue: &pb.Struct{
					Fields: fields,
				},
			},
		}
	case reflect.Map:
		keys := v.MapKeys()
		if len(keys) == 0 {
			return nil
		}
		fields := make(map[string]*pb.Value, len(keys))
		for _, k := range keys {
			if k.Kind() == reflect.String {
				fields[k.String()] = toValue(v.MapIndex(k))
			}
		}
		if len(fields) == 0 {
			return nil
		}
		return &pb.Value{
			Kind: &pb.Value_StructValue{
				StructValue: &pb.Struct{
					Fields: fields,
				},
			},
		}
	case reflect.Interface:
		switch v2 := v.Interface().(type) {
		case bool, int, int8, int32, int64, uint, uint8, uint32, uint64, float32, float64, string, error, nil:
			return ToValue(v2)
		case map[string]interface{}:
			//keys := v.MapKeys()
			if len(v2) == 0 {
				return nil
			}
			fields := make(map[string]*pb.Value, len(v2))
			for k3, v3 := range v2 {
				fields[k3] = ToValue(v3)
			}
			if len(fields) == 0 {
				return nil
			}
			return &pb.Value{
				Kind: &pb.Value_StructValue{
					StructValue: &pb.Struct{
						Fields: fields,
					},
				},
			}
		case []interface{}:
			size := len(v2)
			if size == 0 {
				return nil
			}
			values := make([]*pb.Value, size)
			for i := 0; i < size; i++ {
				values[i] = ToValue(v2[i])
			}
			return &pb.Value{
				Kind: &pb.Value_ListValue{
					ListValue: &pb.ListValue{
						Values: values,
					},
				},
			}
		default:
			return &pb.Value{
				Kind: &pb.Value_StringValue{
					StringValue: fmt.Sprint(v),
				},
			}
		}
	default:
		// Last resort
		return &pb.Value{
			Kind: &pb.Value_StringValue{
				StringValue: fmt.Sprint(v),
			},
		}
	}
}
