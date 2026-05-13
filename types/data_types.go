package types

import (
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
}

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

// String returns the local date-time formatted without timezone (e.g., "2024-06-15T14:30:00").
func (ldt LocalDateTime) String() string {
	t := ldt.Time.UTC()
	if t.Nanosecond() != 0 {
		return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d.%09d",
			t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond())
	}
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}

// ZonedDateTime represents a date-time with timezone offset.
type ZonedDateTime struct {
	Time          time.Time
	OffsetMinutes int16 // UTC offset in minutes
}

// GqldbDate represents a date without time.
type GqldbDate struct {
	Year  uint16
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
