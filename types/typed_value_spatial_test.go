package types

import (
	"reflect"
	"testing"
)

// Covers the from-string parse branches added for Point / Point3D / Vector /
// Blob and their round-trip with FormatValue.

func mustParse(t *testing.T, s string, pt PropertyType) *TypedValue {
	t.Helper()
	tv, err := NewTypedValueFromString(s, pt)
	if err != nil {
		t.Fatalf("NewTypedValueFromString(%q, %d) error: %v", s, pt, err)
	}
	return tv
}

func TestParsePoint(t *testing.T) {
	// A parsed point has no SRID; the encode/decode round-trip (via ToGo)
	// fills the default 2D SRID, so the decoded value carries DefaultPoint2DSRID.
	want := Point{Latitude: 30.5, Longitude: 114.3, SRID: DefaultPoint2DSRID}
	for _, s := range []string{
		"point({latitude: 30.5, longitude: 114.3})",
		"point({longitude: 114.3, latitude: 30.5})", // key order independent
		"30.5,114.3",                                // lenient lat,lon
		"(30.5,114.3)",
	} {
		tv := mustParse(t, s, PropertyTypePoint)
		got, err := tv.ToGo()
		if err != nil {
			t.Fatalf("ToGo(%q): %v", s, err)
		}
		if got.(Point) != want {
			t.Errorf("parse %q = %+v, want %+v", s, got, want)
		}
	}
	// out-of-range must error
	if _, err := NewTypedValueFromString("100,0", PropertyTypePoint); err == nil {
		t.Error("latitude 100 must be rejected")
	}
	if _, err := NewTypedValueFromString("0,200", PropertyTypePoint); err == nil {
		t.Error("longitude 200 must be rejected")
	}
}

func TestParsePoint3D(t *testing.T) {
	want := Point3D{X: 1, Y: 2, Z: 3}
	for _, s := range []string{
		"point({x: 1, y: 2, z: 3})",
		"point({z: 3, x: 1, y: 2})",
		"1,2,3",
		"(1,2,3)",
	} {
		tv := mustParse(t, s, PropertyTypePoint3D)
		got, _ := tv.ToGo()
		if got.(Point3D) != want {
			t.Errorf("parse %q = %+v, want %+v", s, got, want)
		}
	}
	if _, err := NewTypedValueFromString("1,2", PropertyTypePoint3D); err == nil {
		t.Error("3D point with 2 values must be rejected")
	}
}

func TestParseVector(t *testing.T) {
	cases := map[string][]float32{
		"[0.1,0.2,0.3]": {0.1, 0.2, 0.3},
		"0.1,0.2,0.3":   {0.1, 0.2, 0.3},
		"[]":            {},
	}
	for s, want := range cases {
		tv := mustParse(t, s, PropertyTypeVector)
		got, _ := tv.ToGo()
		if !reflect.DeepEqual(got.(Vector).Values, want) {
			t.Errorf("parse %q = %v, want %v", s, got.(Vector).Values, want)
		}
	}
	if _, err := NewTypedValueFromString("[1,abc]", PropertyTypeVector); err == nil {
		t.Error("non-numeric vector element must be rejected")
	}
}

func TestParseBlob(t *testing.T) {
	// base64 of "hi" = "aGk="
	tv := mustParse(t, "aGk=", PropertyTypeBlob)
	got, _ := tv.ToGo()
	if string(got.([]byte)) != "hi" {
		t.Errorf("base64 blob = %q, want %q", got, "hi")
	}
	// hex form
	tv = mustParse(t, "0x6869", PropertyTypeBlob)
	got, _ = tv.ToGo()
	if string(got.([]byte)) != "hi" {
		t.Errorf("hex blob = %q, want %q", got, "hi")
	}
	if _, err := NewTypedValueFromString("not!base64!", PropertyTypeBlob); err == nil {
		t.Error("invalid base64 must be rejected")
	}
}

// Round-trip: FormatValue output must parse back to the same value.
func TestSpatialRoundTrip(t *testing.T) {
	vals := []interface{}{
		Point{Latitude: 30.5, Longitude: 114.3},
		Point3D{X: 1.5, Y: -2.5, Z: 3},
		Vector{Values: []float32{0.1, 0.2, 0.3}},
		[]byte("hello"),
	}
	for _, v := range vals {
		tv, err := NewTypedValue(v)
		if err != nil {
			t.Fatalf("NewTypedValue(%v): %v", v, err)
		}
		s, err := tv.FormatValue()
		if err != nil {
			t.Fatalf("FormatValue(%v): %v", v, err)
		}
		tv2 := mustParse(t, s, tv.Type)
		got, _ := tv2.ToGo()
		want, _ := tv.ToGo()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("round-trip %v: format=%q reparsed=%v, want %v", v, s, got, want)
		}
	}
}
