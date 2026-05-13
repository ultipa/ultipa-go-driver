//go:build unit

package unit

import (
	"testing"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// Unit tests for Vector.Len() (#9 — Python-parity: len(vec)).
// Iteration is idiomatic in Go via `for _, v := range vec.Values`, so we
// just pin Len() here.

func TestVector_Len_ReturnsDimensions(t *testing.T) {
	cases := []struct {
		values []float32
		want   int
	}{
		{nil, 0},
		{[]float32{}, 0},
		{[]float32{1.0}, 1},
		{[]float32{0.5, -1.5, 2.0}, 3},
	}
	for i, c := range cases {
		v := gqldb.Vector{Values: c.values}
		if got := v.Len(); got != c.want {
			t.Errorf("case %d: Len() = %d, want %d", i, got, c.want)
		}
	}
}

func TestVector_Range_YieldsValuesInOrder(t *testing.T) {
	v := gqldb.Vector{Values: []float32{0.5, -1.5, 2.0}}
	expected := []float32{0.5, -1.5, 2.0}
	for i, x := range v.Values {
		if x != expected[i] {
			t.Errorf("index %d: got %v, want %v", i, x, expected[i])
		}
	}
}
