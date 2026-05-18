package types

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"
)

// TypedValue represents a typed value that can be sent over the wire.
type TypedValue struct {
	Type   PropertyType
	Data   []byte
	IsNull bool
	// CachedGo holds a pre-decoded Go value populated by FromGo. When
	// non-nil, ToGo returns it directly instead of decoding Data. Used
	// by SDK paths that synthesize TypedValues from non-wire sources
	// (e.g. DeleteEdges reshaping 5 raw columns into a single EDGE
	// column).
	CachedGo interface{}
}

// FromGo wraps a pre-decoded Go value with a declared PropertyType.
// ToGo returns the value directly, bypassing binary decode.
func FromGo(t PropertyType, v interface{}) *TypedValue {
	if v == nil {
		return &TypedValue{Type: PropertyTypeNull, IsNull: true}
	}
	return &TypedValue{Type: t, CachedGo: v}
}

// NewTypedValue creates a TypedValue from a Go value.
func NewTypedValue(v interface{}) (*TypedValue, error) {
	if v == nil {
		return &TypedValue{Type: PropertyTypeNull, IsNull: true}, nil
	}

	switch val := v.(type) {
	case *TypedValue:
		return val, nil

	case TypedValue:
		return &val, nil

	case bool:
		data := make([]byte, 1)
		if val {
			data[0] = 1
		}
		return &TypedValue{Type: PropertyTypeBool, Data: data}, nil

	case int32:
		data := make([]byte, 4)
		binary.LittleEndian.PutUint32(data, uint32(val))
		return &TypedValue{Type: PropertyTypeInt32, Data: data}, nil

	case uint32:
		data := make([]byte, 4)
		binary.LittleEndian.PutUint32(data, val)
		return &TypedValue{Type: PropertyTypeUint32, Data: data}, nil

	case int64:
		data := make([]byte, 8)
		binary.LittleEndian.PutUint64(data, uint64(val))
		return &TypedValue{Type: PropertyTypeInt64, Data: data}, nil

	case int:
		data := make([]byte, 8)
		binary.LittleEndian.PutUint64(data, uint64(val))
		return &TypedValue{Type: PropertyTypeInt64, Data: data}, nil

	case uint64:
		data := make([]byte, 8)
		binary.LittleEndian.PutUint64(data, val)
		return &TypedValue{Type: PropertyTypeUint64, Data: data}, nil

	case float32:
		data := make([]byte, 4)
		binary.LittleEndian.PutUint32(data, math.Float32bits(val))
		return &TypedValue{Type: PropertyTypeFloat, Data: data}, nil

	case float64:
		data := make([]byte, 8)
		binary.LittleEndian.PutUint64(data, math.Float64bits(val))
		return &TypedValue{Type: PropertyTypeDouble, Data: data}, nil

	case string:
		return &TypedValue{Type: PropertyTypeString, Data: []byte(val)}, nil

	case []byte:
		return &TypedValue{Type: PropertyTypeBlob, Data: val}, nil

	case time.Time:
		// TIMESTAMP: 12-byte format [int64 unix seconds (8B LE)] + [uint32 nanoseconds (4B LE)]
		t := val.UTC()
		data := make([]byte, 12)
		binary.LittleEndian.PutUint64(data[0:8], uint64(t.Unix()))
		binary.LittleEndian.PutUint32(data[8:12], uint32(t.Nanosecond()))
		return &TypedValue{Type: PropertyTypeTimestamp, Data: data}, nil

	case Datetime:
		// DATETIME (deprecated): same as LOCAL_DATETIME, 11-byte structured format
		t := val.Time
		data := make([]byte, 11)
		// year is signed int16 so BCE years (e.g. -44) round-trip.
		binary.LittleEndian.PutUint16(data[0:2], uint16(int16(t.Year())))
		data[2] = uint8(t.Month())
		data[3] = uint8(t.Day())
		data[4] = uint8(t.Hour())
		data[5] = uint8(t.Minute())
		data[6] = uint8(t.Second())
		binary.LittleEndian.PutUint32(data[7:11], uint32(t.Nanosecond()))
		return &TypedValue{Type: PropertyTypeDatetime, Data: data}, nil

	case []interface{}:
		// For lists, we encode each element and concatenate
		return encodeList(val)

	case map[string]interface{}:
		return encodeMap(val)

	case Point:
		data := make([]byte, 16)
		binary.LittleEndian.PutUint64(data[0:8], math.Float64bits(val.Longitude))
		binary.LittleEndian.PutUint64(data[8:16], math.Float64bits(val.Latitude))
		return &TypedValue{Type: PropertyTypePoint, Data: data}, nil

	case GqldbDate:
		data := make([]byte, 8)
		// year is signed int16; uint16 cast preserves the bit pattern.
		binary.LittleEndian.PutUint16(data[0:2], uint16(val.Year))
		data[2] = val.Month
		data[3] = val.Day
		// bytes 4-7 are padding (zeros)
		return &TypedValue{Type: PropertyTypeDate, Data: data}, nil

	case Point3D:
		data := make([]byte, 24)
		binary.LittleEndian.PutUint64(data[0:8], math.Float64bits(val.X))
		binary.LittleEndian.PutUint64(data[8:16], math.Float64bits(val.Y))
		binary.LittleEndian.PutUint64(data[16:24], math.Float64bits(val.Z))
		return &TypedValue{Type: PropertyTypePoint3D, Data: data}, nil

	case Decimal:
		return &TypedValue{Type: PropertyTypeDecimal, Data: []byte(val.Value)}, nil

	case LocalDateTime:
		// LOCAL_DATETIME: 11-byte structured format [year:2][month:1][day:1][hour:1][min:1][sec:1][nanos:4]
		t := val.Time
		data := make([]byte, 11)
		// year is signed int16 so BCE years (e.g. -44) round-trip.
		binary.LittleEndian.PutUint16(data[0:2], uint16(int16(t.Year())))
		data[2] = uint8(t.Month())
		data[3] = uint8(t.Day())
		data[4] = uint8(t.Hour())
		data[5] = uint8(t.Minute())
		data[6] = uint8(t.Second())
		binary.LittleEndian.PutUint32(data[7:11], uint32(t.Nanosecond()))
		return &TypedValue{Type: PropertyTypeLocalDatetime, Data: data}, nil

	case ZonedDateTime:
		// ZONED_DATETIME: 13-byte structured format [year:2][month:1][day:1][hour:1][min:1][sec:1][nanos:4][offset_min:2]
		t := val.Time
		data := make([]byte, 13)
		// year is signed int16 so BCE years (e.g. -44) round-trip.
		binary.LittleEndian.PutUint16(data[0:2], uint16(int16(t.Year())))
		data[2] = uint8(t.Month())
		data[3] = uint8(t.Day())
		data[4] = uint8(t.Hour())
		data[5] = uint8(t.Minute())
		data[6] = uint8(t.Second())
		binary.LittleEndian.PutUint32(data[7:11], uint32(t.Nanosecond()))
		binary.LittleEndian.PutUint16(data[11:13], uint16(val.OffsetMinutes))
		return &TypedValue{Type: PropertyTypeZonedDatetime, Data: data}, nil

	case LocalTime:
		data := make([]byte, 8)
		data[0] = val.Hour
		data[1] = val.Minute
		data[2] = val.Second
		// byte 3 is padding
		binary.LittleEndian.PutUint32(data[4:8], val.Nanosecond)
		return &TypedValue{Type: PropertyTypeLocalTime, Data: data}, nil

	case ZonedTime:
		data := make([]byte, 10)
		data[0] = val.Hour
		data[1] = val.Minute
		data[2] = val.Second
		// byte 3 is padding
		binary.LittleEndian.PutUint32(data[4:8], val.Nanosecond)
		binary.LittleEndian.PutUint16(data[8:10], uint16(val.OffsetMinutes))
		return &TypedValue{Type: PropertyTypeZonedTime, Data: data}, nil

	case YearToMonth:
		data := make([]byte, 4)
		binary.LittleEndian.PutUint32(data, uint32(val.Months))
		return &TypedValue{Type: PropertyTypeYearToMonth, Data: data}, nil

	case DayToSecond:
		data := make([]byte, 12)
		binary.LittleEndian.PutUint64(data[0:8], uint64(val.Seconds))
		binary.LittleEndian.PutUint32(data[8:12], val.Nanoseconds)
		return &TypedValue{Type: PropertyTypeDayToSecond, Data: data}, nil

	case Vector:
		data := make([]byte, 4+4*len(val.Values))
		binary.LittleEndian.PutUint32(data[0:4], uint32(len(val.Values)))
		for i, v := range val.Values {
			binary.LittleEndian.PutUint32(data[4+i*4:8+i*4], math.Float32bits(v))
		}
		return &TypedValue{Type: PropertyTypeVector, Data: data}, nil

	case Record:
		tv, err := encodeMap(map[string]interface{}(val))
		if err != nil {
			return nil, err
		}
		tv.Type = PropertyTypeRecord
		return tv, nil

	case Set:
		return encodeSet(val)

	default:
		return nil, fmt.Errorf("unsupported type: %T", v)
	}
}

