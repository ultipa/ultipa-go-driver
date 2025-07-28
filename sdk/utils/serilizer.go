package utils

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/types"
	"google.golang.org/protobuf/proto"
)

var default_nil_string = string([]byte{0x00})

const SERVER_BOOL_TRUE_VALUE = "1"
const SERVER_BOOL_FALSE_VALUE = "0"

// Convert Bytes to GoLang DBType and return to an interface
func ConvertBytesToInterface(bs []byte, t ultipa.PropertyType, subTypes []ultipa.PropertyType) (interface{}, error) {
	if IsNull(t, bs) {
		return nil, nil
	}
	switch t {
	case ultipa.PropertyType_STRING:
		return AsString(bs), nil
	case ultipa.PropertyType_INT32:
		return AsInt32(bs), nil
	case ultipa.PropertyType_INT64:
		return AsInt64(bs), nil
	case ultipa.PropertyType_UINT32:
		return AsUint32(bs), nil
	case ultipa.PropertyType_UINT64:
		return AsUint64(bs), nil
	case ultipa.PropertyType_FLOAT:
		return AsFloat32(bs), nil
	case ultipa.PropertyType_DOUBLE:
		return AsFloat64(bs), nil
	case ultipa.PropertyType_DATETIME:
		if len(bs) == 0 {
			return nil, nil
		}
		value := AsUint64(bs)
		if value == 0 {
			return uint64(0), nil
		}
		return NewDateTime(value), nil
	case ultipa.PropertyType_TIMESTAMP:
		if len(bs) == 0 {
			return nil, nil
		}
		value := AsUint32(bs)
		return NewTimeStamp(int64(value)), nil
	case ultipa.PropertyType_TEXT:
		return AsString(bs), nil
	case ultipa.PropertyType_BLOB:
		return bs, nil
	case ultipa.PropertyType_POINT:
		str, err := AsPointString(bs)
		if str == "" || err != nil {
			return nil, err
		}
		return types.PointFromStr(str)
		//TODO
	case ultipa.PropertyType_DECIMAL:
		if len(bs) == 0 {
			return nil, nil
		}
		return AsString(bs), nil
	case ultipa.PropertyType_LIST:
		return deserializeList(bs, subTypes)
	//	//TODO
	case ultipa.PropertyType_SET:
		return deserializeSet(bs, subTypes)
	//	//TODO
	//case ultipa.PropertyType_MAP:
	//	//TODO
	//case ultipa.PropertyType_UNSET:
	//	return nil
	case ultipa.PropertyType_BOOL:
		serverValue := AsString(bs)
		if serverValue == "" {
			return nil, nil
		}
		return strconv.ParseBool(serverValue)

	case ultipa.PropertyType_DATE:
		return decodeUtpDate(bs)
	case ultipa.PropertyType_LOCAL_TIME:
		return decodeUtpTime(bs, false)
	case ultipa.PropertyType_ZONED_TIME:
		return decodeUtpTime(bs, true)
	case ultipa.PropertyType_LOCAL_DATETIME:
		return decodeUtpDatetime(bs, false)
	case ultipa.PropertyType_ZONED_DATETIME:
		return decodeUtpDatetime(bs, true)
	case ultipa.PropertyType_YEAR_TO_MONTH:
		return decodeUtpDurationYTM(bs)
	case ultipa.PropertyType_DAY_TO_SECOND:
		return decodeUtpDurationDTS(bs)
	default:
		return nil, nil
	}
}

// deserializeList deserialize bs to list
func deserializeList(bs []byte, subTypes []ultipa.PropertyType) (interface{}, error) {
	listData := &ultipa.ListData{}
	if err := proto.Unmarshal(bs, listData); err != nil {
		return nil, err
	}
	var list []interface{}
	if listData.IsNull {
		return list, nil
	}
	if listData.Values != nil {
		for _, value := range listData.Values {
			element, err := ConvertBytesToInterface(value, subTypes[0], nil)
			if err != nil {
				return nil, err
			}
			list = append(list, element)
		}
	}
	return list, nil
}

// deserializeSet deserialize bs to set
func deserializeSet(bs []byte, subTypes []ultipa.PropertyType) (interface{}, error) {
	setData := &ultipa.SetData{}
	if err := proto.Unmarshal(bs, setData); err != nil {
		return nil, err
	}
	var list []interface{}
	if setData.IsNull {
		return list, nil
	}
	if setData.Values != nil {
		for _, value := range setData.Values {
			element, err := ConvertBytesToInterface(value, subTypes[0], nil)
			if err != nil {
				return nil, err
			}
			list = append(list, element)
		}
	}
	return list, nil
}

