package test

import (
	"fmt"
	"testing"

	"github.com/ultipa/ultipa-go-driver/v5/sdk/types"
)

// TestPoint3DCreation tests creating Point3D instances
func TestPoint3DCreation(t *testing.T) {
	// Test with NewPoint3D constructor
	p := types.NewPoint3D(1.0, 2.0, 3.0)
	if p.X != 1.0 || p.Y != 2.0 || p.Z != 3.0 {
		t.Errorf("NewPoint3D failed: expected (1.0, 2.0, 3.0), got (%f, %f, %f)", p.X, p.Y, p.Z)
	}

	// Test with struct literal
	point3d := types.Point3D{
		X: 0.0,
		Y: -1.0,
		Z: 5.5,
	}
	if point3d.X != 0.0 || point3d.Y != -1.0 || point3d.Z != 5.5 {
		t.Errorf("Point3D struct literal failed: expected (0.0, -1.0, 5.5), got (%f, %f, %f)", point3d.X, point3d.Y, point3d.Z)
	}
}

// TestPoint3DString tests Point3D string representation
func TestPoint3DString(t *testing.T) {
	testCases := []struct {
		point3d  *types.Point3D
		expected string
	}{
		{types.NewPoint3D(1.0, 2.0, 3.0), "POINT3D(1.000000 2.000000 3.000000)"},
		{types.NewPoint3D(0.0, 0.0, 0.0), "POINT3D(0.000000 0.000000 0.000000)"},
		{types.NewPoint3D(-10.5, 20.3, -30.7), "POINT3D(-10.500000 20.300000 -30.700000)"},
		{types.NewPoint3D(100.123456, -200.654321, 300.999), "POINT3D(100.123456 -200.654321 300.999000)"},
	}

	for i, tc := range testCases {
		result := tc.point3d.String()
		if result != tc.expected {
			t.Errorf("Test case %d: expected %s, got %s", i, tc.expected, result)
		}
	}
}

// TestPoint3DFromStr tests parsing Point3D from string
func TestPoint3DFromStr(t *testing.T) {
	testCases := []string{
		"POINT3D(1.0 2.0 3.0)",
		"Point3D(9 22.2 -5.5)",
		"Point3D(-10.1 22.2 0.0)",
		"Point3D(-80 -80 -80)",
		"Point3D(0.00 0.00 0.00)",
		"point3d(1.00 -2.00 3.50)",
		"POINT3D(100.123 -200.456 300.789)",
	}

	for i, str := range testCases {
		point3d, err := types.Point3DFromStr(str)
		if err != nil {
			t.Errorf("Test case %d: failed to parse %s: %v", i, str, err)
			continue
		}
		fmt.Printf("Parsed %s -> %v\n", str, point3d)
	}
}

// TestPoint3DFromStrInvalid tests parsing invalid Point3D strings
func TestPoint3DFromStrInvalid(t *testing.T) {
	invalidCases := []string{
		"POINT3D(1.0 2.0)",        // Missing Z coordinate
		"POINT3D(1.0)",            // Only one coordinate
		"POINT(1.0 2.0 3.0)",      // Wrong prefix (POINT instead of POINT3D)
		"POINT3D(a b c)",          // Non-numeric values
		"POINT3D 1.0 2.0 3.0",     // Missing parentheses
		"",                        // Empty string
		"invalid",                 // Invalid format
	}

	for i, str := range invalidCases {
		point3d, err := types.Point3DFromStr(str)
		if err == nil {
			t.Errorf("Test case %d: expected error for invalid string %s, but got point3d: %v", i, str, point3d)
		}
	}
}

// TestPoint3DRoundTrip tests converting Point3D to string and back
func TestPoint3DRoundTrip(t *testing.T) {
	originalPoints := []*types.Point3D{
		types.NewPoint3D(1.0, 2.0, 3.0),
		types.NewPoint3D(-5.5, 10.25, -15.75),
		types.NewPoint3D(0.0, 0.0, 0.0),
		types.NewPoint3D(100.123, -200.456, 300.789),
	}

	for i, original := range originalPoints {
		// Convert to string
		str := original.String()

		// Parse back from string
		parsed, err := types.Point3DFromStr(str)
		if err != nil {
			t.Errorf("Test case %d: failed to parse %s: %v", i, str, err)
			continue
		}

		// Compare coordinates (with tolerance for float precision)
		tolerance := 0.000001
		if !floatEqual(original.X, parsed.X, tolerance) ||
			!floatEqual(original.Y, parsed.Y, tolerance) ||
			!floatEqual(original.Z, parsed.Z, tolerance) {
			t.Errorf("Test case %d: round trip failed. Original: (%f, %f, %f), Parsed: (%f, %f, %f)",
				i, original.X, original.Y, original.Z, parsed.X, parsed.Y, parsed.Z)
		}
	}
}

// floatEqual compares two float64 values with tolerance
func floatEqual(a, b, tolerance float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < tolerance
}

// TestPoint3DVsPoint tests that Point3D and Point are separate types
func TestPoint3DVsPoint(t *testing.T) {
	// Create a 2D point
	point2d := types.NewPoint(10.0, 20.0)
	point2dStr := point2d.String()

	// Create a 3D point
	point3d := types.NewPoint3D(10.0, 20.0, 30.0)
	point3dStr := point3d.String()

	// Verify they have different string representations
	if point2dStr == point3dStr {
		t.Error("Point2D and Point3D should have different string representations")
	}

	// Verify Point2D cannot be parsed as Point3D
	_, err := types.Point3DFromStr(point2dStr)
	if err == nil {
		t.Error("Expected error when parsing Point2D string as Point3D")
	}

	// Verify Point3D cannot be parsed as Point2D
	_, err = types.PointFromStr(point3dStr)
	if err == nil {
		t.Error("Expected error when parsing Point3D string as Point2D")
	}
}