// ToGo converts a TypedValue to a Go value.
func (tv *TypedValue) ToGo() (interface{}, error) {
	if tv.IsNull {
		return nil, nil
	}
	if tv.CachedGo != nil {
		return tv.CachedGo, nil
	}

	switch tv.Type {
	case PropertyTypeNull:
		return nil, nil

	case PropertyTypeBool:
		if len(tv.Data) < 1 {
			return false, nil
		}
		return tv.Data[0] != 0, nil

	case PropertyTypeInt32:
		if len(tv.Data) < 4 {
			return int32(0), nil
		}
		return int32(binary.LittleEndian.Uint32(tv.Data)), nil

	case PropertyTypeUint32:
		if len(tv.Data) < 4 {
			return uint32(0), nil
		}
		return binary.LittleEndian.Uint32(tv.Data), nil

	case PropertyTypeInt64:
		if len(tv.Data) < 8 {
			return int64(0), nil
		}
		return int64(binary.LittleEndian.Uint64(tv.Data)), nil

	case PropertyTypeUint64:
		if len(tv.Data) < 8 {
			return uint64(0), nil
		}
		return binary.LittleEndian.Uint64(tv.Data), nil

	case PropertyTypeFloat:
		if len(tv.Data) < 4 {
			return float32(0), nil
		}
		return math.Float32frombits(binary.LittleEndian.Uint32(tv.Data)), nil

	case PropertyTypeDouble:
		if len(tv.Data) < 8 {
			return float64(0), nil
		}
		return math.Float64frombits(binary.LittleEndian.Uint64(tv.Data)), nil

	case PropertyTypeString, PropertyTypeText:
		return string(tv.Data), nil

	case PropertyTypeBlob:
		if tv.Data == nil {
			return []byte{}, nil
		}
		return tv.Data, nil

	case PropertyTypeTimestamp:
		if len(tv.Data) < 4 {
			return time.Time{}, nil
		}
		// 12-byte format: [int64 unix seconds (8B LE)] + [uint32 nanoseconds (4B LE)]
		if len(tv.Data) >= 12 {
			secs := int64(binary.LittleEndian.Uint64(tv.Data[0:8]))
			nanos := int64(binary.LittleEndian.Uint32(tv.Data[8:12]))
			return time.Unix(secs, nanos).UTC(), nil
		}
		return time.Time{}, nil

	case PropertyTypeDate:
		if len(tv.Data) < 4 {
			return GqldbDate{}, nil
		}
		return GqldbDate{
			// year is signed int16; int16 cast restores the sign.
			Year:  int16(binary.LittleEndian.Uint16(tv.Data[0:2])),
			Month: tv.Data[2],
			Day:   tv.Data[3],
		}, nil

	case PropertyTypeList:
		return decodeList(tv.Data)

	case PropertyTypeMap:
		return decodeMap(tv.Data)

	case PropertyTypeRecord:
		m, err := decodeMap(tv.Data)
		if err != nil {
			return nil, err
		}
		return Record(m), nil

	case PropertyTypePoint:
		if len(tv.Data) < 16 {
			return Point{}, nil
		}
		return Point{
			Longitude: math.Float64frombits(binary.LittleEndian.Uint64(tv.Data[0:8])),
			Latitude:  math.Float64frombits(binary.LittleEndian.Uint64(tv.Data[8:16])),
		}, nil

	case PropertyTypePoint3D:
		if len(tv.Data) < 24 {
			return Point3D{}, nil
		}
		return Point3D{
			X: math.Float64frombits(binary.LittleEndian.Uint64(tv.Data[0:8])),
			Y: math.Float64frombits(binary.LittleEndian.Uint64(tv.Data[8:16])),
			Z: math.Float64frombits(binary.LittleEndian.Uint64(tv.Data[16:24])),
		}, nil

	case PropertyTypeDecimal:
		return Decimal{Value: string(tv.Data)}, nil

	case PropertyTypeSet:
		return decodeSet(tv.Data)

	case PropertyTypeLocalDatetime, PropertyTypeDatetime:
		// LOCAL_DATETIME/DATETIME: 11-byte structured format [year:2][month:1][day:1][hour:1][min:1][sec:1][nanos:4]
		if len(tv.Data) < 7 {
			return LocalDateTime{}, nil
		}
		// year is signed int16 so BCE years (e.g. -44) round-trip.
		year := int(int16(binary.LittleEndian.Uint16(tv.Data[0:2])))
		month := time.Month(tv.Data[2])
		day := int(tv.Data[3])
		hour := int(tv.Data[4])
		min := int(tv.Data[5])
		sec := int(tv.Data[6])
		var nsec int
		if len(tv.Data) >= 11 {
			nsec = int(binary.LittleEndian.Uint32(tv.Data[7:11]))
		}
		return LocalDateTime{Time: time.Date(year, month, day, hour, min, sec, nsec, time.UTC)}, nil

	case PropertyTypeZonedDatetime:
		// ZONED_DATETIME: 13-byte structured format [year:2][month:1][day:1][hour:1][min:1][sec:1][nanos:4][offset_min:2]
		if len(tv.Data) < 7 {
			return ZonedDateTime{}, nil
		}
		// year is signed int16 so BCE years (e.g. -44) round-trip.
		year := int(int16(binary.LittleEndian.Uint16(tv.Data[0:2])))
		month := time.Month(tv.Data[2])
		day := int(tv.Data[3])
		hour := int(tv.Data[4])
		min := int(tv.Data[5])
		sec := int(tv.Data[6])
		var nsec int
		var offset int16
		if len(tv.Data) >= 11 {
			nsec = int(binary.LittleEndian.Uint32(tv.Data[7:11]))
		}
		if len(tv.Data) >= 13 {
			offset = int16(binary.LittleEndian.Uint16(tv.Data[11:13]))
		}
		loc := time.FixedZone("", int(offset)*60)
		return ZonedDateTime{
			Time:          time.Date(year, month, day, hour, min, sec, nsec, loc),
			OffsetMinutes: offset,
		}, nil

	case PropertyTypeLocalTime:
		if len(tv.Data) < 8 {
			return LocalTime{}, nil
		}
		return LocalTime{
			Hour:       tv.Data[0],
			Minute:     tv.Data[1],
			Second:     tv.Data[2],
			Nanosecond: binary.LittleEndian.Uint32(tv.Data[4:8]),
		}, nil

	case PropertyTypeZonedTime:
		if len(tv.Data) < 10 {
			return ZonedTime{}, nil
		}
		return ZonedTime{
			Hour:          tv.Data[0],
			Minute:        tv.Data[1],
			Second:        tv.Data[2],
			Nanosecond:    binary.LittleEndian.Uint32(tv.Data[4:8]),
			OffsetMinutes: int16(binary.LittleEndian.Uint16(tv.Data[8:10])),
		}, nil

	case PropertyTypeYearToMonth:
		if len(tv.Data) < 4 {
			return YearToMonth{}, nil
		}
		return YearToMonth{Months: int32(binary.LittleEndian.Uint32(tv.Data))}, nil

	case PropertyTypeDayToSecond:
		if len(tv.Data) < 12 {
			return DayToSecond{}, nil
		}
		return DayToSecond{
			Seconds:     int64(binary.LittleEndian.Uint64(tv.Data[0:8])),
			Nanoseconds: binary.LittleEndian.Uint32(tv.Data[8:12]),
		}, nil

	case PropertyTypeVector:
		if len(tv.Data) < 4 {
			return Vector{}, nil
		}
		dim := binary.LittleEndian.Uint32(tv.Data[0:4])
		if len(tv.Data) < int(4+dim*4) {
			return Vector{}, nil
		}
		values := make([]float32, dim)
		for i := uint32(0); i < dim; i++ {
			values[i] = math.Float32frombits(binary.LittleEndian.Uint32(tv.Data[4+i*4 : 8+i*4]))
		}
		return Vector{Values: values}, nil

	case PropertyTypeTable:
		return decodeTable(tv.Data), nil

	case PropertyTypePath:
		return decodePath(tv.Data), nil

	case PropertyTypeError:
		return decodeError(tv.Data), nil

	case PropertyTypeNode:
		return decodeNode(tv.Data), nil

	case PropertyTypeEdge:
		return decodeEdge(tv.Data), nil

	default:
		// For unsupported types, try to parse as JSON first, then return as string
		if len(tv.Data) > 0 && (tv.Data[0] == '{' || tv.Data[0] == '[') {
			var result interface{}
			if err := json.Unmarshal(tv.Data, &result); err == nil {
				return result, nil
			}
		}
		// Return as string instead of raw bytes to avoid base64 encoding in JSON
		return string(tv.Data), nil
	}
}

