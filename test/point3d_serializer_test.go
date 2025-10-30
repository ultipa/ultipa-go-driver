package test

import (
	"encoding/binary"
	"math"
	"testing"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/types"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/utils"
)

// TestAsPoint3DString tests deserializing bytes to Point3D string
func TestAsPoint3DString(t *testing.T) {
	// Create 24 bytes (3 × 8 bytes for X, Y, Z)
	bytes := make([]byte, 24)

	// X = 1.0
	binary.BigEndian.PutUint64(bytes[0:8], math.Float64bits(1.0))
	// Y = 2.0
	binary.BigEndian.PutUint64(bytes[8:16], math.Float64bits(2.0))
	// Z = 3.0
	binary.BigEndian.PutUint64(bytes[16:24], math.Float64bits(3.0))

	str, err := utils.AsPoint3DString(bytes)
	if err != nil {
		t.Fatalf("AsPoint3DString failed: %v", err)
	}

	expected := "POINT3D(1.000000 2.000000 3.000000)"
	if str != expected {
		t.Errorf("Expected %s, got %s", expected, str)
	}
}

// TestAsPoint3DStringInvalidLength tests error handling for invalid byte length
func TestAsPoint3DStringInvalidLength(t *testing.T) {
	testCases := []int{0, 8, 16, 23, 25, 32}

	for _, length := range testCases {
		bytes := make([]byte, length)
		_, err := utils.AsPoint3DString(bytes)
		if err == nil {
			t.Errorf("Expected error for byte length %d, but got none", length)
		}
	}
}

// TestAsPoint3DStringNaN tests handling of NaN values
func TestAsPoint3DStringNaN(t *testing.T) {
	bytes := make([]byte, 24)

	// X = NaN, Y = 2.0, Z = 3.0
	binary.BigEndian.PutUint64(bytes[0:8], math.Float64bits(math.NaN()))
	binary.BigEndian.PutUint64(bytes[8:16], math.Float64bits(2.0))
	binary.BigEndian.PutUint64(bytes[16:24], math.Float64bits(3.0))

	str, err := utils.AsPoint3DString(bytes)
	if err != nil {
		t.Fatalf("AsPoint3DString failed: %v", err)
	}

	// Should return empty string for NaN
	if str != "" {
		t.Errorf("Expected empty string for NaN, got %s", str)
	}
}

// TestConvertBytesToInterfacePoint3D tests converting bytes to Point3D interface
func TestConvertBytesToInterfacePoint3D(t *testing.T) {
	// Create 24 bytes
	bytes := make([]byte, 24)

	// X = 10.5, Y = -20.3, Z = 30.7
	binary.BigEndian.PutUint64(bytes[0:8], math.Float64bits(10.5))
	binary.BigEndian.PutUint64(bytes[8:16], math.Float64bits(-20.3))
	binary.BigEndian.PutUint64(bytes[16:24], math.Float64bits(30.7))

	result, err := utils.ConvertBytesToInterface(bytes, ultipa.PropertyType_POINT3D, nil)
	if err != nil {
		t.Fatalf("ConvertBytesToInterface failed: %v", err)
	}

	point3d, ok := result.(*types.Point3D)
	if !ok {
		t.Fatalf("Result is not *types.Point3D, got %T", result)
	}

	tolerance := 0.000001
	if !floatEqualWithTolerance(point3d.X, 10.5, tolerance) ||
		!floatEqualWithTolerance(point3d.Y, -20.3, tolerance) ||
		!floatEqualWithTolerance(point3d.Z, 30.7, tolerance) {
		t.Errorf("Expected Point3D(10.5, -20.3, 30.7), got Point3D(%f, %f, %f)",
			point3d.X, point3d.Y, point3d.Z)
	}
}

// TestConvertInterfaceToBytesSafePoint3D tests serializing Point3D to bytes
func TestConvertInterfaceToBytesSafePoint3D(t *testing.T) {
	testCases := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{
			name:     "Point3D struct",
			value:    types.Point3D{X: 1.0, Y: 2.0, Z: 3.0},
			expected: "POINT3D(1.000000 2.000000 3.000000)",
		},
		{
			name:     "Point3D pointer",
			value:    types.NewPoint3D(5.5, -10.3, 20.7),
			expected: "POINT3D(5.500000 -10.300000 20.700000)",
		},
		{
			name:     "Point3D string",
			value:    "POINT3D(100.0 -200.0 300.0)",
			expected: "POINT3D(100.000000 -200.000000 300.000000)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bytes, err := utils.ConvertInterfaceToBytesSafe(tc.value, ultipa.PropertyType_POINT3D, nil, nil)
			if err != nil {
				t.Fatalf("ConvertInterfaceToBytesSafe failed: %v", err)
			}

			result := string(bytes)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

// TestConvertInterfaceToBytesSafePoint3DInvalidString tests error handling for invalid string
func TestConvertInterfaceToBytesSafePoint3DInvalidString(t *testing.T) {
	invalidString := "POINT(1.0 2.0)"  // Wrong format (2D instead of 3D)

	_, err := utils.ConvertInterfaceToBytesSafe(invalidString, ultipa.PropertyType_POINT3D, nil, nil)
	if err == nil {
		t.Error("Expected error for invalid Point3D string, but got none")
	}
}

// TestPoint3DRoundTripThroughBytes tests full serialization/deserialization cycle
func TestPoint3DRoundTripThroughBytes(t *testing.T) {
	originalPoints := []*types.Point3D{
		types.NewPoint3D(1.0, 2.0, 3.0),
		types.NewPoint3D(-5.5, 10.25, -15.75),
		types.NewPoint3D(0.0, 0.0, 0.0),
		types.NewPoint3D(100.123, -200.456, 300.789),
	}

	for i, original := range originalPoints {
		// Serialize to bytes
		bytes, err := utils.ConvertInterfaceToBytesSafe(original, ultipa.PropertyType_POINT3D, nil, nil)
		if err != nil {
			t.Fatalf("Test case %d: serialization failed: %v", i, err)
		}

		// Deserialize back to interface
		result, err := utils.ConvertBytesToInterface(bytes, ultipa.PropertyType_POINT3D, nil)
		if err != nil {
			t.Fatalf("Test case %d: deserialization failed: %v", i, err)
		}

		parsed, ok := result.(*types.Point3D)
		if !ok {
			t.Fatalf("Test case %d: result is not *types.Point3D, got %T", i, result)
		}

		// Compare coordinates
		tolerance := 0.000001
		if !floatEqualWithTolerance(original.X, parsed.X, tolerance) ||
			!floatEqualWithTolerance(original.Y, parsed.Y, tolerance) ||
			!floatEqualWithTolerance(original.Z, parsed.Z, tolerance) {
			t.Errorf("Test case %d: round trip failed. Original: (%f, %f, %f), Parsed: (%f, %f, %f)",
				i, original.X, original.Y, original.Z, parsed.X, parsed.Y, parsed.Z)
		}
	}
}

// floatEqualWithTolerance compares two float64 values with tolerance
func floatEqualWithTolerance(a, b, tolerance float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < tolerance
}
