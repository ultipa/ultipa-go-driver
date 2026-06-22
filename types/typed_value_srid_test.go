package types

import (
	"encoding/binary"
	"math"
	"testing"
)

// SRID encode/decode: explicit SRID round-trips; unset (0) fills the default;
// legacy 16/24-byte payloads (no SRID) decode to the default SRID.

func TestSRIDRoundTrip(t *testing.T) {
	// Explicit non-default SRID must survive encode -> decode.
	p := Point{Latitude: 30.5, Longitude: 114.3, SRID: 3857}
	tv, err := NewTypedValue(p)
	if err != nil {
		t.Fatalf("NewTypedValue(Point): %v", err)
	}
	if len(tv.Data) != 20 {
		t.Errorf("Point payload = %d bytes, want 20", len(tv.Data))
	}
	got, _ := tv.ToGo()
	if g := got.(Point); g.SRID != 3857 || g.Latitude != 30.5 || g.Longitude != 114.3 {
		t.Errorf("Point round-trip = %+v, want SRID 3857 / 30.5 / 114.3", g)
	}

	p3 := Point3D{X: 1, Y: 2, Z: 3, SRID: 7}
	tv3, _ := NewTypedValue(p3)
	if len(tv3.Data) != 28 {
		t.Errorf("Point3D payload = %d bytes, want 28", len(tv3.Data))
	}
	got3, _ := tv3.ToGo()
	if g := got3.(Point3D); g.SRID != 7 || g.X != 1 || g.Y != 2 || g.Z != 3 {
		t.Errorf("Point3D round-trip = %+v, want SRID 7 / 1 2 3", g)
	}
}

func TestSRIDUnsetFillsDefault(t *testing.T) {
	tv, _ := NewTypedValue(Point{Latitude: 1, Longitude: 2}) // SRID 0 -> default
	got, _ := tv.ToGo()
	if g := got.(Point); g.SRID != DefaultPoint2DSRID {
		t.Errorf("unset 2D SRID = %d, want %d", g.SRID, DefaultPoint2DSRID)
	}
	tv3, _ := NewTypedValue(Point3D{X: 1, Y: 2, Z: 3}) // SRID 0 -> default (0)
	got3, _ := tv3.ToGo()
	if g := got3.(Point3D); g.SRID != DefaultPoint3DSRID {
		t.Errorf("unset 3D SRID = %d, want %d", g.SRID, DefaultPoint3DSRID)
	}
}

func TestSRIDLegacyPayloadCompat(t *testing.T) {
	// Legacy 16-byte Point payload (no trailing SRID) -> default 2D SRID.
	legacy2D := make([]byte, 16)
	binary.LittleEndian.PutUint64(legacy2D[0:8], math.Float64bits(114.3)) // lon
	binary.LittleEndian.PutUint64(legacy2D[8:16], math.Float64bits(30.5)) // lat
	g2, _ := (&TypedValue{Type: PropertyTypePoint, Data: legacy2D}).ToGo()
	if g := g2.(Point); g.SRID != DefaultPoint2DSRID || g.Latitude != 30.5 || g.Longitude != 114.3 {
		t.Errorf("legacy 16-byte Point = %+v, want SRID %d / 30.5 / 114.3", g, DefaultPoint2DSRID)
	}

	// Legacy 24-byte Point3D payload -> default 3D SRID.
	legacy3D := make([]byte, 24)
	binary.LittleEndian.PutUint64(legacy3D[0:8], math.Float64bits(1))
	binary.LittleEndian.PutUint64(legacy3D[8:16], math.Float64bits(2))
	binary.LittleEndian.PutUint64(legacy3D[16:24], math.Float64bits(3))
	g3, _ := (&TypedValue{Type: PropertyTypePoint3D, Data: legacy3D}).ToGo()
	if g := g3.(Point3D); g.SRID != DefaultPoint3DSRID || g.X != 1 || g.Y != 2 || g.Z != 3 {
		t.Errorf("legacy 24-byte Point3D = %+v, want SRID %d / 1 2 3", g, DefaultPoint3DSRID)
	}
}