// Parameter represents a named query parameter.
type Parameter struct {
	Name  string
	Value *TypedValue
}

// NewParameter creates a new parameter from a name and Go value.
func NewParameter(name string, value interface{}) (*Parameter, error) {
	tv, err := NewTypedValue(value)
	if err != nil {
		return nil, err
	}
	return &Parameter{Name: name, Value: tv}, nil
}

// =============================================================================
// Binary Format Helpers
// =============================================================================

// decodeString decodes a length-prefixed string: [len:2 uint16LE][utf8 bytes]
// Returns the string and the number of bytes consumed.
func decodeString(data []byte, offset int) (string, int) {
	if offset+2 > len(data) {
		return "", 0
	}
	length := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	if offset+2+length > len(data) {
		return "", 0
	}
	return string(data[offset+2 : offset+2+length]), 2 + length
}

// decodeTypedValueEntry decodes a TypedValueEntry: [type:1][isNull:1][dataLen:4 uint32LE][data]
// Returns the TypedValue and the number of bytes consumed.
func decodeTypedValueEntry(data []byte, offset int) (*TypedValue, int) {
	if offset+6 > len(data) {
		return &TypedValue{Type: PropertyTypeNull, IsNull: true}, 0
	}
	tvType := PropertyType(data[offset])
	isNull := data[offset+1] != 0
	dataLen := int(binary.LittleEndian.Uint32(data[offset+2 : offset+6]))
	if offset+6+dataLen > len(data) {
		return &TypedValue{Type: PropertyTypeNull, IsNull: true}, 0
	}
	tvData := data[offset+6 : offset+6+dataLen]
	return &TypedValue{Type: tvType, Data: tvData, IsNull: isNull}, 6 + dataLen
}

