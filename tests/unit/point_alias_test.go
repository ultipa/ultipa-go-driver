//go:build unit

package unit

import (
	"testing"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// Unit tests for Point / Point3D alias accessor methods (Python-parity
// additions: Point.X/Y, Point3D.Longitude/Latitude/Height).

func TestPoint_XAliasReturnsLongitude(t *testing.T) {
	p := gqldb.Point{Latitude: 40.7128, Longitude: -74.006} // NYC
	if got, want := p.X(), p.Longitude; got != want {
		t.Errorf("Point.X() = %v, want Longitude=%v", got, want)
	}
	if got, want := p.Y(), p.Latitude; got != want {
		t.Errorf("Point.Y() = %v, want Latitude=%v", got, want)
	}
}

func TestPoint_AliasesRoundTripIdentically(t *testing.T) {
	cases := []gqldb.Point{
		{Latitude: 0, Longitude: 0},
		{Latitude: 90, Longitude: 180},
		{Latitude: -90, Longitude: -180},
	}
	for i, p := range cases {
		if p.X() != p.Longitude {
			t.Errorf("case %d: X() != Longitude", i)
		}
		if p.Y() != p.Latitude {
			t.Errorf("case %d: Y() != Latitude", i)
		}
	}
}

func TestPoint3D_GeographicAliasesForwardToXYZ(t *testing.T) {
	p := gqldb.Point3D{X: 1.5, Y: 2.5, Z: 3.5}
	if p.Longitude() != p.X {
		t.Errorf("Point3D.Longitude() = %v, want X=%v", p.Longitude(), p.X)
	}
	if p.Latitude() != p.Y {
		t.Errorf("Point3D.Latitude() = %v, want Y=%v", p.Latitude(), p.Y)
	}
	if p.Height() != p.Z {
		t.Errorf("Point3D.Height() = %v, want Z=%v", p.Height(), p.Z)
	}
}

func TestPoint3D_AliasesRoundTripIdentically(t *testing.T) {
	cases := []gqldb.Point3D{
		{X: 0, Y: 0, Z: 0},
		{X: 1e10, Y: -1e10, Z: 1e-5},
		{X: -1.5, Y: -2.5, Z: -3.5},
	}
	for i, p := range cases {
		if p.Longitude() != p.X {
			t.Errorf("case %d: Longitude() != X", i)
		}
		if p.Latitude() != p.Y {
			t.Errorf("case %d: Latitude() != Y", i)
		}
		if p.Height() != p.Z {
			t.Errorf("case %d: Height() != Z", i)
		}
	}
}