// ConvertInterfaceToBytesSafe convert value to []byte, if value is nil, will set default value according to PropertyType t
func ConvertInterfaceToBytesSafe(value interface{}, t ultipa.PropertyType, subTypes []ultipa.PropertyType, req *configuration.RequestConfig) ([]byte, error) {
	toConvertValue := value
	if toConvertValue == nil {
		switch t {
		//case ultipa.PropertyType_SET:
		//    return nil, errors.New(fmt.Sprintf("unsuppoted ultipa.PropertyType [%s]", t))
		case ultipa.PropertyType_MAP:
			return nil, errors.New(fmt.Sprintf("unsuppoted ultipa.PropertyType [%s]", t))
		//case ultipa.PropertyType_DECIMAL:
		//	return nil, errors.New(fmt.Sprintf("unsuppoted ultipa.PropertyType [%s]", t))
		default:
			return GetNullBytes(t), nil
		}
	}
	switch t {
	case ultipa.PropertyType_LIST:
		return SerializeListData(value, subTypes, req)
	case ultipa.PropertyType_POINT:
		switch v := value.(type) {
		case types.Point:
			return []byte(v.String()), nil

		case *types.Point:
			return []byte(v.String()), nil

		case string:
			point, err := types.PointFromStr(v)
			if err != nil {
				return nil, err
			}
			return []byte(point.String()), nil
		default:
			return ConvertInterfaceToBytes(value)
		}

	case ultipa.PropertyType_DECIMAL:
		return serializeDecimal(value)
	case ultipa.PropertyType_SET:
		return SerializeSetData(value, subTypes, req)
	case ultipa.PropertyType_MAP:
		return nil, errors.New(fmt.Sprintf("unsuppoted ultipa.PropertyType [%s]", t))
	case ultipa.PropertyType_DATETIME:
		switch v := value.(type) {
		case UltipaTime:
			return ConvertInterfaceToBytes(v.Datetime)
		case *UltipaTime:
			return ConvertInterfaceToBytes(v.Datetime)
		case string:
			datetime, err := StringAsInterface(v, ultipa.PropertyType_DATETIME, GetLocationFromConfig(req))

			if err != nil {
				return nil, err
			}
			return ConvertInterfaceToBytes(datetime)
		default:
			return ConvertInterfaceToBytes(value)
		}
	case ultipa.PropertyType_TIMESTAMP:
		switch v := value.(type) {
		case UltipaTime:
			return ConvertInterfaceToBytes(v.GetTimeStamp())
		case *UltipaTime:
			return ConvertInterfaceToBytes(v.GetTimeStamp())
		case string:
			timestamp, err := StringAsInterface(v, ultipa.PropertyType_TIMESTAMP, GetLocationFromConfig(req))

			if err != nil {
				return nil, err
			}
			return ConvertInterfaceToBytes(timestamp)
		default:
			return ConvertInterfaceToBytes(value)
		}
	case ultipa.PropertyType_BOOL:
		switch v := value.(type) {
		case bool:
			return doSerializeBool(v)
		case string:
			boolValue, err := strconv.ParseBool(v)
			if err != nil {
				return nil, err
			}
			return doSerializeBool(boolValue)
		default:
			return nil, errors.New(fmt.Sprintf("unable to parse the value to bool:%v", value))
		}
	default:
		return ConvertInterfaceToBytes(toConvertValue)
	}
}

// serializeDecimal serialize value to bytes for decimal property type.
func serializeDecimal(value interface{}) ([]byte, error) {

	switch t := value.(type) {
	case int32:
		v := value.(int32)
		str := strconv.Itoa(int(v))
		return []byte(str), nil
	case int:
		str := strconv.Itoa(value.(int))
		return []byte(str), nil
	case string:
		return []byte(value.(string)), nil
	case int64:
		v := value.(int64)
		str := strconv.Itoa(int(v))
		return []byte(str), nil
	case uint32:
		v := value.(uint32)
		str := strconv.FormatUint(uint64(v), 10)
		return []byte(str), nil
	case uint64:
		str := strconv.FormatUint(value.(uint64), 10)
		return []byte(str), nil
	case float32:
		v := value.(float32)
		str := strconv.FormatFloat(float64(v), 'f', -1, 32)
		return []byte(str), nil
	case float64:
		str := strconv.FormatFloat(value.(float64), 'f', -1, 64)
		return []byte(str), nil
	default:
		return nil, errors.New(fmt.Sprint("not supported ultipa type : ", t))
	}
}