// decodePropertiesBinary decodes binary properties: [count:2][keyLen:2][key][TVE]...
// Returns the properties map and the number of bytes consumed.
func decodePropertiesBinary(data []byte, offset int) (map[string]interface{}, int) {
	if offset+2 > len(data) {
		return map[string]interface{}{}, 0
	}
	count := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	pos := offset + 2
	result := make(map[string]interface{})

	for i := 0; i < count; i++ {
		key, keyLen := decodeString(data, pos)
		if keyLen == 0 {
			break
		}
		pos += keyLen

		tv, tvLen := decodeTypedValueEntry(data, pos)
		if tvLen == 0 {
			break
		}
		pos += tvLen

		val, _ := tv.ToGo()
		result[key] = val
	}

	return result, pos - offset
}

// decodeNode decodes a node from binary format.
//
// Wire format:
//
//	pre-6.1.147:  [idLen:2][id][labelCount:2][labelLen:2][label]...[properties_binary]
//	6.1.147+:     ...[properties_binary][InternalID:8 LE uint64]
//
// The 8-byte InternalID trailer is read when present; absent on
// pre-6.1.147 servers, in which case Node.UUID is left empty and
// application code can fall back to Node.ID.
func decodeNode(data []byte) *Node {
	offset := 0

	// Decode ID
	id, idLen := decodeString(data, offset)
	if idLen == 0 {
		return &Node{ID: "", Labels: []string{}, Properties: map[string]interface{}{}}
	}
	offset += idLen

	// Decode labels
	if offset+2 > len(data) {
		return &Node{ID: id, Labels: []string{}, Properties: map[string]interface{}{}}
	}
	labelCount := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2

	labels := make([]string, 0, labelCount)
	for i := 0; i < labelCount; i++ {
		label, labelLen := decodeString(data, offset)
		if labelLen == 0 {
			break
		}
		labels = append(labels, label)
		offset += labelLen
	}

	// Decode properties
	properties, propsLen := decodePropertiesBinary(data, offset)
	offset += propsLen

	// Decode optional InternalID trailer (6.1.147+ wire format).
	uuid := decodeInternalIDTrailer(data, offset)

	return &Node{ID: id, UUID: uuid, Labels: labels, Properties: properties}
}

