//go:build unit

package unit

import (
	"math"
	"testing"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// Regression tests for DAY_TO_SECOND signed seconds round-trip (#16 —
// fix for the unsigned-seconds bug that mangled negative durations).

func roundTripDts(t *testing.T, in gqldb.DayToSecond) gqldb.DayToSecond {
	t.Helper()
	tv, err := gqldb.NewTypedValue(in)
	if err != nil {
		t.Fatalf("NewTypedValue(%v) failed: %v", in, err)
	}
	if tv.Type != gqldb.PropertyTypeDayToSecond {
		t.Fatalf("expected PROPERTY_TYPE_DAY_TO_SECOND, got %v", tv.Type)
	}
	out, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo failed: %v", err)
	}
	dts, ok := out.(gqldb.DayToSecond)
	if !ok {
		t.Fatalf("expected DayToSecond, got %T", out)
	}
	return dts
}

func TestDayToSecond_Positive_RoundTrips(t *testing.T) {
	in := gqldb.DayToSecond{Seconds: 86400, Nanoseconds: 0}
	out := roundTripDts(t, in)
	if out.Seconds != 86400 || out.Nanoseconds != 0 {
		t.Errorf("got %+v, want %+v", out, in)
	}
}

func TestDayToSecond_Negative_RoundTripsAsSigned(t *testing.T) {
	in := gqldb.DayToSecond{Seconds: -3600, Nanoseconds: 0}
	out := roundTripDts(t, in)
	if out.Seconds != -3600 {
		t.Errorf("negative round-trip lost: got %d, want -3600", out.Seconds)
	}
}

func TestDayToSecond_MinInt64_RoundTrips(t *testing.T) {
	in := gqldb.DayToSecond{Seconds: math.MinInt64, Nanoseconds: 0}
	out := roundTripDts(t, in)
	if out.Seconds != math.MinInt64 {
		t.Errorf("MinInt64 round-trip lost: got %d, want %d", out.Seconds, int64(math.MinInt64))
	}
}
