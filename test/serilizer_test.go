package test

import (
	"fmt"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/utils/logger"
	"google.golang.org/protobuf/proto"
	"testing"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/utils"
)

func TestNull(t *testing.T) {
	listData := &ultipa.SetData{
		IsNull: true,
	}
	bs, err := proto.Marshal(listData)
	if err != nil {
		logger.PrintError(fmt.Sprintf("failed to get bytes of null list, %v", err))
	}
	t.Log(bs)
}

func TestStringAsInterface(t *testing.T) {
	datetime, err := utils.StringAsInterface("1970-01-01", ultipa.PropertyType_DATETIME, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(datetime)
}

func TestConvertInterfaceToBytesSafe(t *testing.T) {
	ultipaTime, err := utils.NewTimestampFromString("2010-12-12 00:00:00.000", nil)
	if err != nil {
		t.Fatal(err)
	}
	datetime, err := utils.ConvertInterfaceToBytesSafe(ultipaTime, ultipa.PropertyType_DATETIME, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(datetime)

	datetime, err = utils.ConvertInterfaceToBytesSafe("2010-12-12T00:00:00.000Z", ultipa.PropertyType_DATETIME, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(datetime)

	ultipaTime, err = utils.NewTimestampFromString("1994-12-12 00:00:00", nil)
	if err != nil {
		t.Fatal(err)
	}
	datetime, err = utils.ConvertInterfaceToBytesSafe(ultipaTime, ultipa.PropertyType_DATETIME, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(datetime)

	datetime, err = utils.ConvertInterfaceToBytesSafe("1994-12-12 00:00:00", ultipa.PropertyType_DATETIME, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(datetime)
}

func TestSerializePoint(t *testing.T) {
	pointBytes, err := utils.ConvertInterfaceToBytesSafe("Point(1.0 2.0)", ultipa.PropertyType_POINT, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(pointBytes)
	t.Log(string(pointBytes))

}

func TestSerializeBlob(t *testing.T) {
	value := []byte{97, 98, 99}

	bytes, err := utils.ConvertInterfaceToBytesSafe(value, ultipa.PropertyType_BLOB, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(bytes)
	t.Log(string(bytes))

}