func doSerializeBool(v bool) ([]byte, error) {
	if v {
		return []byte(SERVER_BOOL_TRUE_VALUE), nil
	}
	return []byte(SERVER_BOOL_FALSE_VALUE), nil
}

func SerializeListData(list interface{}, subTypes []ultipa.PropertyType, req *configuration.RequestConfig) ([]byte, error) {
	if subTypes == nil {
		return nil, errors.New("subTypes is nil, unable to serialize list")
	}
	if len(subTypes) == 0 {
		return nil, errors.New("subTypes is not specified, unable to serialize list")
	}
	listData := &ultipa.ListData{}
	if list == nil {
		listData.IsNull = true
		return proto.Marshal(listData)
	}
	vi := reflect.ValueOf(list)
	for index := 0; index < vi.Len(); index++ {
		//TODO if vi.Index(index) is ListValue?
		bs, err := ConvertInterfaceToBytesSafe(vi.Index(index).Interface(), subTypes[0], nil, req)
		if err != nil {
			return nil, err
		}
		listData.Values = append(listData.Values, bs)
	}
	return proto.Marshal(listData)
}

func SerializeSetData(set interface{}, subTypes []ultipa.PropertyType, req *configuration.RequestConfig) ([]byte, error) {
	if subTypes == nil && len(subTypes) == 0 {
		return nil, errors.New("subTypes is nil, unable to serialize SetData")
	}
	if len(subTypes) == 0 {
		return nil, errors.New("subTypes is not specified, unable to serialize SetData")
	}
	if set == nil {
		setData := &ultipa.SetData{}
		setData.IsNull = true
		return proto.Marshal(setData)
	}
	vi := reflect.ValueOf(set)
	setData := &ultipa.SetData{}
	for index := 0; index < vi.Len(); index++ {
		//TODO if vi.Index(index) is ListValue?
		bs, err := ConvertInterfaceToBytesSafe(vi.Index(index).Interface(), subTypes[0], nil, req)
		if err != nil {
			return nil, err
		}
		setData.Values = append(setData.Values, bs)
	}
	return proto.Marshal(setData)
}

func SerializeMapData(value interface{}, subTypes []ultipa.PropertyType, req *configuration.RequestConfig) ([]byte, error) {
	if subTypes == nil && len(subTypes) == 0 {
		return nil, errors.New("subTypes is nil, unable to serialize SetData")
	}
	if len(subTypes) == 0 {
		return nil, errors.New("subTypes is not specified, unable to serialize SetData")
	}
	if value == nil {
		mapData := &ultipa.MapData{}
		mapData.IsNull = true
		return proto.Marshal(mapData)
	}

	switch t := value.(type) {
	case map[interface{}]interface{}:
		toSerializeValue := value.(map[interface{}]interface{})
		var mapValues []*ultipa.MapValue
		for k, v := range toSerializeValue {
			kb, err := ConvertInterfaceToBytesSafe(k, subTypes[0], nil, req)
			if err != nil {
				return nil, err
			}
			vb, err := ConvertInterfaceToBytesSafe(v, subTypes[0], nil, req)
			if err != nil {
				return nil, err
			}
			mapValue := &ultipa.MapValue{
				Key:   kb,
				Value: vb,
			}
			mapValues = append(mapValues, mapValue)
		}
		mapData := &ultipa.MapData{Values: mapValues}
		return proto.Marshal(mapData)
	default:
		return nil, errors.New(fmt.Sprintf("value is not a map, but %s, unable to serialize as MapData", t))
	}
}