// decodeEdge decodes an edge from binary format. See decodeNode for
// the InternalID trailer semantics.
func decodeEdge(data []byte) *Edge {
	offset := 0

	id, idLen := decodeString(data, offset)
	if idLen == 0 {
		return &Edge{ID: "", Label: "", FromNodeID: "", ToNodeID: "", Properties: map[string]interface{}{}}
	}
	offset += idLen

	label, labelLen := decodeString(data, offset)
	if labelLen == 0 {
		return &Edge{ID: id, Label: "", FromNodeID: "", ToNodeID: "", Properties: map[string]interface{}{}}
	}
	offset += labelLen

	fromNodeID, fromLen := decodeString(data, offset)
	if fromLen == 0 {
		return &Edge{ID: id, Label: label, FromNodeID: "", ToNodeID: "", Properties: map[string]interface{}{}}
	}
	offset += fromLen

	toNodeID, toLen := decodeString(data, offset)
	if toLen == 0 {
		return &Edge{ID: id, Label: label, FromNodeID: fromNodeID, ToNodeID: "", Properties: map[string]interface{}{}}
	}
	offset += toLen

	properties, propsLen := decodePropertiesBinary(data, offset)
	offset += propsLen

	uuid := decodeInternalIDTrailer(data, offset)

	return &Edge{ID: id, UUID: uuid, Label: label, FromNodeID: fromNodeID, ToNodeID: toNodeID, Properties: properties}
}

