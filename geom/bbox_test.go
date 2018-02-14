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

func TestBBoxAttributes(t *testing.T) {
	bblncmp := func(pt [2]float64, x, y float64) bool { return pt[0] == x && pt[1] == y }

	fn := func(t *testing.T, bb geom.BoundingBox) {

		t.Parallel()

		if !bblncmp(bb.TopLeft(), bb[0][0], bb[0][1]) {
			t.Errorf("top left, expected %v, got %v", bb[0], bb.TopLeft())
		}
		if !bblncmp(bb.BottomRight(), bb[1][0], bb[1][1]) {
			t.Errorf("bottom right, expected %v, got %v", bb[1], bb.BottomRight())
		}
		if !bblncmp(bb.TopRight(), bb[1][0], bb[0][1]) {
			t.Errorf("top right, expected %v, got %v", [2]float64{bb[1][0], bb[0][1]}, bb.TopRight())
		}
		if !bblncmp(bb.BottomLeft(), bb[0][0], bb[1][1]) {
			t.Errorf("bottom left, expected %v, got %v", [2]float64{bb[0][0], bb[1][1]}, bb.BottomLeft())
		}

		minx, miny, maxx, maxy := bb[0][0], bb[0][1], bb[1][0], bb[1][1]
		if minx > maxx {
			minx, maxx = maxx, minx
		}
		if miny > maxy {
			miny, maxy = maxy, miny
		}

		if maxx != bb.MaxX() {
			t.Errorf("maxx, expected %v, got %v", maxx, bb.MaxX())
		}
		if minx != bb.MinX() {
			t.Errorf("minx, expected %v, got %v", minx, bb.MinX())
		}
		if maxy != bb.MaxY() {
			t.Errorf("maxy, expected %v, got %v", maxy, bb.MaxY())
		}
		if miny != bb.MinY() {
			t.Errorf("miny, expected %v, got %v", miny, bb.MinY())
		}

	}
	tests := map[string]geom.BoundingBox{
		"std": geom.BoundingBox{
			[2]float64{0.0, 0.0},
			[2]float64{10.0, 10.0},
		},
		"inverted-y": geom.BoundingBox{
			[2]float64{0.0, 10.0},
			[2]float64{10.0, 0.0},
		},
		"inverted-x": geom.BoundingBox{
			[2]float64{10.0, 0.0},
			[2]float64{0.0, 10.0},
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}
