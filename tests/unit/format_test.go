package unit

import (
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
	"github.com/ultipa/ultipa-go-driver/v6/types"
)

func TestFormatAndParseDate(t *testing.T) {
	d := types.GqldbDate{Year: 1993, Month: 5, Day: 6}
	tv, _ := gqldb.NewTypedValue(d)
	s, err := tv.FormatValue()
	if err != nil {
		t.Fatal(err)
	}
	if s != "1993-05-06" {
		t.Fatalf("expected 1993-05-06, got %q", s)
	}
	tv2, err := gqldb.NewTypedValueFromString(s, gqldb.PropertyTypeDate)
	if err != nil {
		t.Fatal(err)
	}
	val, _ := tv2.ToGo()
	d2 := val.(types.GqldbDate)
	if d2 != d {
		t.Fatalf("roundtrip failed: %+v != %+v", d2, d)
	}
}

func TestFormatAndParseLocalTime(t *testing.T) {
	tests := []struct {
		name string
		lt   types.LocalTime
		want string
	}{
		{"no_nanos", types.LocalTime{Hour: 9, Minute: 11, Second: 2}, "09:11:02"},
		{"with_nanos", types.LocalTime{Hour: 14, Minute: 30, Second: 0, Nanosecond: 123456789}, "14:30:00.123456789"},
		{"trailing_zeros", types.LocalTime{Hour: 14, Minute: 30, Second: 0, Nanosecond: 100000000}, "14:30:00.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, _ := gqldb.NewTypedValue(tt.lt)
			s, _ := tv.FormatValue()
			if s != tt.want {
				t.Fatalf("FormatValue: expected %q, got %q", tt.want, s)
			}
			tv2, err := gqldb.NewTypedValueFromString(s, gqldb.PropertyTypeLocalTime)
			if err != nil {
				t.Fatal(err)
			}
			val, _ := tv2.ToGo()
			lt2 := val.(types.LocalTime)
			if lt2 != tt.lt {
				t.Fatalf("roundtrip failed: %+v != %+v", lt2, tt.lt)
			}
		})
	}
}

func TestFormatAndParseZonedTime(t *testing.T) {
	zt := types.ZonedTime{Hour: 9, Minute: 11, Second: 2, OffsetMinutes: -480}
	tv, _ := gqldb.NewTypedValue(zt)
	s, _ := tv.FormatValue()
	if s != "09:11:02-08:00" {
		t.Fatalf("expected 09:11:02-08:00, got %q", s)
	}
	tv2, err := gqldb.NewTypedValueFromString(s, gqldb.PropertyTypeZonedTime)
	if err != nil {
		t.Fatal(err)
	}
	val, _ := tv2.ToGo()
	zt2 := val.(types.ZonedTime)
	if zt2 != zt {
		t.Fatalf("roundtrip failed: %+v != %+v", zt2, zt)
	}
}

func TestFormatAndParseLocalDateTime(t *testing.T) {
	ldt := types.LocalDateTime{Time: time.Date(1993, 5, 6, 9, 11, 2, 0, time.UTC)}
	tv, _ := gqldb.NewTypedValue(ldt)
	s, _ := tv.FormatValue()
	if s != "1993-05-06 09:11:02" {
		t.Fatalf("expected '1993-05-06 09:11:02', got %q", s)
	}
	tv2, err := gqldb.NewTypedValueFromString(s, gqldb.PropertyTypeLocalDatetime)
	if err != nil {
		t.Fatal(err)
	}
	val, _ := tv2.ToGo()
	ldt2 := val.(types.LocalDateTime)
	if !ldt2.Time.Equal(ldt.Time) {
		t.Fatalf("roundtrip failed: %v != %v", ldt2.Time, ldt.Time)
	}
}

func TestFormatAndParseZonedDateTime(t *testing.T) {
	loc := time.FixedZone("", -8*3600)
	zdt := types.ZonedDateTime{
		Time:          time.Date(1993, 5, 6, 9, 11, 2, 0, loc),
		OffsetMinutes: -480,
	}
	tv, _ := gqldb.NewTypedValue(zdt)
	s, _ := tv.FormatValue()
	if s != "1993-05-06 09:11:02-08:00" {
		t.Fatalf("expected '1993-05-06 09:11:02-08:00', got %q", s)
	}
	tv2, err := gqldb.NewTypedValueFromString(s, gqldb.PropertyTypeZonedDatetime)
	if err != nil {
		t.Fatal(err)
	}
	val, _ := tv2.ToGo()
	zdt2 := val.(types.ZonedDateTime)
	if zdt2.OffsetMinutes != zdt.OffsetMinutes {
		t.Fatalf("offset mismatch: %d != %d", zdt2.OffsetMinutes, zdt.OffsetMinutes)
	}
	if !zdt2.Time.Equal(zdt.Time) {
		t.Fatalf("time mismatch: %v != %v", zdt2.Time, zdt.Time)
	}
}

