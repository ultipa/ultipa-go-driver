package types

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Point3D represents a 3D point with X, Y, Z coordinates
type Point3D struct {
	X float64 // X coordinate
	Y float64 // Y coordinate
	Z float64 // Z coordinate
}

// NewPoint3D creates a new Point3D with given coordinates
func NewPoint3D(x, y, z float64) *Point3D {
	return &Point3D{
		X: x,
		Y: y,
		Z: z,
	}
}

// Point3DRegularExpress regular expression for parsing a string to Point3D
const Point3DRegularExpress = "(?i)Point3D\\((?P<x>(?:-?\\d+)(?:\\.\\d+)?)\\s+(?P<y>(?:-?\\d+)(?:\\.\\d+)?)\\s+(?P<z>(?:-?\\d+)(?:\\.\\d+)?)\\)"

// Point3DFromStr parses a string to Point3D
// Expected format: "POINT3D(x y z)" where x, y, z are floating point numbers
func Point3DFromStr(point3DStr string) (*Point3D, error) {
	point3DMatcher := regexp.MustCompile(Point3DRegularExpress)
	result := point3DMatcher.FindStringSubmatch(strings.TrimSpace(point3DStr))
	if len(result) == 0 {
		return nil, errors.New(fmt.Sprintf("%v is not a valid point3d pattern string", point3DStr))
	}

	// Parse X coordinate
	xIdx := point3DMatcher.SubexpIndex("x")
	xStr := result[xIdx]
	x, err := strconv.ParseFloat(xStr, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse x coordinate: %w", err)
	}

	// Parse Y coordinate
	yIdx := point3DMatcher.SubexpIndex("y")
	yStr := result[yIdx]
	y, err := strconv.ParseFloat(yStr, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse y coordinate: %w", err)
	}

	// Parse Z coordinate
	zIdx := point3DMatcher.SubexpIndex("z")
	zStr := result[zIdx]
	z, err := strconv.ParseFloat(zStr, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse z coordinate: %w", err)
	}

	return NewPoint3D(x, y, z), nil
}

// String returns the string representation of Point3D
func (p *Point3D) String() string {
	return fmt.Sprintf(`POINT3D(%f %f %f)`, p.X, p.Y, p.Z)
}

// MarshalJSON implements json.Marshaler interface
// Returns the Point3D in UQL format: "POINT3D(x y z)"
func (p *Point3D) MarshalJSON() ([]byte, error) {
	if p == nil {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf(`"%s"`, p.String())), nil
}