func ConvertInterfaceToBytes(value interface{}) ([]byte, error) {
	v := []byte{}

	switch t := value.(type) {
	case int32:
		v = make([]byte, 4)
		binary.BigEndian.PutUint32(v, uint32(value.(int32)))
	case int:
		v = make([]byte, 4)
		binary.BigEndian.PutUint32(v, uint32(value.(int)))
	case string:
		v = []byte(value.(string))
	case int64:
		v = make([]byte, 8)
		binary.BigEndian.PutUint64(v, uint64(value.(int64)))
	case uint32:
		v = make([]byte, 4)
		binary.BigEndian.PutUint32(v, uint32(value.(uint32)))
	case uint64:
		v = make([]byte, 8)
		binary.BigEndian.PutUint64(v, uint64(value.(uint64)))
	case float32:
		v = make([]byte, 4)
		binary.BigEndian.PutUint32(v, math.Float32bits(value.(float32)))
	case float64:
		v = make([]byte, 8)
		binary.BigEndian.PutUint64(v, math.Float64bits(value.(float64)))
	case []byte:
		v = value.([]byte)
	default:
		return nil, errors.New(fmt.Sprint("not supported ultipa type for value: ", t))
	}

	return v, nil
}

func readBytes(value []byte, out interface{}) {
	bs := make([]byte, len(value))
	copy(bs, value)
	buff := bytes.NewBuffer(bs)
	binary.Read(buff, binary.BigEndian, out)
}

func AsInt32(value []byte) int32 {
	var out int32
	readBytes(value, &out)
	return out
}

func AsInt64(value []byte) int64 {
	var out int64
	readBytes(value, &out)
	return out
}

func AsUint32(value []byte) uint32 {
	var out uint32
	readBytes(value, &out)
	return out
}

func AsUint64(value []byte) uint64 {
	var out uint64
	readBytes(value, &out)
	return out
}

func AsFloat32(value []byte) float32 {
	var out float32
	readBytes(value, &out)
	return out
}

func AsFloat64(value []byte) float64 {
	var out float64
	readBytes(value, &out)
	return out
}

func AsString(value []byte) string {
	return string(value)
}

func AsPointString(value []byte) (string, error) {
	if len(value) != 16 {
		return "", errors.New("deserialize point type error: length != 16")
	}
	latitudeBytes := value[:8]
	latitude := AsFloat64(latitudeBytes)
	longitudeBytes := value[8:]
	longitude := AsFloat64(longitudeBytes)
	if math.IsNaN(latitude) || math.IsNaN(longitude) {
		return "", nil
	}
	//POINT(48.500000 22.200000)
	return fmt.Sprintf("POINT(%f %f)", latitude, longitude), nil
}

func AsBool(value []byte) bool {
	var out uint16
	readBytes(value, &out)
	if out == 1 {
		return true
	} else {
		return false
	}
}

func GetDefaultNilString(t ultipa.PropertyType) string {

	switch t {
	case ultipa.PropertyType_INT32:
		fallthrough
	case ultipa.PropertyType_INT64:
		fallthrough
	case ultipa.PropertyType_UINT32:
		fallthrough
	case ultipa.PropertyType_UINT64:
		fallthrough
	case ultipa.PropertyType_FLOAT:
		fallthrough
	case ultipa.PropertyType_DOUBLE:
		return "0"
	case ultipa.PropertyType_DATETIME:
		return "1970-01-01"
	case ultipa.PropertyType_TIMESTAMP:
		return "1970-01-01"
	default:
		return ""
	}

}

func GetDefaultNilInterface(t ultipa.PropertyType) interface{} {

	switch t {
	case ultipa.PropertyType_INT32:
		return math.MaxInt32
	case ultipa.PropertyType_INT64:
		return math.MaxInt64
	case ultipa.PropertyType_UINT32:
		return math.MaxUint32
	case ultipa.PropertyType_UINT64:
		return uint64(math.MaxUint64)
	case ultipa.PropertyType_FLOAT:
		return math.MaxFloat32
	case ultipa.PropertyType_DOUBLE:
		return math.MaxFloat64
	case ultipa.PropertyType_DATETIME:
		return uint64(math.MaxUint64)
	case ultipa.PropertyType_TIMESTAMP:
		return math.MaxUint32
	default:
		return default_nil_string
	}

}

