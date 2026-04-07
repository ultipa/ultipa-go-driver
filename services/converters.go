package services

import (
	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"github.com/ultipa/ultipa-go-driver/v6/types"
)

// TypedValue mirrors the main package TypedValue for property conversion.
type TypedValue struct {
	Type   PropertyType
	Data   []byte
	IsNull bool
}

// PropertyType mirrors the main package PropertyType.
type PropertyType int32

// ConvertProperties converts Go map to proto TypedValue map.
// Takes a newTypedValue function to avoid circular dependencies.
func ConvertProperties(props map[string]interface{}, newTypedValue func(v interface{}) (*TypedValue, error)) (map[string]*pb.TypedValue, error) {
	if props == nil {
		return nil, nil
	}

	result := make(map[string]*pb.TypedValue)
	for k, v := range props {
		tv, err := newTypedValue(v)
		if err != nil {
			return nil, err
		}
		result[k] = &pb.TypedValue{
			Type:   pb.PropertyType(tv.Type),
			Data:   tv.Data,
			IsNull: tv.IsNull,
		}
	}
	return result, nil
}

// ConvertPropertiesFromProto converts proto TypedValue map to Go map.
// Takes a toGo function to avoid circular dependencies.
func ConvertPropertiesFromProto(props map[string]*pb.TypedValue, toGo func(tv *TypedValue) (interface{}, error)) (map[string]interface{}, error) {
	if props == nil {
		return nil, nil
	}

	result := make(map[string]interface{})
	for k, v := range props {
		tv := &TypedValue{
			Type:   PropertyType(v.Type),
			Data:   v.Data,
			IsNull: v.IsNull,
		}
		val, err := toGo(tv)
		if err != nil {
			return nil, err
		}
		result[k] = val
	}
	return result, nil
}

// GraphTypeToProto converts GraphType to proto enum.
func GraphTypeToProto(gt types.GraphType) pb.GraphType {
	return pb.GraphType(gt)
}

// CacheTypeToProto converts CacheType to proto enum.
func CacheTypeToProto(ct int32) pb.CacheType {
	return pb.CacheType(ct)
}

// HealthStatusFromProto converts proto HealthStatus to int32.
func HealthStatusFromProto(status pb.HealthStatus) int32 {
	return int32(status)
}
