package types

import (
	"fmt"
	"time"
)

// Point represents a 2D geographic point.
type Point struct {
	Latitude  float64
	Longitude float64
}

// Point3D represents a 3D point.
type Point3D struct {
	X float64
	Y float64
	Z float64
}

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
type DayToSecond struct {
	Seconds     uint64
	Nanoseconds uint32
}

// Vector represents a float32 vector for embeddings.
type Vector struct {
	Values []float32
}

// Record represents a record/document type.
type Record map[string]interface{}

// Set represents a set (unique, unordered elements).
type Set []interface{}

// GqldbTable represents a table data with columns and rows.
type GqldbTable struct {
	Columns []string
	Rows    [][]interface{}
}