// decodeInternalIDTrailer reads the 8-byte little-endian uint64
// InternalID emitted by gqldb 6.1.147+ servers at the tail of every
// encoded node/edge payload. Returns the value formatted as a decimal
// string. Returns "" when the trailer is missing (pre-6.1.147 wire
// format) so application code can fall back to `_id`.
func decodeInternalIDTrailer(data []byte, offset int) string {
	if offset+8 > len(data) {
		return ""
	}
	id := binary.LittleEndian.Uint64(data[offset : offset+8])
	return strconv.FormatUint(id, 10)
}

// decodePath decodes a path from binary format.
// Format: [nodeCount:2][nodeLen:4][nodeData]...[edgeCount:2][edgeLen:4][edgeData]...
func decodePath(data []byte) Path {
	offset := 0

	// Decode nodes
	if offset+2 > len(data) {
		return Path{Nodes: []*Node{}, Edges: []*Edge{}}
	}
	nodeCount := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2

	nodes := make([]*Node, 0, nodeCount)
	for i := 0; i < nodeCount; i++ {
		if offset+4 > len(data) {
			break
		}
		nodeLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4

		if offset+nodeLen > len(data) {
			break
		}
		nodeData := data[offset : offset+nodeLen]
		nodes = append(nodes, decodeNode(nodeData))
		offset += nodeLen
	}

	// Decode edges
	if offset+2 > len(data) {
		return Path{Nodes: nodes, Edges: []*Edge{}}
	}
	edgeCount := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2

	edges := make([]*Edge, 0, edgeCount)
	for i := 0; i < edgeCount; i++ {
		if offset+4 > len(data) {
			break
		}
		edgeLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4

		if offset+edgeLen > len(data) {
			break
		}
		edgeData := data[offset : offset+edgeLen]
		edges = append(edges, decodeEdge(edgeData))
		offset += edgeLen
	}

	return Path{Nodes: nodes, Edges: edges}
}