func TestFormatAndParseTimestamp(t *testing.T) {
	ts := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)
	tv, _ := gqldb.NewTypedValue(ts)
	s, _ := tv.FormatValue()
	if s != "2024-06-15 14:30:00" {
		t.Fatalf("expected '2024-06-15 14:30:00', got %q", s)
	}
	// Parse back as timestamp
	tv2, err := gqldb.NewTypedValueFromString(s, gqldb.PropertyTypeTimestamp)
	if err != nil {
		t.Fatal(err)
	}
	val, _ := tv2.ToGo()
	ts2 := val.(time.Time)
	if !ts2.Equal(ts) {
		t.Fatalf("roundtrip failed: %v != %v", ts2, ts)
	}

	// Test epoch seconds parsing
	tv3, err := gqldb.NewTypedValueFromString("1715169600", gqldb.PropertyTypeTimestamp)
	if err != nil {
		t.Fatal(err)
	}
	val3, _ := tv3.ToGo()
	ts3 := val3.(time.Time)
	if ts3.Unix() != 1715169600 {
		t.Fatalf("epoch parse failed: expected 1715169600, got %d", ts3.Unix())
	}

	// Test timestamp with timezone offset - should convert to UTC
	offsetTests := []struct {
		input    string
		expected time.Time
	}{
		{"2024-06-15 14:30:00+08:00", time.Date(2024, 6, 15, 6, 30, 0, 0, time.UTC)},
		{"2024-06-15 14:30:00-05:00", time.Date(2024, 6, 15, 19, 30, 0, 0, time.UTC)},
		{"2024-06-15 14:30:00+00:00", time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)},
		{"2024-06-15T14:30:00+0200", time.Date(2024, 6, 15, 12, 30, 0, 0, time.UTC)},
	}
	for _, tt := range offsetTests {
		t.Run(tt.input, func(t *testing.T) {
			tv4, err := gqldb.NewTypedValueFromString(tt.input, gqldb.PropertyTypeTimestamp)
			if err != nil {
				t.Fatal(err)
			}
			val4, _ := tv4.ToGo()
			ts4 := val4.(time.Time)
			if !ts4.Equal(tt.expected) {
				t.Fatalf("offset parse: expected %v, got %v", tt.expected, ts4)
			}
		})
	}
}

func TestFormatAndParseYearToMonth(t *testing.T) {
	tests := []struct {
		months int32
		want   string
	}{
		{0, "P0M"},
		{29, "P2Y5M"},
		{-17, "-P1Y5M"},
		{12, "P1Y"},
		{3, "P3M"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			ytm := types.YearToMonth{Months: tt.months}
			tv, _ := gqldb.NewTypedValue(ytm)
			s, _ := tv.FormatValue()
			if s != tt.want {
				t.Fatalf("FormatValue: expected %q, got %q", tt.want, s)
			}
			tv2, err := gqldb.NewTypedValueFromString(s, gqldb.PropertyTypeYearToMonth)
			if err != nil {
				t.Fatal(err)
			}
			val, _ := tv2.ToGo()
			ytm2 := val.(types.YearToMonth)
			if ytm2.Months != tt.months {
				t.Fatalf("roundtrip failed: %d != %d", ytm2.Months, tt.months)
			}
		})
	}
}

func TestFormatAndParseDayToSecond(t *testing.T) {
	tests := []struct {
		seconds uint64
		nanos   uint32
		want    string
	}{
		{0, 0, "PT0S"},
		{3*86400 + 4*3600, 0, "P3DT4H"},
		{1*86400 + 2*3600 + 3*60 + 4, 120000000, "P1DT2H3M4.12S"},
		{90, 0, "PT1M30S"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			dts := types.DayToSecond{Seconds: tt.seconds, Nanoseconds: tt.nanos}
			tv, _ := gqldb.NewTypedValue(dts)
			s, _ := tv.FormatValue()
			if s != tt.want {
				t.Fatalf("FormatValue: expected %q, got %q", tt.want, s)
			}
			tv2, err := gqldb.NewTypedValueFromString(s, gqldb.PropertyTypeDayToSecond)
			if err != nil {
				t.Fatal(err)
			}
			val, _ := tv2.ToGo()
			dts2 := val.(types.DayToSecond)
			if dts2.Seconds != tt.seconds || dts2.Nanoseconds != tt.nanos {
				t.Fatalf("roundtrip failed: {%d, %d} != {%d, %d}", dts2.Seconds, dts2.Nanoseconds, tt.seconds, tt.nanos)
			}
		})
	}
}

