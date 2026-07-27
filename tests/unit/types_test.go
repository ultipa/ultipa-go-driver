package unit

import (
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestTypedValueBool(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		expected bool
	}{
		{"true", true, true},
		{"false", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := gqldb.NewTypedValue(tt.input)
			if err != nil {
				t.Fatalf("NewTypedValue failed: %v", err)
			}

			if tv.Type != gqldb.PropertyTypeBool {
				t.Errorf("expected type %v, got %v", gqldb.PropertyTypeBool, tv.Type)
			}

			result, err := tv.ToGo()
			if err != nil {
				t.Fatalf("ToGo failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTypedValueInt32(t *testing.T) {
	tests := []struct {
		name     string
		input    int32
		expected int32
	}{
		{"zero", 0, 0},
		{"positive", 42, 42},
		{"negative", -100, -100},
		{"max", 2147483647, 2147483647},
		{"min", -2147483648, -2147483648},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := gqldb.NewTypedValue(tt.input)
			if err != nil {
				t.Fatalf("NewTypedValue failed: %v", err)
			}

			if tv.Type != gqldb.PropertyTypeInt32 {
				t.Errorf("expected type %v, got %v", gqldb.PropertyTypeInt32, tv.Type)
			}

			result, err := tv.ToGo()
			if err != nil {
				t.Fatalf("ToGo failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTypedValueInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected int64
	}{
		{"zero", 0, 0},
		{"positive", 9223372036854775807, 9223372036854775807},
		{"negative", -9223372036854775808, -9223372036854775808},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := gqldb.NewTypedValue(tt.input)
			if err != nil {
				t.Fatalf("NewTypedValue failed: %v", err)
			}

			if tv.Type != gqldb.PropertyTypeInt64 {
				t.Errorf("expected type %v, got %v", gqldb.PropertyTypeInt64, tv.Type)
			}

			result, err := tv.ToGo()
			if err != nil {
				t.Fatalf("ToGo failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTypedValueFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{"zero", 0.0, 0.0},
		{"positive", 3.14159, 3.14159},
		{"negative", -2.71828, -2.71828},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := gqldb.NewTypedValue(tt.input)
			if err != nil {
				t.Fatalf("NewTypedValue failed: %v", err)
			}

			if tv.Type != gqldb.PropertyTypeDouble {
				t.Errorf("expected type %v, got %v", gqldb.PropertyTypeDouble, tv.Type)
			}

			result, err := tv.ToGo()
			if err != nil {
				t.Fatalf("ToGo failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTypedValueString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"simple", "hello", "hello"},
		{"unicode", "你好世界", "你好世界"},
		{"special", "line1\nline2\ttab", "line1\nline2\ttab"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := gqldb.NewTypedValue(tt.input)
			if err != nil {
				t.Fatalf("NewTypedValue failed: %v", err)
			}

			if tv.Type != gqldb.PropertyTypeString {
				t.Errorf("expected type %v, got %v", gqldb.PropertyTypeString, tv.Type)
			}

			result, err := tv.ToGo()
			if err != nil {
				t.Fatalf("ToGo failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTypedValueBytes(t *testing.T) {
	input := []byte{0x00, 0x01, 0x02, 0xFF}

	tv, err := gqldb.NewTypedValue(input)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypeBlob {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypeBlob, tv.Type)
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	bytes, ok := result.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", result)
	}

	if len(bytes) != len(input) {
		t.Errorf("expected length %d, got %d", len(input), len(bytes))
	}

	for i, b := range bytes {
		if b != input[i] {
			t.Errorf("byte %d: expected %v, got %v", i, input[i], b)
		}
	}
}

func TestTypedValueTime(t *testing.T) {
	// TIMESTAMP uses 12-byte format: [int64 unix seconds (8B)] + [uint32 nanoseconds (4B)]
	input := time.Date(2024, 1, 15, 10, 30, 45, 123456789, time.UTC)

	tv, err := gqldb.NewTypedValue(input)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypeTimestamp {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypeTimestamp, tv.Type)
	}

	// Verify 12-byte encoding
	if len(tv.Data) != 12 {
		t.Fatalf("expected 12 bytes, got %d", len(tv.Data))
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	tm, ok := result.(time.Time)
	if !ok {
		t.Fatalf("expected time.Time, got %T", result)
	}

	if !tm.Equal(input) {
		t.Errorf("expected %v, got %v", input, tm)
	}
}

func TestTypedValueNull(t *testing.T) {
	tv, err := gqldb.NewTypedValue(nil)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypeNull {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypeNull, tv.Type)
	}

	if !tv.IsNull {
		t.Error("expected IsNull to be true")
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestTypedValueList(t *testing.T) {
	input := []interface{}{"a", int64(1), true}

	tv, err := gqldb.NewTypedValue(input)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypeList {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypeList, tv.Type)
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	list, ok := result.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", result)
	}

	if len(list) != len(input) {
		t.Errorf("expected length %d, got %d", len(input), len(list))
	}
}

func TestTypedValueMap(t *testing.T) {
	input := map[string]interface{}{
		"name": "test",
		"age":  int64(25),
	}

	tv, err := gqldb.NewTypedValue(input)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypeMap {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypeMap, tv.Type)
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result)
	}

	if m["name"] != "test" {
		t.Errorf("expected name=test, got %v", m["name"])
	}
}

func TestNewParameter(t *testing.T) {
	param, err := gqldb.NewParameter("name", "John")
	if err != nil {
		t.Fatalf("NewParameter failed: %v", err)
	}

	if param.Name != "name" {
		t.Errorf("expected name=name, got %v", param.Name)
	}

	if param.Value.Type != gqldb.PropertyTypeString {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypeString, param.Value.Type)
	}
}

// Additional PropertyType Tests

func TestTypedValueUint32(t *testing.T) {
	tests := []struct {
		name     string
		input    uint32
		expected uint32
	}{
		{"zero", 0, 0},
		{"positive", 42, 42},
		{"max", 4294967295, 4294967295},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := gqldb.NewTypedValue(tt.input)
			if err != nil {
				t.Fatalf("NewTypedValue failed: %v", err)
			}

			if tv.Type != gqldb.PropertyTypeUint32 {
				t.Errorf("expected type %v, got %v", gqldb.PropertyTypeUint32, tv.Type)
			}

			result, err := tv.ToGo()
			if err != nil {
				t.Fatalf("ToGo failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTypedValueUint64(t *testing.T) {
	tests := []struct {
		name     string
		input    uint64
		expected uint64
	}{
		{"zero", 0, 0},
		{"positive", 42, 42},
		{"large", 9223372036854775808, 9223372036854775808},
		{"max", 18446744073709551615, 18446744073709551615},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := gqldb.NewTypedValue(tt.input)
			if err != nil {
				t.Fatalf("NewTypedValue failed: %v", err)
			}

			if tv.Type != gqldb.PropertyTypeUint64 {
				t.Errorf("expected type %v, got %v", gqldb.PropertyTypeUint64, tv.Type)
			}

			result, err := tv.ToGo()
			if err != nil {
				t.Fatalf("ToGo failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTypedValueFloat32(t *testing.T) {
	tests := []struct {
		name     string
		input    float32
		expected float32
	}{
		{"zero", 0.0, 0.0},
		{"positive", 3.14, 3.14},
		{"negative", -2.71, -2.71},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := gqldb.NewTypedValue(tt.input)
			if err != nil {
				t.Fatalf("NewTypedValue failed: %v", err)
			}

			if tv.Type != gqldb.PropertyTypeFloat {
				t.Errorf("expected type %v, got %v", gqldb.PropertyTypeFloat, tv.Type)
			}

			result, err := tv.ToGo()
			if err != nil {
				t.Fatalf("ToGo failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTypedValuePoint(t *testing.T) {
	input := gqldb.Point{Latitude: 40.7128, Longitude: -74.0060}

	tv, err := gqldb.NewTypedValue(input)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypePoint {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypePoint, tv.Type)
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	pt, ok := result.(gqldb.Point)
	if !ok {
		t.Fatalf("expected Point, got %T", result)
	}

	if pt.Latitude != input.Latitude || pt.Longitude != input.Longitude {
		t.Errorf("expected %v, got %v", input, pt)
	}
}

func TestTypedValuePoint3D(t *testing.T) {
	input := gqldb.Point3D{X: 1.0, Y: 2.0, Z: 3.0}

	tv, err := gqldb.NewTypedValue(input)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypePoint3D {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypePoint3D, tv.Type)
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	pt, ok := result.(gqldb.Point3D)
	if !ok {
		t.Fatalf("expected Point3D, got %T", result)
	}

	if pt.X != input.X || pt.Y != input.Y || pt.Z != input.Z {
		t.Errorf("expected %v, got %v", input, pt)
	}
}

func TestTypedValueDecimal(t *testing.T) {
	input := gqldb.Decimal{Value: "123.456789012345678901234567890"}

	tv, err := gqldb.NewTypedValue(input)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypeDecimal {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypeDecimal, tv.Type)
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	dec, ok := result.(gqldb.Decimal)
	if !ok {
		t.Fatalf("expected Decimal, got %T", result)
	}

	if dec.Value != input.Value {
		t.Errorf("expected %v, got %v", input.Value, dec.Value)
	}
}

func TestTypedValueBigFloatDecimal(t *testing.T) {
	// A native *big.Float maps to the high-precision DECIMAL type (not a lossy
	// float64), mirroring the other SDKs' native-decimal support.
	f, _, err := big.ParseFloat("123.456", 10, 200, big.ToNearestEven)
	if err != nil {
		t.Fatalf("ParseFloat failed: %v", err)
	}

	tv, err := gqldb.NewTypedValue(f)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypeDecimal {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypeDecimal, tv.Type)
	}

	if got := string(tv.Data); got != "123.456" {
		t.Errorf("expected data %q, got %q", "123.456", got)
	}
}

func TestTypedValueLocalDateTime(t *testing.T) {
	input := gqldb.LocalDateTime{Time: time.Date(2024, 6, 15, 14, 30, 45, 123456789, time.UTC)}

	tv, err := gqldb.NewTypedValue(input)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypeLocalDatetime {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypeLocalDatetime, tv.Type)
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	ldt, ok := result.(gqldb.LocalDateTime)
	if !ok {
		t.Fatalf("expected LocalDateTime, got %T", result)
	}

	if ldt.Time.Year() != 2024 || ldt.Time.Month() != 6 || ldt.Time.Day() != 15 ||
		ldt.Time.Hour() != 14 || ldt.Time.Minute() != 30 || ldt.Time.Second() != 45 ||
		ldt.Time.Nanosecond() != 123456789 {
		t.Errorf("expected 2024-06-15T14:30:45.123456789, got %v", ldt.Time)
	}
}

func TestTypedValueZonedDateTime(t *testing.T) {
	loc := time.FixedZone("", 3600) // +1 hour
	now := time.Date(2024, 6, 15, 14, 30, 45, 123456789, loc)
	input := gqldb.ZonedDateTime{Time: now, OffsetMinutes: 60}

	tv, err := gqldb.NewTypedValue(input)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypeZonedDatetime {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypeZonedDatetime, tv.Type)
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	zdt, ok := result.(gqldb.ZonedDateTime)
	if !ok {
		t.Fatalf("expected ZonedDateTime, got %T", result)
	}

	if !zdt.Time.Equal(input.Time) || zdt.OffsetMinutes != input.OffsetMinutes {
		t.Errorf("expected %v, got %v", input, zdt)
	}
}

func TestTypedValueLocalTime(t *testing.T) {
	input := gqldb.LocalTime{Hour: 12, Minute: 0, Second: 0, Nanosecond: 0} // 12:00:00

	tv, err := gqldb.NewTypedValue(input)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypeLocalTime {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypeLocalTime, tv.Type)
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	lt, ok := result.(gqldb.LocalTime)
	if !ok {
		t.Fatalf("expected LocalTime, got %T", result)
	}

	if lt.Hour != input.Hour || lt.Minute != input.Minute || lt.Second != input.Second || lt.Nanosecond != input.Nanosecond {
		t.Errorf("expected %v, got %v", input, lt)
	}
}

func TestTypedValueZonedTime(t *testing.T) {
	input := gqldb.ZonedTime{Hour: 12, Minute: 0, Second: 0, Nanosecond: 0, OffsetMinutes: -300} // 12:00:00 -05:00

	tv, err := gqldb.NewTypedValue(input)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypeZonedTime {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypeZonedTime, tv.Type)
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	zt, ok := result.(gqldb.ZonedTime)
	if !ok {
		t.Fatalf("expected ZonedTime, got %T", result)
	}

	if zt.Hour != input.Hour || zt.Minute != input.Minute || zt.Second != input.Second || zt.Nanosecond != input.Nanosecond || zt.OffsetMinutes != input.OffsetMinutes {
		t.Errorf("expected %v, got %v", input, zt)
	}
}

func TestTypedValueYearToMonth(t *testing.T) {
	input := gqldb.YearToMonth{Months: 18} // 1 year 6 months

	tv, err := gqldb.NewTypedValue(input)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypeYearToMonth {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypeYearToMonth, tv.Type)
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	ytm, ok := result.(gqldb.YearToMonth)
	if !ok {
		t.Fatalf("expected YearToMonth, got %T", result)
	}

	if ytm.Months != input.Months {
		t.Errorf("expected %v, got %v", input.Months, ytm.Months)
	}
}

func TestTypedValueDayToSecond(t *testing.T) {
	input := gqldb.DayToSecond{Seconds: 86400, Nanoseconds: 0} // 1 day

	tv, err := gqldb.NewTypedValue(input)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypeDayToSecond {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypeDayToSecond, tv.Type)
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	dts, ok := result.(gqldb.DayToSecond)
	if !ok {
		t.Fatalf("expected DayToSecond, got %T", result)
	}

	if dts.Seconds != input.Seconds || dts.Nanoseconds != input.Nanoseconds {
		t.Errorf("expected %v, got %v", input, dts)
	}
}

func TestTypedValueVector(t *testing.T) {
	input := gqldb.Vector{Values: []float32{1.0, 2.0, 3.0, 4.0}}

	tv, err := gqldb.NewTypedValue(input)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypeVector {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypeVector, tv.Type)
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	vec, ok := result.(gqldb.Vector)
	if !ok {
		t.Fatalf("expected Vector, got %T", result)
	}

	if len(vec.Values) != len(input.Values) {
		t.Errorf("expected length %d, got %d", len(input.Values), len(vec.Values))
	}

	for i, v := range vec.Values {
		if v != input.Values[i] {
			t.Errorf("value %d: expected %v, got %v", i, input.Values[i], v)
		}
	}
}

func TestTypedValueSet(t *testing.T) {
	input := gqldb.Set{"a", int64(1), true}

	tv, err := gqldb.NewTypedValue(input)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypeSet {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypeSet, tv.Type)
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	set, ok := result.(gqldb.Set)
	if !ok {
		t.Fatalf("expected Set, got %T", result)
	}

	if len(set) != len(input) {
		t.Errorf("expected length %d, got %d", len(input), len(set))
	}
}

func TestTypedValueRecord(t *testing.T) {
	input := gqldb.Record{"name": "test", "value": float64(42)}

	tv, err := gqldb.NewTypedValue(input)
	if err != nil {
		t.Fatalf("NewTypedValue failed: %v", err)
	}

	if tv.Type != gqldb.PropertyTypeRecord {
		t.Errorf("expected type %v, got %v", gqldb.PropertyTypeRecord, tv.Type)
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	rec, ok := result.(gqldb.Record)
	if !ok {
		t.Fatalf("expected Record, got %T", result)
	}

	if rec["name"] != "test" {
		t.Errorf("expected name=test, got %v", rec["name"])
	}
}

// Decode-only tests for graph types (PATH, ERROR, NODE, EDGE)

func TestTypedValuePathDecode(t *testing.T) {
	// Create binary format PATH data with 1 node and 1 edge
	// Binary format: [nodeCount:2][nodeLen:4][nodeData]...[edgeCount:2][edgeLen:4][edgeData]...
	var data []byte

	// Build node binary: [idLen:2][id][labelCount:2][labelLen:2][label]...[propCount:2]
	nodeData := []byte{}
	// ID "n1"
	nodeData = append(nodeData, 2, 0) // idLen
	nodeData = append(nodeData, []byte("n1")...)
	// Label count 1
	nodeData = append(nodeData, 1, 0)
	// Label "Person"
	nodeData = append(nodeData, 6, 0) // labelLen
	nodeData = append(nodeData, []byte("Person")...)
	// Property count 0 (simplified)
	nodeData = append(nodeData, 0, 0)

	// Node count: 1
	data = append(data, 1, 0)
	// Node length
	nodeLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(nodeLen, uint32(len(nodeData)))
	data = append(data, nodeLen...)
	data = append(data, nodeData...)

	// Build edge binary: [idLen:2][id][labelLen:2][label][fromLen:2][from][toLen:2][to][propCount:2]
	edgeData := []byte{}
	// ID "e1"
	edgeData = append(edgeData, 2, 0)
	edgeData = append(edgeData, []byte("e1")...)
	// Label "KNOWS"
	edgeData = append(edgeData, 5, 0)
	edgeData = append(edgeData, []byte("KNOWS")...)
	// From "n1"
	edgeData = append(edgeData, 2, 0)
	edgeData = append(edgeData, []byte("n1")...)
	// To "n2"
	edgeData = append(edgeData, 2, 0)
	edgeData = append(edgeData, []byte("n2")...)
	// Property count 0
	edgeData = append(edgeData, 0, 0)

	// Edge count: 1
	data = append(data, 1, 0)
	// Edge length
	edgeLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(edgeLen, uint32(len(edgeData)))
	data = append(data, edgeLen...)
	data = append(data, edgeData...)

	tv := &gqldb.TypedValue{
		Type: gqldb.PropertyTypePath,
		Data: data,
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	path, ok := result.(gqldb.Path)
	if !ok {
		t.Fatalf("expected Path, got %T", result)
	}

	if len(path.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(path.Nodes))
	}

	if len(path.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(path.Edges))
	}

	if path.Nodes[0].ID != "n1" {
		t.Errorf("expected node ID n1, got %v", path.Nodes[0].ID)
	}

	if path.Edges[0].Label != "KNOWS" {
		t.Errorf("expected edge label KNOWS, got %v", path.Edges[0].Label)
	}
}

func TestTypedValueErrorDecode(t *testing.T) {
	jsonData := `{"code":1001,"message":"Something went wrong"}`

	tv := &gqldb.TypedValue{
		Type: gqldb.PropertyTypeError,
		Data: []byte(jsonData),
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	// Check if result implements error interface
	gqlErr, ok := result.(error)
	if !ok {
		t.Fatalf("expected error type, got %T", result)
	}

	errMsg := gqlErr.Error()
	if errMsg != "Something went wrong" {
		t.Errorf("expected message 'Something went wrong', got %v", errMsg)
	}
}

func TestTypedValueErrorDecodePlainMessage(t *testing.T) {
	tv := &gqldb.TypedValue{
		Type: gqldb.PropertyTypeError,
		Data: []byte("Plain error message"),
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	// Check if result implements error interface
	gqlErr, ok := result.(error)
	if !ok {
		t.Fatalf("expected error type, got %T", result)
	}

	errMsg := gqlErr.Error()
	if errMsg != "Plain error message" {
		t.Errorf("expected message 'Plain error message', got %v", errMsg)
	}
}

func TestTypedValueNodeDecode(t *testing.T) {
	// Create binary format NODE data
	// Binary format: [idLen:2][id][labelCount:2][labelLen:2][label]...[propCount:2]
	var data []byte

	// ID "node123"
	data = append(data, 7, 0) // idLen = 7
	data = append(data, []byte("node123")...)

	// Label count = 2
	data = append(data, 2, 0)

	// Label 1: "Person"
	data = append(data, 6, 0) // labelLen = 6
	data = append(data, []byte("Person")...)

	// Label 2: "Employee"
	data = append(data, 8, 0) // labelLen = 8
	data = append(data, []byte("Employee")...)

	// Property count = 0 (simplified for test)
	data = append(data, 0, 0)

	tv := &gqldb.TypedValue{
		Type: gqldb.PropertyTypeNode,
		Data: data,
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	node, ok := result.(*gqldb.Node)
	if !ok {
		t.Fatalf("expected *Node, got %T", result)
	}

	if node.ID != "node123" {
		t.Errorf("expected ID node123, got %v", node.ID)
	}

	if len(node.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(node.Labels))
	}
}

func TestTypedValueEdgeDecode(t *testing.T) {
	// Create binary format EDGE data
	// Binary format: [idLen:2][id][labelLen:2][label][fromLen:2][from][toLen:2][to][propCount:2]
	var data []byte

	// ID "edge456"
	data = append(data, 7, 0) // idLen = 7
	data = append(data, []byte("edge456")...)

	// Label "KNOWS"
	data = append(data, 5, 0) // labelLen = 5
	data = append(data, []byte("KNOWS")...)

	// From "n1"
	data = append(data, 2, 0) // fromLen = 2
	data = append(data, []byte("n1")...)

	// To "n2"
	data = append(data, 2, 0) // toLen = 2
	data = append(data, []byte("n2")...)

	// Property count = 0 (simplified for test)
	data = append(data, 0, 0)

	tv := &gqldb.TypedValue{
		Type: gqldb.PropertyTypeEdge,
		Data: data,
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	edge, ok := result.(*gqldb.Edge)
	if !ok {
		t.Fatalf("expected *Edge, got %T", result)
	}

	if edge.ID != "edge456" {
		t.Errorf("expected ID edge456, got %v", edge.ID)
	}

	if edge.Label != "KNOWS" {
		t.Errorf("expected label KNOWS, got %v", edge.Label)
	}

	if edge.FromNodeID != "n1" {
		t.Errorf("expected from_node_id n1, got %v", edge.FromNodeID)
	}

	if edge.ToNodeID != "n2" {
		t.Errorf("expected to_node_id n2, got %v", edge.ToNodeID)
	}
}

func TestTypedValueTableDecode(t *testing.T) {
	// Create binary format TABLE data (simplified)
	// Binary format: [colCount:2][colNameLen:2][colName]...[rowCount:2][TypedValueEntry]...
	var data []byte

	// Column count = 2
	data = append(data, 2, 0)

	// Column 1: "col1"
	data = append(data, 4, 0)
	data = append(data, []byte("col1")...)

	// Column 2: "col2"
	data = append(data, 4, 0)
	data = append(data, []byte("col2")...)

	// Row count = 0 (simplified for test)
	data = append(data, 0, 0)

	tv := &gqldb.TypedValue{
		Type: gqldb.PropertyTypeTable,
		Data: data,
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	// Table now decodes to GqldbTable
	table, ok := result.(gqldb.GqldbTable)
	if !ok {
		t.Fatalf("expected GqldbTable, got %T", result)
	}

	if len(table.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(table.Columns))
	}
}

func TestTypedValueTextDecode(t *testing.T) {
	tv := &gqldb.TypedValue{
		Type: gqldb.PropertyTypeText,
		Data: []byte("This is a text value"),
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	text, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}

	if text != "This is a text value" {
		t.Errorf("expected 'This is a text value', got %v", text)
	}
}

func TestTypedValueDateDecode(t *testing.T) {
	// New binary format: [year:u16][month:u8][day:u8][padding:4]
	data := make([]byte, 8)
	binary.LittleEndian.PutUint16(data[0:2], 2024)  // year
	data[2] = 1                                      // month (January)
	data[3] = 15                                     // day
	// padding bytes 4-7 are already zero

	tv := &gqldb.TypedValue{
		Type: gqldb.PropertyTypeDate,
		Data: data,
	}

	result, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}

	// Date now decodes to GqldbDate
	date, ok := result.(gqldb.GqldbDate)
	if !ok {
		t.Fatalf("expected GqldbDate, got %T", result)
	}

	if date.Year != 2024 || date.Month != 1 || date.Day != 15 {
		t.Errorf("expected 2024-01-15, got %d-%d-%d", date.Year, date.Month, date.Day)
	}
}
