package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// Point represents a 2D geographic point (WGS-84 — server enforces
// longitude ∈ [-180, 180], latitude ∈ [-90, 90]).
//
// Primary fields are Latitude and Longitude. For symmetry with Point3D
// and cartesian-style code, X() (= Longitude) and Y() (= Latitude) are
// exposed as accessor aliases.
type Point struct {
	Latitude  float64
	Longitude float64
	// SRID is the spatial reference system id. 0 means "unset" — the encoder
	// fills DefaultPoint2DSRID (WGS-84). Decoded values from servers that
	// don't report an SRID (legacy 16-byte form) also fall back to the default.
	SRID int32
}

// Default spatial reference system ids, matching the server converter
// (server/converter/from_proto.go).
const (
	DefaultPoint2DSRID = 4326 // WGS-84 (geographic)
	DefaultPoint3DSRID = 0    // cartesian, no CRS
)

// X is an alias for Longitude — matches the Point3D x/y/z convention.
func (p Point) X() float64 { return p.Longitude }

// Y is an alias for Latitude — matches the Point3D x/y/z convention.
func (p Point) Y() float64 { return p.Latitude }

// Point3D represents a 3D point (cartesian semantics — server does NOT
// enforce geographic bounds, so X and Y may legitimately exceed lon/lat ranges).
//
// Primary fields are X, Y, Z. For symmetry with the geographic Point,
// Longitude() (= X), Latitude() (= Y), and Height() (= Z) are exposed
// as accessor aliases for geographic-style readers.
type Point3D struct {
	X float64
	Y float64
	Z float64
	// SRID is the spatial reference system id. 0 means "unset" — the encoder
	// fills DefaultPoint3DSRID (0, cartesian). See Point.SRID.
	SRID int32
}

// Longitude is an alias for X — for geographic-style readers.
func (p Point3D) Longitude() float64 { return p.X }

// Latitude is an alias for Y — for geographic-style readers.
func (p Point3D) Latitude() float64 { return p.Y }

// Height is an alias for Z — for geographic-3D (lon/lat/altitude) readers.
func (p Point3D) Height() float64 { return p.Z }

// Decimal represents a high-precision decimal number.
type Decimal struct {
	Value string // String representation, e.g., "123.456"
}

// Datetime represents a datetime value (deprecated, use Timestamp).
// This type exists for compatibility with PROPERTY_TYPE_DATETIME (8).
type Datetime struct {
	Time time.Time
}

// LocalDateTime represents a local date-time without timezone.
type LocalDateTime struct {
	Time time.Time
}

// String returns the canonical form, e.g. "2026-07-01 15:40:12.153"
// (matches TypedValue.FormatValue(); trailing fractional zeros trimmed).
func (ldt LocalDateTime) String() string {
	t := ldt.Time.UTC()
	return formatDateTime(t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond())
}

// String returns the canonical form with offset, e.g. "2026-07-01 15:40:12.153+08:00".
func (zdt ZonedDateTime) String() string {
	t := zdt.Time // FixedZone wall-clock
	return formatDateTime(t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond()) +
		formatOffset(zdt.OffsetMinutes)
}

// String returns the date as "2026-07-01".
func (d GqldbDate) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

// String returns the local time, e.g. "15:40:12.153".
func (lt LocalTime) String() string {
	return formatTime(lt.Hour, lt.Minute, lt.Second, lt.Nanosecond)
}

// String returns the time with offset, e.g. "15:40:12.153+08:00".
func (zt ZonedTime) String() string {
	return formatTime(zt.Hour, zt.Minute, zt.Second, zt.Nanosecond) + formatOffset(zt.OffsetMinutes)
}

// MarshalJSON renders each temporal type as its canonical string (not a struct),
// e.g. "2026-07-01 15:40:12.153", matching String().
func (ldt LocalDateTime) MarshalJSON() ([]byte, error) { return json.Marshal(ldt.String()) }
func (zdt ZonedDateTime) MarshalJSON() ([]byte, error) { return json.Marshal(zdt.String()) }
func (d GqldbDate) MarshalJSON() ([]byte, error)       { return json.Marshal(d.String()) }
func (lt LocalTime) MarshalJSON() ([]byte, error)      { return json.Marshal(lt.String()) }
func (zt ZonedTime) MarshalJSON() ([]byte, error)      { return json.Marshal(zt.String()) }

// ZonedDateTime represents a date-time with timezone offset.
type ZonedDateTime struct {
	Time          time.Time
	OffsetMinutes int16 // UTC offset in minutes
}

// GqldbDate represents a date without time.
//
// Year is signed (int16) so BCE years (e.g. -44) round-trip correctly.
// The wire format encodes/decodes via uint16 bit pattern (little-endian),
// matching Python's `<h` struct format and Java's signed short.
type GqldbDate struct {
	Year  int16
	Month uint8 // 1-12
	Day   uint8 // 1-31
}

// LocalTime represents a local time without timezone.
type LocalTime struct {
	Hour       uint8
	Minute     uint8
	Second     uint8
	Nanosecond uint32
}

// ZonedTime represents a time with timezone offset.
type ZonedTime struct {
	Hour          uint8
	Minute        uint8
	Second        uint8
	Nanosecond    uint32
	OffsetMinutes int16
}

// YearToMonth represents a year-month interval.
type YearToMonth struct {
	Months int32 // Total months (can be negative)
}

// DayToSecond represents a day-second interval.
//
// Seconds is signed (int64) so negative durations round-trip correctly.
// The wire format encodes/decodes via uint64 bit pattern (little-endian),
// which is identical to int64 on two's-complement hardware.
type DayToSecond struct {
	Seconds     int64
	Nanoseconds uint32
}

// Vector represents a float32 vector for embeddings.
//
// Iterate the elements via `for _, x := range vec.Values`. Len() returns
// the dimension count — mirrors the Python driver's len(vec) ergonomic.
type Vector struct {
	Values []float32
}

// Len returns the number of dimensions in the vector.
func (v Vector) Len() int { return len(v.Values) }

// Record represents a record/document type.
type Record map[string]interface{}

// Set represents a set (unique, unordered elements).
type Set []interface{}

// GqldbTable represents a table data with columns and rows.
type GqldbTable struct {
	Columns []string
	Rows    [][]interface{}
}
