package test

import (
	"fmt"
	"testing"

	"github.com/ultipa/ultipa-go-driver/v5/sdk/types"
)

func TestString(t *testing.T) {
	p := types.NewPoint(0.00, 1.00)
	fmt.Println(p)

	point := types.Point{
		Latitude:  0.0,
		Longitude: -1,
	}
	fmt.Println(point.String())

}

func TestParsePointFromString(t *testing.T) {

	poinStrs := []string{
		"Point(9  22.2)",
		"Point(-10.1  22.2)",
		"Point(-80  -80)",
		"Point(0.00 0.00)",
		"POINT(0.00 0.00)",
		"point(1.00 -2.00)",
	}

	for i, str := range poinStrs {
		point, err := types.PointFromStr(str)
		if err != nil {
			t.Fatal(fmt.Sprintf("%d: %s, %v", i, str, err))
		}
		fmt.Println(point)
	}
}