func StringAsInterface(source string, t ultipa.PropertyType, location *time.Location) (interface{}, error) {

	str := strings.Trim(source, " ")

	if str == "" {
		if t == ultipa.PropertyType_DATETIME {
			//for empty string value of DATETIME, server will return 8 bytes 0, which is 1970-01-01 00:00:00, to keep align with that, set str = "1970-01-01"
			str = "1970-01-01"
		} else if t != ultipa.PropertyType_STRING && t != ultipa.PropertyType_TEXT {
			// SDK 4.3 support nil value, won't set default value again
			return nil, nil
		}
	}
	//if str == "" && t != ultipa.PropertyType_STRING && t != ultipa.PropertyType_TEXT {
	//str = GetDefaultNilString(t)
	// SDK 4.3 support nil value, won't set default value again
	//return nil, nil
	//}

	switch t {
	case ultipa.PropertyType_INT32:
		v, err := strconv.ParseInt(str, 10, 32)
		if err != nil {
			return nil, err
		}
		return int32(v), err
	case ultipa.PropertyType_INT64:
		return strconv.ParseInt(str, 10, 64)
	case ultipa.PropertyType_UINT32:
		v, err := strconv.ParseUint(str, 10, 32)
		if err != nil {
			return nil, err
		}
		return uint32(v), err
	case ultipa.PropertyType_UINT64:
		return strconv.ParseUint(str, 10, 64)
	case ultipa.PropertyType_FLOAT:
		v, err := strconv.ParseFloat(str, 32)
		if err != nil {
			return nil, err
		}
		return float32(v), err
	case ultipa.PropertyType_DOUBLE:
		v, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return nil, err
		}
		return float64(v), err
	case ultipa.PropertyType_DATETIME:
		v, err := NewDatetimeFromString(str)
		if err != nil {
			return nil, err
		}
		return v.Datetime, err
	case ultipa.PropertyType_TIMESTAMP:
		v, err := NewTimestampFromString(str, location)
		if err != nil {
			return nil, err
		}
		return v.GetTimeStamp(), err
	default:
		return str, nil
	}

	return nil, nil
}

func StringTimestampAsInterface(str string, location *time.Location) (interface{}, error) {

	str = strings.Trim(str, " ")

	if str == "" {
		str = GetDefaultNilString(ultipa.PropertyType_TIMESTAMP)
	}

	v, err := NewTimestampFromString(str, location)
	if err != nil {
		return nil, err
	}
	return v.GetTimeStamp(), err
}

func StringAsUUID(str string) (types.UUID, error) {
	str = strings.Trim(str, " ")
	v, err := strconv.ParseUint(str, 10, 64)
	return v, err
}

func GetLocationFromConfig(req *configuration.RequestConfig) *time.Location {
	if req == nil {
		return nil
	}
	if req.TimezoneOffset != "" {
		offsetSeconds, err := parseTimezoneOffset(req.TimezoneOffset)
		if err == nil {
			return time.FixedZone("UTC", offsetSeconds)
		}
	} else if req.Timezone != "" {
		return getLocationFromTimezone(req.Timezone)
	}

	return nil
}

func parseTimezoneOffset(offset string) (int, error) {
	if offset == "" {
		return 0, nil
	}

	// 正则匹配格式 ±hh:mm 或 ±hhmm
	re := regexp.MustCompile(`^([+-])(\d{2}):?(\d{2})$`)
	matches := re.FindStringSubmatch(offset)
	if len(matches) != 4 {
		return 0, fmt.Errorf("invalid timezoneOffset format: %s", offset)
	}

	sign := matches[1]
	hours, _ := strconv.Atoi(matches[2])
	minutes, _ := strconv.Atoi(matches[3])

	if hours > 23 || minutes > 59 {
		return 0, fmt.Errorf("invalid hour/minute in timezoneOffset: %s", offset)
	}

	total := hours*3600 + minutes*60
	if sign == "-" {
		total = -total
	}
	return total, nil
}

func getLocationFromTimezone(timezone string) *time.Location {
	if IsTimezoneOffsetCandidate(timezone) {
		offsetStr := timezone
		if strings.Contains(offsetStr, ":") {
			offsetStr = strings.ReplaceAll(offsetStr, ":", "")
		}
		offset, _ := strconv.ParseInt(offsetStr, 10, 64)
		if offset != 0 {
			return time.FixedZone("UTC", int(offset*36))
		}
	} else {
		if timezone != "" {
			location, err := time.LoadLocation(timezone)
			if err != nil {
				return nil
			}
			return location
		}
	}
	return nil
}

// IsTimezoneOffsetCandidate check whether offsetCandidate begin with +/- or not.
func IsTimezoneOffsetCandidate(offsetCandidate string) bool {
	return strings.HasPrefix(offsetCandidate, "+") || strings.HasPrefix(offsetCandidate, "-")
}
