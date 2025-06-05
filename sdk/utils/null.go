package utils

import (
	"fmt"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/utils/logger"
	"google.golang.org/protobuf/proto"
)

var nullBytes map[ultipa.PropertyType][]byte

func init() {
	nullBytes = map[ultipa.PropertyType][]byte{
		ultipa.PropertyType_INT32: {0x7f, 0xff, 0xff, 0xff},
		ultipa.PropertyType_DATETIME: {
			0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff,
		},
		ultipa.PropertyType_UINT32: {
			0xff, 0xff, 0xff, 0xff},
		ultipa.PropertyType_INT64: {
			0x7f, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff,
		},
		ultipa.PropertyType_TIMESTAMP: {
			0xff, 0xff, 0xff, 0xff,
		},
		ultipa.PropertyType_UINT64: {
			0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff,
		},
		ultipa.PropertyType_FLOAT: {
			0xff, 0xff, 0xff, 0xff,
		},
		ultipa.PropertyType_DOUBLE: {
			0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff,
		},
		ultipa.PropertyType_STRING: {0},
		ultipa.PropertyType_TEXT:   {0},
		ultipa.PropertyType_POINT: {
			0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff,
		},
		ultipa.PropertyType_LIST: getListNullValue(),
		ultipa.PropertyType_MAP:  getMapNullValue(),
		ultipa.PropertyType_BOOL: {0x2},
	}
}

func GetNullBytes(propertyType ultipa.PropertyType) []byte {
	return nullBytes[propertyType]
}

func GetNullValue(propertyType ultipa.PropertyType) []byte {
	return nullBytes[propertyType]
}

func getListNullValue() []byte {
	listData := &ultipa.ListData{
		IsNull: true,
	}
	bs, err := proto.Marshal(listData)
	if err != nil {
		logger.PrintError(fmt.Sprintf("failed to get bytes of null list, %v", err))
	}
	return bs
}

func getMapNullValue() []byte {
	mapData := &ultipa.MapData{IsNull: true}
	bs, err := proto.Marshal(mapData)
	if err != nil {
		logger.PrintError(fmt.Sprintf("failed to get bytes of null map, %v", err))
	}
	return bs
}

func IsNull(propertyType ultipa.PropertyType, bs []byte) bool {
	if ultipa.PropertyType_NULL_ == propertyType {
		return true
	}
	nullBs := GetNullBytes(propertyType)
	return BytesEqual(bs, nullBs)
}
