package types

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// point represents of a geographic Point.
type Point struct {
	//Geographic latitude:-90 ~ 90
	Latitude float64
	// Geographic longitude:-180 ~ 180
	Longitude float64
}

func NewPoint(Latitude float64, Longitude float64) *Point {
	return &Point{
		Latitude:  Latitude,
		Longitude: Longitude,
	}
}

// PointRegularExpress regular expression for parse a string  to point
const PointRegularExpress = "(?i)Point\\((?P<latitude>(?:-?\\d+)(?:\\.\\d+)?)\\s+(?P<longitude>(?:-?\\d+)(?:\\.\\d+)?)\\)"

// PointFromStr parse a string to Point
func PointFromStr(pointStr string) (*Point, error) {
	pointMatcher := regexp.MustCompile(PointRegularExpress)
	result := pointMatcher.FindStringSubmatch(strings.TrimSpace(pointStr))
	if len(result) == 0 {
		return nil, errors.New(fmt.Sprintf("%v is not a valid point pattern string", pointStr))
	}
	latitudeIdx := pointMatcher.SubexpIndex("latitude")
	latitudeStr := result[latitudeIdx]
	latitude, err := strconv.ParseFloat(latitudeStr, 64)
	if err != nil {
		return nil, err
	}
	longitudeIdx := pointMatcher.SubexpIndex("longitude")
	longitudeStr := result[longitudeIdx]
	longitude, err := strconv.ParseFloat(longitudeStr, 64)
	if err != nil {
		return nil, err
	}
	return NewPoint(latitude, longitude), nil
}

func (p *Point) String() string {
	return fmt.Sprintf(`POINT(%f %f)`, p.Latitude, p.Longitude)
}

// MarshalJSON implements json.Marshaler interface
// Returns the Point in UQL format: "POINT(lat lon)"
func (p *Point) MarshalJSON() ([]byte, error) {
	if p == nil {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf(`"%s"`, p.String())), nil
}