func TestFormatAndParseEmptyString(t *testing.T) {
	tv, err := gqldb.NewTypedValueFromString("", gqldb.PropertyTypeDate)
	if err != nil {
		t.Fatal(err)
	}
	if !tv.IsNull {
		t.Fatal("expected null for empty string")
	}
}

func TestParseLocalDateTimeWithT(t *testing.T) {
	tv, err := gqldb.NewTypedValueFromString("1993-05-06T09:11:02", gqldb.PropertyTypeLocalDatetime)
	if err != nil {
		t.Fatal(err)
	}
	val, _ := tv.ToGo()
	ldt := val.(types.LocalDateTime)
	expected := time.Date(1993, 5, 6, 9, 11, 2, 0, time.UTC)
	if !ldt.Time.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, ldt.Time)
	}
}

// Tests for Z suffix and space-separated offset support

func TestParseLocalDateTimeStripsOffset(t *testing.T) {
	tests := []struct {
		input string
		want  time.Time
	}{
		{"2006-12-12T00:00:00.000Z", time.Date(2006, 12, 12, 0, 0, 0, 0, time.UTC)},
		{"2006-12-12 00:00:00.000 +08:00", time.Date(2006, 12, 12, 0, 0, 0, 0, time.UTC)},
		{"2006-12-12 14:30:00+05:00", time.Date(2006, 12, 12, 14, 30, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tv, err := gqldb.NewTypedValueFromString(tt.input, gqldb.PropertyTypeLocalDatetime)
			if err != nil {
				t.Fatal(err)
			}
			val, _ := tv.ToGo()
			ldt := val.(types.LocalDateTime)
			if !ldt.Time.Equal(tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, ldt.Time)
			}
		})
	}
}

func TestParseLocalTimeStripsOffset(t *testing.T) {
	tests := []struct {
		input string
		want  types.LocalTime
	}{
		{"14:30:00+08:00", types.LocalTime{Hour: 14, Minute: 30}},
		{"14:30:00 +08:00", types.LocalTime{Hour: 14, Minute: 30}},
		{"14:30:00Z", types.LocalTime{Hour: 14, Minute: 30}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tv, err := gqldb.NewTypedValueFromString(tt.input, gqldb.PropertyTypeLocalTime)
			if err != nil {
				t.Fatal(err)
			}
			val, _ := tv.ToGo()
			lt := val.(types.LocalTime)
			if lt.Hour != tt.want.Hour || lt.Minute != tt.want.Minute {
				t.Fatalf("expected %+v, got %+v", tt.want, lt)
			}
		})
	}
}

func TestParseTimestampWithZ(t *testing.T) {
	tv, err := gqldb.NewTypedValueFromString("2006-12-12T00:00:00.000Z", gqldb.PropertyTypeTimestamp)
	if err != nil {
		t.Fatal(err)
	}
	val, _ := tv.ToGo()
	ts := val.(time.Time)
	expected := time.Date(2006, 12, 12, 0, 0, 0, 0, time.UTC)
	if !ts.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, ts)
	}
}

func TestParseTimestampWithSpaceOffset(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Time
	}{
		{"2024-06-15 14:30:00 +08:00", time.Date(2024, 6, 15, 6, 30, 0, 0, time.UTC)},
		{"2024-06-15 14:30:00 -05:00", time.Date(2024, 6, 15, 19, 30, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tv, err := gqldb.NewTypedValueFromString(tt.input, gqldb.PropertyTypeTimestamp)
			if err != nil {
				t.Fatal(err)
			}
			val, _ := tv.ToGo()
			ts := val.(time.Time)
			if !ts.Equal(tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, ts)
			}
		})
	}
}

func TestParseZonedDateTimeWithSpaceOffset(t *testing.T) {
	tv, err := gqldb.NewTypedValueFromString("2024-06-15 14:30:00 +08:00", gqldb.PropertyTypeZonedDatetime)
	if err != nil {
		t.Fatal(err)
	}
	val, _ := tv.ToGo()
	zdt := val.(types.ZonedDateTime)
	if zdt.OffsetMinutes != 480 {
		t.Fatalf("expected offset 480, got %d", zdt.OffsetMinutes)
	}
}

func TestParseZonedDateTimeWithZ(t *testing.T) {
	tv, err := gqldb.NewTypedValueFromString("2024-06-15T14:30:00Z", gqldb.PropertyTypeZonedDatetime)
	if err != nil {
		t.Fatal(err)
	}
	val, _ := tv.ToGo()
	zdt := val.(types.ZonedDateTime)
	if zdt.OffsetMinutes != 0 {
		t.Fatalf("expected offset 0, got %d", zdt.OffsetMinutes)
	}
}
