package geom_test

import (
	"reflect"
	"testing"

	"github.com/terranodo/tegola/geom"
)

func TestNewBBox(t *testing.T) {
	type tcase struct {
		points   [][2]float64
		expected geom.BoundingBox
	}
	var tests map[string]tcase
	fn := func(t *testing.T, tc tcase) {
		t.Parallel()
		got := geom.NewBBox(tc.points...)
		if !reflect.DeepEqual(got, tc.expected) {
			t.Errorf("failed,  expected %+v got %+v", tc.expected, got)
		}
	}
	tests = map[string]tcase{

		"a point": {
			points: [][2]float64{
				{1.0, 2.0},
			},
			expected: geom.BoundingBox{
				[2]float64{1.0, 2.0},
				[2]float64{1.0, 2.0},
			},
		},
		"3 points": {
			points: [][2]float64{
				{0.0, 0.0},
				{6.0, 4.0},
				{3.0, 7.0},
			},
			expected: geom.BoundingBox{
				[2]float64{0.0, 0.0},
				[2]float64{6.0, 7.0},
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestBBoxAdd(t *testing.T) {
	type tcase struct {
		bb       geom.BoundingBox
		bbox     geom.BoundingBox
		expected geom.BoundingBox
	}
	fn := func(t *testing.T, tc tcase) {
		t.Parallel()
		bb := tc.bb
		bb.Add(tc.bbox)
		if !reflect.DeepEqual(tc.expected, bb) {
			t.Errorf("failed, expected %+v got %+v", tc.expected, bb)
		}
	}
	tests := map[string]tcase{
		"point expanded by point": {
			bb: geom.BoundingBox{
				[2]float64{1.0, 2.0},
				[2]float64{1.0, 2.0},
			},
			bbox: geom.BoundingBox{
				[2]float64{3.0, 3.0},
				[2]float64{3.0, 3.0},
			},
			expected: geom.BoundingBox{
				[2]float64{1.0, 2.0},
				[2]float64{3.0, 3.0},
			},
		},
		"point expaned by enclosing box": {
			bb: geom.BoundingBox{
				[2]float64{1.0, 2.0},
				[2]float64{1.0, 2.0},
			},
			bbox: geom.BoundingBox{
				[2]float64{0.0, 0.0},
				[2]float64{3.0, 3.0},
			},
			expected: geom.BoundingBox{
				[2]float64{0.0, 0.0},
				[2]float64{3.0, 3.0},
			},
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestBBoxContains(t *testing.T) {
	type tcase struct {
		bb       geom.BoundingBox
		pt       [2]float64
		expected bool
	}
	fn := func(t *testing.T, tc tcase) {
		t.Parallel()
		bb := tc.bb
		got := bb.Contains(tc.pt)
		exp := tc.expected
		does := "does "
		if !exp {
			does = "does not "
		}
		if got != exp {
			t.Errorf(does+" contain, expected %v got %v", exp, got)
		}
	}
	tests := map[string]tcase{
		"contained point": {
			bb: geom.BoundingBox{
				[2]float64{0.0, 0.0},
				[2]float64{3.0, 3.0},
			},
			pt:       [2]float64{1.0, 1.0},
			expected: true,
		},
		"uncontained point": {
			bb: geom.BoundingBox{
				[2]float64{0.0, 0.0},
				[2]float64{3.0, 3.0},
			},
			pt:       [2]float64{-1.0, -1.0},
			expected: false,
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}
