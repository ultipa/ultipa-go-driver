package types

import (
	"encoding/json"
	"testing"
	"time"
)

// Canonical string form (String) + JSON must both render as
// "YYYY-MM-DD HH:MM:SS[.frac][±HH:MM]" with trailing fractional zeros trimmed.
func TestTemporalString(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"LocalDateTime.ms", LocalDateTime{Time: time.Date(2026, 7, 1, 15, 40, 12, 153000000, time.UTC)}.String(), "2026-07-01 15:40:12.153"},
		{"LocalDateTime.noFrac", LocalDateTime{Time: time.Date(2026, 7, 1, 15, 40, 12, 0, time.UTC)}.String(), "2026-07-01 15:40:12"},
		{"LocalDateTime.nanos", LocalDateTime{Time: time.Date(2026, 7, 1, 15, 40, 12, 152663000, time.UTC)}.String(), "2026-07-01 15:40:12.152663"},
		{"ZonedTime", ZonedTime{Hour: 15, Minute: 40, Second: 12, Nanosecond: 153000000, OffsetMinutes: 480}.String(), "15:40:12.153+08:00"},
		{"ZonedTime.neg", ZonedTime{Hour: 15, Minute: 40, Second: 12, OffsetMinutes: -330}.String(), "15:40:12-05:30"},
		{"Date", GqldbDate{Year: 2026, Month: 7, Day: 1}.String(), "2026-07-01"},
		{"LocalTime", LocalTime{Hour: 9, Minute: 5, Second: 3, Nanosecond: 0}.String(), "09:05:03"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestTemporalMarshalJSON(t *testing.T) {
	b, err := json.Marshal(LocalDateTime{Time: time.Date(2026, 7, 1, 15, 40, 12, 153000000, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `"2026-07-01 15:40:12.153"` {
		t.Errorf("LocalDateTime JSON: got %s, want a bare canonical string", b)
	}
	b2, _ := json.Marshal(GqldbDate{Year: 2026, Month: 7, Day: 1})
	if string(b2) != `"2026-07-01"` {
		t.Errorf("Date JSON: got %s", b2)
	}
}