// decodeTable decodes a table from binary format.
// Format: [colCount:2][colLen:2][col]...[rowCount:2][cellCount:2][TVE]...
func decodeTable(data []byte) GqldbTable {
	offset := 0

	// Decode columns
	if offset+2 > len(data) {
		return GqldbTable{Columns: []string{}, Rows: [][]interface{}{}}
	}
	colCount := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2

	columns := make([]string, 0, colCount)
	for i := 0; i < colCount; i++ {
		col, colLen := decodeString(data, offset)
		if colLen == 0 {
			break
		}
		columns = append(columns, col)
		offset += colLen
	}

	// Decode rows
	if offset+2 > len(data) {
		return GqldbTable{Columns: columns, Rows: [][]interface{}{}}
	}
	rowCount := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2

	rows := make([][]interface{}, 0, rowCount)
	for i := 0; i < rowCount; i++ {
		if offset+2 > len(data) {
			break
		}
		cellCount := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
		offset += 2

		row := make([]interface{}, 0, cellCount)
		for j := 0; j < cellCount; j++ {
			tv, tvLen := decodeTypedValueEntry(data, offset)
			if tvLen == 0 {
				break
			}
			val, _ := tv.ToGo()
			row = append(row, val)
			offset += tvLen
		}
		rows = append(rows, row)
	}

	return GqldbTable{Columns: columns, Rows: rows}
}

// decodeError decodes an error from binary/JSON format.
func decodeError(data []byte) *GqldbError {
	var errData struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &errData); err != nil {
		// If not valid JSON, treat data as plain error message
		return &GqldbError{Message: string(data)}
	}
	return &GqldbError{Code: errData.Code, Message: errData.Message}
}
