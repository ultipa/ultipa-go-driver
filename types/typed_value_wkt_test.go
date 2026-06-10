package types

import "testing"

// WKT POINT parsing: OGC/PostGIS order POINT(<longitude> <latitude>) for the 2D
// geographic Point (lon first); cartesian x,y,z for Point3D.

func TestParseWKTPoint2D_OGCOrder(t *testing.T) {
	// POINT(lon lat) — longitude first.
	cases := map[string]struct{ lat, lon float64 }{
		"POINT(114.3 30.5)":   {30.5, 114.3},   // Wuhan
		"POINT(-0.12 51.05)":  {51.05, -0.12},  // London (standard WKT)
		"POINT(116.4 39.9)":   {39.9, 116.4},   // Beijing — lon 116>90 must NOT error
		"point(1 2)":          {2, 1},          // case-insensitive
		"POINT(1,2)":          {2, 1},          // comma separator
		"  POINT( 10  20 )  ": {20, 10},        // whitespace tolerant
	}
	for s, want := range cases {
		p, err := parsePoint(s)
		if err != nil {
			t.Errorf("parsePoint(%q) unexpected error: %v", s, err)
			continue
		}
		if p.Latitude != want.lat || p.Longitude != want.lon {
			t.Errorf("parsePoint(%q) = lat=%v lon=%v, want lat=%v lon=%v",
				s, p.Latitude, p.Longitude, want.lat, want.lon)
		}
	}
}

func TestParseWKTPoint2D_Errors(t *testing.T) {
	bad := []string{
		"POINT(1 abc)",   // non-numeric
		"POINT(1 2 3)",   // 3 values for a 2D point
		"POINT()",        // empty
		"POINT(200 30)",  // longitude 200 out of [-180,180]
		"POINT(30 95)",   // latitude 95 out of [-90,90]
	}
	for _, s := range bad {
		if _, err := parsePoint(s); err == nil {
			t.Errorf("parsePoint(%q) expected error, got nil", s)
		}
	}
}

func TestParseWKTPoint3D_Cartesian(t *testing.T) {
	// 3D is cartesian x,y,z — positional, no lon/lat swap.
	for _, s := range []string{"POINT(1 2 3)", "POINT Z(1 2 3)", "point z(1,2,3)"} {
		p, err := parsePoint3D(s)
		if err != nil {
			t.Errorf("parsePoint3D(%q) unexpected error: %v", s, err)
			continue
		}
		if p.X != 1 || p.Y != 2 || p.Z != 3 {
			t.Errorf("parsePoint3D(%q) = %+v, want {1 2 3}", s, p)
		}
	}
	if _, err := parsePoint3D("POINT(1 2)"); err == nil {
		t.Error("parsePoint3D(\"POINT(1 2)\") expected error (needs 3 values)")
	}
}
