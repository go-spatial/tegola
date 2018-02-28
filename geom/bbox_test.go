package geom_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/geom"
	"github.com/go-spatial/tegola/geom/cmp"
)

func TestBBoxNew(t *testing.T) {
	type tcase struct {
		points   [][2]float64
		expected *geom.BoundingBox
	}
	var tests map[string]tcase
	fn := func(t *testing.T, tc tcase) {
		t.Parallel()
		got := geom.NewBBox(tc.points...)
		if !reflect.DeepEqual(got, tc.expected) {
			t.Errorf("failed,  expected %+v got %+v", tc.expected, *got)
		}
	}
	tests = map[string]tcase{

		"a point": {
			points: [][2]float64{
				{1.0, 2.0},
			},
			expected: &geom.BoundingBox{1.0, 2.0, 1.0, 2.0},
		},
		"3 points": {
			points: [][2]float64{
				{0.0, 0.0},
				{6.0, 4.0},
				{3.0, 7.0},
			},
			expected: &geom.BoundingBox{0.0, 0.0, 6.0, 7.0},
		},
		"4 points": {
			points: [][2]float64{
				{0.0, 0.0},
				{-10.0, -10.0},
				{6.0, 4.0},
				{3.0, 7.0},
			},
			expected: &geom.BoundingBox{-10.0, -10.0, 6.0, 7.0},
		},
		"0 points": {
			points:   [][2]float64{},
			expected: nil,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestBBoxAdd(t *testing.T) {
	type tcase struct {
		bb       *geom.BoundingBox
		bbox     *geom.BoundingBox
		expected *geom.BoundingBox
	}
	fn := func(t *testing.T, tc tcase) {
		t.Parallel()
		bb := tc.bb
		bb.Add(tc.bbox)
		if !cmp.BBox(tc.expected, bb) {
			t.Errorf("failed, expected %+v got %+v", tc.expected, bb)
		}
	}
	tests := map[string]tcase{
		"nil expanded by point": {
			bb:       nil,
			bbox:     &geom.BoundingBox{3.0, 3.0, 3.0, 3.0},
			expected: nil,
		},
		"point expanded by nil": {
			bb:       &geom.BoundingBox{1.0, 2.0, 1.0, 2.0},
			bbox:     nil,
			expected: nil,
		},
		"point expanded by point": {
			bb:       &geom.BoundingBox{1.0, 2.0, 1.0, 2.0},
			bbox:     &geom.BoundingBox{3.0, 3.0, 3.0, 3.0},
			expected: &geom.BoundingBox{1.0, 2.0, 3.0, 3.0},
		},
		"point expanded by enclosing box": {
			bb:       &geom.BoundingBox{1.0, 2.0, 1.0, 2.0},
			bbox:     &geom.BoundingBox{0.0, 0.0, 3.0, 3.0},
			expected: &geom.BoundingBox{0.0, 0.0, 3.0, 3.0},
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestBBoxAddPoints(t *testing.T) {
	type tcase struct {
		bb       *geom.BoundingBox
		points   [][2]float64
		expected *geom.BoundingBox
	}
	fn := func(t *testing.T, tc tcase) {
		t.Parallel()
		bb := tc.bb
		bb.AddPoints(tc.points...)
		if !cmp.BBox(tc.expected, bb) {
			t.Errorf("failed, expected %+v got %+v", tc.expected, bb)
		}
	}
	tests := map[string]tcase{
		"nil expanded by point": {
			bb: nil,
			points: [][2]float64{
				[2]float64{1.0, 2.0},
			},
			expected: nil,
		},
		"point expanded zero points": {
			bb:       &geom.BoundingBox{1.0, 2.0, 1.0, 2.0},
			points:   [][2]float64{},
			expected: &geom.BoundingBox{1.0, 2.0, 1.0, 2.0},
		},
		"point expanded by point": {
			bb: &geom.BoundingBox{1.0, 2.0, 1.0, 2.0},
			points: [][2]float64{
				{3.0, 3.0},
				{1.0, 1.0},
			},
			expected: &geom.BoundingBox{1.0, 1.0, 3.0, 3.0},
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestBBoxContains(t *testing.T) {
	type tcase struct {
		mm       geom.MinMaxer
		bb       *geom.BoundingBox
		expected bool
	}
	fn := func(t *testing.T, tc tcase) {
		got := tc.bb.Contains(tc.mm)
		if got != tc.expected {
			t.Errorf(" contains, expected %v got %v", tc.expected, got)
		}
	}
	tests := map[string]tcase{
		"nil bb nil mm": tcase{
			expected: true,
		},
		"nil bb non-nil mm": tcase{
			mm:       geom.NewBBox([2]float64{0, 0}, [2]float64{10, 10}),
			expected: true,
		},
		"non-nil bb nil mm": tcase{
			bb:       geom.NewBBox([2]float64{0, 0}, [2]float64{10, 10}),
			expected: false,
		},
		"same": tcase{
			bb:       geom.NewBBox([2]float64{0, 0}, [2]float64{10, 10}),
			mm:       geom.NewBBox([2]float64{0, 0}, [2]float64{10, 10}),
			expected: true,
		},
		"contained": tcase{
			bb:       geom.NewBBox([2]float64{0, 0}, [2]float64{10, 10}),
			mm:       geom.NewBBox([2]float64{1, 1}, [2]float64{5, 5}),
			expected: true,
		},
		"same only at 0,0": tcase{
			bb:       geom.NewBBox([2]float64{0, 0}, [2]float64{10, 10}),
			mm:       geom.NewBBox([2]float64{0, 0}, [2]float64{-10, -10}),
			expected: false,
		},
		"overlap not contained": tcase{
			bb:       geom.NewBBox([2]float64{-1, -1}, [2]float64{10, 10}),
			mm:       geom.NewBBox([2]float64{0, 0}, [2]float64{-10, -10}),
			expected: false,
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestBBoxContainsPoint(t *testing.T) {
	type tcase struct {
		bb       *geom.BoundingBox
		pt       [2]float64
		expected bool
	}
	fn := func(t *testing.T, tc tcase) {
		t.Parallel()
		bb := tc.bb
		got := bb.ContainsPoint(tc.pt)
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
			bb:       &geom.BoundingBox{0.0, 0.0, 3.0, 3.0},
			pt:       [2]float64{1.0, 1.0},
			expected: true,
		},
		"uncontained point": {
			bb:       &geom.BoundingBox{0.0, 0.0, 3.0, 3.0},
			pt:       [2]float64{-1.0, -1.0},
			expected: false,
		},
		"nil bb": {
			bb:       nil,
			pt:       [2]float64{-1.0, -1.0},
			expected: true,
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

// TestBBoxAttributes check that the bbox is returning the correct values for the different
//	attributes that a bbox can have.
func TestBBoxAttributes(t *testing.T) {
	bblncmp := func(pt [2]float64, x, y float64) bool {
		return pt[0] == x && pt[1] == y
	}

	fn := func(t *testing.T, bb *geom.BoundingBox) {

		t.Parallel()

		{
			vert := bb.Vertices()

			if len(vert) != 4 {
				t.Errorf("vertices length, expected %v got %v", 4, len(vert))
				return
			}
			if !bblncmp(vert[0], bb.MinX(), bb.MinY()) {
				t.Errorf("vert0] top left, expected %v got %v", [2]float64{bb.MinX(), bb.MinY()}, vert[0])
			}
			if !bblncmp(vert[1], bb.MaxX(), bb.MinY()) {
				t.Errorf("vert[1] top right, expected %v got %v", [2]float64{bb.MaxX(), bb.MinY()}, vert[1])
			}
			if !bblncmp(vert[2], bb.MaxX(), bb.MaxY()) {
				t.Errorf("vert[2] bottom right, expected %v, got %v", [2]float64{bb.MaxX(), bb.MaxY()}, vert[2])
			}
			if !bblncmp(vert[3], bb.MinX(), bb.MaxY()) {
				t.Errorf("vert[3], expected %v got %v", [2]float64{bb.MinX(), bb.MaxY()}, vert[3])
			}
			edges := bb.Edges(nil)
			if len(edges) != 4 {
				t.Errorf("edges length, expected 4 got %v", len(edges))
			} else {
				eedge := [][2][2]float64{
					[2][2]float64{vert[0], vert[1]},
					[2][2]float64{vert[1], vert[2]},
					[2][2]float64{vert[2], vert[3]},
					[2][2]float64{vert[3], vert[0]},
				}
				if !reflect.DeepEqual(edges, eedge) {
					t.Errorf("edges, expected %v got %v", eedge, edges)
				}
			}
			edges = bb.Edges(func(_ ...[2]float64) bool { return false })
			if len(edges) != 4 {
				t.Errorf("edges length, expected 4 got %v", len(edges))
			} else {
				eedge := [][2][2]float64{
					[2][2]float64{vert[3], vert[2]},
					[2][2]float64{vert[2], vert[1]},
					[2][2]float64{vert[1], vert[0]},
					[2][2]float64{vert[0], vert[3]},
				}
				if !reflect.DeepEqual(edges, eedge) {
					t.Errorf("edges, expected %v got %v", eedge, edges)
				}
			}
			poly := bb.AsPolygon()
			epoly := geom.Polygon{vert}
			if !reflect.DeepEqual(epoly, poly) {
				t.Errorf("as polygon, expected %v got %v", epoly, poly)
			}
		}

		minx, miny, maxx, maxy := -math.MaxFloat64, -math.MaxFloat64, math.MaxFloat64, math.MaxFloat64
		if bb != nil {
			minx, miny, maxx, maxy = bb[0], bb[1], bb[2], bb[3]
		}

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
		cbb := bb.Clone()
		if !cmp.BBox(bb, cbb) {
			t.Errorf("Clone, equal, expected (%v) true got (%v) false", bb, cbb)
		}

	}
	tests := map[string]*geom.BoundingBox{
		"std": &geom.BoundingBox{0.0, 0.0, 10.0, 10.0},
		"nil": nil,
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestBBoxScaleBy(t *testing.T) {
	type tcase struct {
		bb    *geom.BoundingBox
		scale float64
		ebb   *geom.BoundingBox
	}
	fn := func(t *testing.T, tc tcase) {
		sbb := tc.bb.ScaleBy(tc.scale)
		if !reflect.DeepEqual(sbb, tc.ebb) {
			t.Errorf("Scale by, expected %v got %v", tc.ebb, sbb)
		}
	}
	tests := map[string]tcase{
		"nil": tcase{
			scale: 2.0,
		},
		"1.0 scale": tcase{
			bb:    &geom.BoundingBox{0, 0, 10, 10},
			ebb:   &geom.BoundingBox{0, 0, 10, 10},
			scale: 1.0,
		},
		"2.0 scale": tcase{
			bb:    &geom.BoundingBox{0, 0, 10, 10},
			ebb:   &geom.BoundingBox{0, 0, 20, 20},
			scale: 2.0,
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestBBoxIntersect(t *testing.T) {
	type tcase struct {
		bb   *geom.BoundingBox
		nbb  *geom.BoundingBox
		ibb  *geom.BoundingBox
		does bool
	}
	fn := func(t *testing.T, tc tcase) {
		gbb, does := tc.bb.Intersect(tc.nbb)
		if does != tc.does {
			t.Errorf(" Intersect does, expected %v got %v", tc.does, does)
		}
		if !cmp.BBox(tc.ibb, gbb) {
			t.Errorf(" Intersect, expected %v got %v", tc.ibb, gbb)
		}
	}
	tests := map[string]tcase{
		"nil": tcase{
			bb:   nil,
			nbb:  nil,
			ibb:  nil,
			does: true,
		},
		"bb not nil": tcase{
			bb:   &geom.BoundingBox{10, 10, 20, 20},
			nbb:  nil,
			ibb:  &geom.BoundingBox{10, 10, 20, 20},
			does: true,
		},
		"1": tcase{
			bb:   &geom.BoundingBox{10, 10, 20, 20},
			nbb:  &geom.BoundingBox{10, 10, 15, 15},
			ibb:  &geom.BoundingBox{10, 10, 15, 15},
			does: true,
		},
		"2": tcase{
			bb:   &geom.BoundingBox{10, 10, 15, 15},
			nbb:  &geom.BoundingBox{10, 10, 20, 20},
			ibb:  &geom.BoundingBox{10, 10, 15, 15},
			does: true,
		},
		"3": tcase{
			bb:   &geom.BoundingBox{10, 10, 15, 15},
			nbb:  &geom.BoundingBox{15, 15, 20, 20},
			ibb:  nil,
			does: false,
		},
		"4": tcase{
			bb:   &geom.BoundingBox{10, 10, 15, 15},
			nbb:  &geom.BoundingBox{10, 15, 20, 20},
			ibb:  nil,
			does: false,
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestBBoxArea(t *testing.T) {
	maxarea := math.Inf(1)
	type tcase struct {
		bb   *geom.BoundingBox
		area float64
	}
	fn := func(t *testing.T, tc tcase) {
		a := tc.bb.Area()
		if !cmp.Float(tc.area, a) {
			t.Errorf("area, expected %v got %v", tc.area, a)
		}
	}
	tests := map[string]tcase{
		"nil": tcase{
			bb:   nil,
			area: maxarea,
		},
		"simple 10x10": tcase{
			bb:   geom.NewBBox([2]float64{0, 0}, [2]float64{10, 10}),
			area: 100,
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestBBoxContainsLine(t *testing.T) {
	type tcase struct {
		bb *geom.BoundingBox
		l  [2][2]float64
		e  bool
	}
	fn := func(t *testing.T, tc tcase) {
		if got := tc.bb.ContainsLine(tc.l); got != tc.e {
			t.Errorf("contains line, expected %v got %v", tc.e, got)
		}
	}
	tests := map[string]tcase{
		"nil": tcase{
			l: [2][2]float64{[2]float64{0, 0}, [2]float64{10, 10}},
			e: true,
		},
		"contained": tcase{
			bb: &geom.BoundingBox{-1, -1, 20, 20},
			l:  [2][2]float64{[2]float64{0, 0}, [2]float64{10, 10}},
			e:  true,
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}

}
