package cmp

import (
	"fmt"
	"testing"

	geom "github.com/go-spatial/tegola/geom"
)

/*
RotateToLeftMostPoint is a slightly more complicated function that is relied upon
by Comparision for LineStrings and all the functions that rely on it. That's the
reason for the test cases. Even though this seems like a trivial function. It got
a bit of complexity to it.
*/
func TestRotateToLeftMostPoint(t *testing.T) {

	fn := func(t *testing.T, tc [][2]float64) {
		t.Parallel()
		if len(tc) == 0 {
			panic(fmt.Sprintf("bad test case Zero or nil."))
			return
		}
		// First we need to find the smallest point as defined by XYLessPoint.
		minptidx := FindMinPointIdx(tc)
		minpt := tc[minptidx]
		// Create a copy that we are going to apply the rotation to.
		ctc := make([][2]float64, len(tc))
		copy(ctc, tc)
		RotateToLeftMostPoint(ctc)
		if ctc[0][0] != minpt[0] || ctc[0][1] != minpt[1] {
			t.Errorf("first point should be the smallest point, expected %v got %v", minpt, ctc[0])
		}
		j := minptidx
		for i := 0; i < len(ctc); i++ {
			if ctc[i][0] != tc[j][0] || ctc[i][1] != tc[j][1] {
				t.Errorf("points are not in the correct order, expected %v(%v) got %v(%v)", i, ctc[i], j, tc[j])
			}
			j++
			if j >= len(tc) {
				j = 0
			}
		}
	}
	tests := map[string][][2]float64{

		"1": [][2]float64{{11, 10}, {9, 8}, {7, 6}, {5, 4}},
		"2": [][2]float64{{0, 10}, {9, 8}, {7, 6}, {5, 4}},
		"3": [][2]float64{{0, 10}},
		"4": [][2]float64{{3, 100}, {4, -5}, {6, 90}, {4, 15}},
		"5": [][2]float64{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
		"6": [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

// This is more to execute that line of code, which is more to cover all the cases. It unlikly to be call in
// regular operation.
func TestByXYLess(t *testing.T) {
	var byxy bySubRingSizeXY
	if !byxy.Less(0, 1) {
		t.Errorf(" first ring should always be less, expected true got false")
	}
}

func TestFindMinIdx(t *testing.T) {
	type tcase struct {
		line [][2]float64
		min  int
	}
	fn := func(t *testing.T, tc tcase) {
		t.Parallel()
		ls := ByXY(tc.line)
		got := FindMinIdx(ls)
		if got != tc.min {
			t.Errorf("FindMinIdx -- %#v , expected %v got %v ", tc.line, tc.min, got)
		}

	}
	tests := map[string]tcase{

		"nil": {
			line: nil,
			min:  0,
		},
		"0": {
			line: [][2]float64{},
			min:  0,
		},
		"1": {
			line: [][2]float64{{11, 10}, {9, 8}, {7, 6}, {5, 4}},
			min:  3,
		},
		"2": {
			line: [][2]float64{{0, 10}, {9, 8}, {7, 6}, {5, 4}},
			min:  0,
		},
		"3": {
			line: [][2]float64{{0, 10}},
			min:  0,
		},
		"4": {
			line: [][2]float64{{3, 100}, {4, -5}, {6, 90}, {4, 15}},
			min:  0,
		},
		"5": {
			line: [][2]float64{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
			min:  1,
		},
		"6": {
			line: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			min:  0,
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestPoint(t *testing.T) {

	less := func(p1, p2 [2]float64) bool {
		if p1[0] == p2[0] {
			return p1[1] < p2[1]
		}
		return p1[0] < p2[0]
	}

	type tc struct {
		p1 [2]float64
		p2 [2]float64
		e  bool
	}

	fn := func(t *testing.T, tc tc) {
		gp1, gp2 := geom.Point(tc.p1), geom.Point(tc.p2)
		e := (tc.p1[0] == tc.p2[0]) && (tc.p1[1] == tc.p2[1])
		if e != PointEqual(tc.p1, tc.p2) {
			t.Errorf("p1 == p2, expected %v got %v", e, !e)
		}
		if e != PointerEqual(gp1, gp2) {
			t.Errorf("p1 == p2, expected %v got %v", e, !e)
		}
		if e != GeometryEqual(gp1, gp2) {
			t.Errorf("p1 == p2, expected %v got %v", e, !e)
		}

		l := less(tc.p1, tc.p2)
		if l != PointLess(tc.p1, tc.p2) {
			t.Errorf("p1 < p2, expected %v got %v", l, !l)
		}
		l = less(tc.p2, tc.p1)
		if l != PointLess(tc.p2, tc.p1) {
			t.Errorf("p2 < p1, expected %v got %v", l, !l)
		}

	}

	tests := map[string]tc{
		"0": tc{
			p1: [2]float64{1, 2},
			p2: [2]float64{1, 2},
			e:  true,
		},
		"1": tc{
			p1: [2]float64{1, 1},
			p2: [2]float64{1, 2},
			e:  false,
		},
		"3": tc{
			p1: [2]float64{1, 2},
			p2: [2]float64{2, 2},
			e:  false,
		},
		"4": tc{
			p1: [2]float64{1, 1},
			p2: [2]float64{2, 2},
			e:  false,
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}

}

func TestMultiPoint(t *testing.T) {
	type tc struct {
		l1 [][2]float64
		l2 [][2]float64
		e  bool
	}

	fn := func(t *testing.T, tc tc) {

		gmp1, gmp2 := geom.MultiPoint(tc.l1), geom.MultiPoint(tc.l2)
		if tc.e != MultiPointerEqual(gmp1, gmp2) {
			t.Errorf("MultiPointer are equal, expected %v got %v", tc.e, !tc.e)
		}
		if tc.e != MultiPointerEqual(gmp1, gmp2) {
			t.Errorf("MultiPointer are equal, expected %v got %v", tc.e, !tc.e)
		}
		if tc.e != GeometryEqual(gmp1, gmp2) {
			t.Errorf("GeometryEqual are equal, expected %v got %v", tc.e, !tc.e)
		}
	}

	tests := map[string]tc{
		"0": tc{
			// Simple test.
			l1: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			l2: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			e:  true,
		},
		"1": tc{
			// Simple test.
			l1: [][2]float64{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
			l2: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			e:  true,
		},
		"2": tc{
			// Simple test.
			l1: [][2]float64{{1, 4}, {1, 5}, {1, 2}, {1, 3}},
			l2: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			e:  true,
		},
		"3": tc{
			// Simple test.
			l1: [][2]float64{},
			l2: [][2]float64{},
			e:  true,
		},
		"4": tc{
			// Simple test.
			l1: nil,
			l2: [][2]float64{},
			e:  true,
		},
		"5": tc{
			// Simple test.
			l1: nil,
			l2: nil,
			e:  true,
		},
		"6": tc{
			// Simple test.
			l1: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			l2: [][2]float64{{1, 5}, {1, 2}, {1, 4}, {1, 4}},
			e:  false,
		},
		"7": tc{
			// Simple test.
			l1: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			l2: [][2]float64{{1, 2}, {1, 3}, {1, 4}},
			e:  false,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestLineString(t *testing.T) {
	type tc struct {
		l1 [][2]float64
		l2 [][2]float64
		e  bool
	}

	fn := func(t *testing.T, tc tc) {
		g1, g2 := geom.LineString(tc.l1), geom.LineString(tc.l2)
		if tc.e != LineStringEqual(tc.l1, tc.l2) {
			t.Errorf("LineString equal, expected %v got %v", tc.e, !tc.e)
		}
		if tc.e != LineStringerEqual(g1, g2) {
			t.Errorf("LineStringer equal, expected %v got %v", tc.e, !tc.e)
		}
		if tc.e != GeometryEqual(g1, g2) {
			t.Errorf("Geometry equal, expected %v got %v", tc.e, !tc.e)
		}
	}

	tests := map[string]tc{
		"0": tc{
			// Simple test.
			l1: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			l2: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			e:  true,
		},
		"1": tc{
			// Simple test.
			l1: [][2]float64{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
			l2: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			e:  true,
		},
		"2": tc{
			// Simple test.
			l1: [][2]float64{{1, 4}, {1, 5}, {1, 2}, {1, 3}},
			l2: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			e:  true,
		},
		"3": tc{
			// Simple test.
			l1: [][2]float64{},
			l2: [][2]float64{},
			e:  true,
		},
		"4": tc{
			// Simple test.
			l1: nil,
			l2: [][2]float64{},
			e:  true,
		},
		"5": tc{
			// Simple test.
			l1: nil,
			l2: nil,
			e:  true,
		},
		"6": tc{
			// Simple test.
			l1: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			l2: [][2]float64{{1, 2}, {1, 3}, {1, 4}},
			e:  false,
		},
		"7": tc{
			// Simple test.
			l1: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			l2: [][2]float64{{1, 5}, {1, 2}, {1, 4}, {1, 4}},
			e:  false,
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestMultiLineString(t *testing.T) {
	type tc struct {
		ml1, ml2 [][][2]float64
		e        bool
	}

	fn := func(t *testing.T, tc tc) {
		if tc.e != MultiLineEqual(tc.ml1, tc.ml2) {
			t.Errorf("MultiLineString equal, expected %v got %v", tc.e, !tc.e)
		}
		g1, g2 := geom.MultiLineString(tc.ml1), geom.MultiLineString(tc.ml2)
		if tc.e != MultiLineStringerEqual(g1, g2) {
			t.Errorf("MultiLineStringer equal, expected %v got %v", tc.e, !tc.e)
		}
		if tc.e != GeometryEqual(g1, g2) {
			t.Errorf("Geometry equal, expected %v got %v", tc.e, !tc.e)
		}

	}

	/***** TEST CASES ******/
	tests := map[string]tc{
		"0": tc{
			// Simple test.
			ml1: [][][2]float64{{{1, 2}, {1, 3}, {1, 4}, {1, 5}}},
			ml2: [][][2]float64{{{1, 2}, {1, 3}, {1, 4}, {1, 5}}},
			e:   true,
		},
		"1": tc{
			// Simple test.
			ml1: [][][2]float64{{{1, 5}, {1, 2}, {1, 3}, {1, 4}}},
			ml2: [][][2]float64{{{1, 2}, {1, 3}, {1, 4}, {1, 5}}},
			e:   true,
		},
		"2": tc{
			// Simple test.
			ml1: [][][2]float64{},
			ml2: [][][2]float64{},
			e:   true,
		},
		"3": tc{
			// Simple test.
			ml1: nil,
			ml2: [][][2]float64{},
			e:   true,
		},
		"4": tc{
			// Simple test.
			ml1: nil,
			ml2: nil,
			e:   true,
		},
		"5": tc{
			// Simple test.
			ml1: [][][2]float64{{{1, 5}, {1, 2}, {1, 3}, {1, 4}}},
			ml2: [][][2]float64{{{1, 2}, {1, 3}, {1, 4}}},
			e:   false,
		},
		"6": tc{
			// Simple test.
			ml1: [][][2]float64{{{1, 5}, {1, 2}, {1, 3}, {1, 4}}},
			ml2: [][][2]float64{{{1, 2}, {1, 3}, {1, 4}, {1, 6}}},
			e:   false,
		},
		"different ring sizes": tc{
			// Simple test.
			ml1: [][][2]float64{
				{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
			},
			ml2: [][][2]float64{
				{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
				{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
			},
			e: false,
		},
		"same rings different order - both": tc{
			// Simple test.
			ml1: [][][2]float64{
				{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
				{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
			},
			ml2: [][][2]float64{
				{{2, 2}, {2, 3}, {2, 4}, {2, 5}},
				{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			},
			e: true,
		},
		"same rings different order in rings": tc{
			// Simple test.
			ml1: [][][2]float64{
				{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
				{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
			},
			ml2: [][][2]float64{
				{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
				{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
			},
			e: true,
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestPolygon(t *testing.T) {
	type tc struct {
		ply1, ply2 [][][2]float64
		e          bool
	}

	fn := func(t *testing.T, tc tc) {
		g1, g2 := geom.Polygon(tc.ply1), geom.Polygon(tc.ply2)
		if tc.e != PolygonEqual(tc.ply1, tc.ply2) {
			t.Errorf("polygons equal, expected %v got %v", tc.e, !tc.e)
		}
		if tc.e != PolygonerEqual(g1, g2) {
			t.Errorf("polygoner equal, expected %v got %v", tc.e, !tc.e)
		}
		if tc.e != GeometryEqual(g1, g2) {
			t.Errorf("geometry equal, expected %v got %v", tc.e, !tc.e)
		}
	}

	/***** TEST CASES ******/
	tests := map[string]tc{
		"0": tc{
			// Simple test.
			ply1: [][][2]float64{{{1, 2}, {1, 3}, {1, 4}, {1, 5}}},
			ply2: [][][2]float64{{{1, 2}, {1, 3}, {1, 4}, {1, 5}}},
			e:    true,
		},
		"1": tc{
			// Simple test.
			ply1: [][][2]float64{{{1, 5}, {1, 2}, {1, 3}, {1, 4}}},
			ply2: [][][2]float64{{{1, 2}, {1, 3}, {1, 4}, {1, 5}}},
			e:    true,
		},
		"2": tc{
			// Simple test.
			ply1: [][][2]float64{},
			ply2: [][][2]float64{},
			e:    true,
		},
		"3": tc{
			// Simple test.
			ply1: nil,
			ply2: [][][2]float64{},
			e:    true,
		},
		"4": tc{
			// Simple test.
			ply1: nil,
			ply2: nil,
			e:    true,
		},
		"5": tc{
			// Simple test.
			ply1: [][][2]float64{{{1, 5}, {1, 2}, {1, 3}, {1, 4}}},
			ply2: [][][2]float64{{{1, 2}, {1, 3}, {1, 4}}},
			e:    false,
		},
		"6": tc{
			// Simple test.
			ply1: [][][2]float64{{{1, 5}, {1, 2}, {1, 3}, {1, 4}}},
			ply2: [][][2]float64{{{1, 2}, {1, 3}, {1, 4}, {1, 6}}},
			e:    false,
		},
		"7": tc{
			// Simple test.
			ply1: [][][2]float64{{{1, 5}, {1, 2}, {1, 3}, {1, 4}}},
			ply2: nil,
			e:    false,
		},
		"first ring not same": tc{
			// Simple test.
			ply1: [][][2]float64{
				{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
				{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
			},
			ply2: [][][2]float64{
				{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
				{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
			},
			e: false,
		},
		"first ring same, different order for others": tc{
			// Simple test.
			ply1: [][][2]float64{
				{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
				{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
				{{4, 5}, {4, 2}, {4, 3}},
			},
			ply2: [][][2]float64{
				{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
				{{4, 5}, {4, 2}, {4, 3}},
				{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
			},
			e: true,
		},
		"first ring same, different order for different others": tc{
			// Simple test.
			ply1: [][][2]float64{
				{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
				{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
				{{4, 5}, {4, 2}, {4, 3}},
			},
			ply2: [][][2]float64{
				{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
				{{4, 5}, {4, 2}, {4, 3}},
				{{2, 5}, {2, 2}, {2, 3}},
			},
			e: false,
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestMultiPolygon(t *testing.T) {
	type tc struct {
		mp1, mp2 [][][][2]float64
		e        bool
	}

	fn := func(t *testing.T, tc tc) {
		g1, g2 := geom.MultiPolygon(tc.mp1), geom.MultiPolygon(tc.mp2)
		if tc.e != MultiPolygonerEqual(g1, g2) {
			t.Errorf("polygoner equal, expected %v got %v", tc.e, !tc.e)
		}
		if tc.e != GeometryEqual(g1, g2) {
			t.Errorf("geometry equal, expected %v got %v", tc.e, !tc.e)
		}
	}

	/***** TEST CASES ******/
	tests := map[string]tc{
		"0": tc{
			// Simple test.
			mp1: [][][][2]float64{{{{1, 2}, {1, 3}, {1, 4}, {1, 5}}}},
			mp2: [][][][2]float64{{{{1, 2}, {1, 3}, {1, 4}, {1, 5}}}},
			e:   true,
		},
		"1": tc{
			// Simple test.
			mp1: [][][][2]float64{{{{1, 5}, {1, 2}, {1, 3}, {1, 4}}}},
			mp2: [][][][2]float64{{{{1, 2}, {1, 3}, {1, 4}, {1, 5}}}},
			e:   true,
		},
		"2": tc{
			// Simple test.
			mp1: [][][][2]float64{},
			mp2: [][][][2]float64{},
			e:   true,
		},
		"3": tc{
			// Simple test.
			mp1: nil,
			mp2: [][][][2]float64{},
			e:   true,
		},
		"4": tc{
			// Simple test.
			mp1: nil,
			mp2: nil,
			e:   true,
		},
		"5": tc{
			// Simple test.
			mp1: [][][][2]float64{{{{1, 5}, {1, 2}, {1, 3}, {1, 4}}}},
			mp2: [][][][2]float64{{{{1, 2}, {1, 3}, {1, 4}}}},
			e:   false,
		},
		"6": tc{
			// Simple test.
			mp1: [][][][2]float64{{{{1, 5}, {1, 2}, {1, 3}, {1, 4}}}},
			mp2: [][][][2]float64{{{{1, 2}, {1, 3}, {1, 4}, {1, 6}}}},
			e:   false,
		},
		"7": tc{
			// Simple test.
			mp1: [][][][2]float64{{{{1, 5}, {1, 2}, {1, 3}, {1, 4}}}},
			mp2: nil,
			e:   false,
		},
		"first ring not same": tc{
			// Simple test.
			mp1: [][][][2]float64{{
				{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
				{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
			}},
			mp2: [][][][2]float64{{
				{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
				{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
			}},
			e: false,
		},
		"first ring same, different order for others": tc{
			// Simple test.
			mp1: [][][][2]float64{{
				{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
				{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
				{{4, 5}, {4, 2}, {4, 3}},
			}},
			mp2: [][][][2]float64{{
				{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
				{{4, 5}, {4, 2}, {4, 3}},
				{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
			}},
			e: true,
		},
		"Polygons in different order": tc{
			// Simple test.
			mp1: [][][][2]float64{
				{ // Polygon one
					{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
					{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
					{{4, 5}, {4, 2}, {4, 3}},
				},
				{ // Polygon two
				},
			},
			mp2: [][][][2]float64{
				{ // Polygon two
				},
				{ // Polygon one
					{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
					{{4, 5}, {4, 2}, {4, 3}},
					{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
				},
			},
			e: true,
		},
		"Polygons in different order 1": tc{
			// Simple test.
			mp1: [][][][2]float64{
				{ // Polygon one
					{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
					{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
					{{4, 5}, {4, 2}, {4, 3}},
				},
				{ // Polygon two
					{{12, 5}, {12, 2}, {12, 3}, {12, 4}},
					{{14, 5}, {14, 2}, {14, 3}},
				},
			},
			mp2: [][][][2]float64{
				{ // Polygon two
					{{12, 5}, {12, 2}, {12, 3}, {12, 4}},
					{{14, 5}, {14, 2}, {14, 3}},
				},
				{ // Polygon one
					{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
					{{4, 5}, {4, 2}, {4, 3}},
					{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
				},
			},
			e: true,
		},
		"different Polygons in different order ": tc{
			// Simple test.
			mp1: [][][][2]float64{
				{ // Polygon one
					{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
					{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
					{{4, 5}, {4, 2}, {4, 3}},
				},
				{ // Polygon two
					{{12, 5}, {12, 2}, {12, 3}, {12, 4}},
					{{14, 5}, {14, 2}, {14, 3}},
				},
			},
			mp2: [][][][2]float64{
				{ // Polygon two
					{{14, 5}, {14, 2}, {14, 3}},
					{{12, 5}, {12, 2}, {12, 3}, {12, 4}},
				},
				{ // Polygon one
					{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
					{{4, 5}, {4, 2}, {4, 3}},
					{{2, 5}, {2, 2}, {2, 3}, {2, 4}},
				},
			},
			e: false,
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestCollection(t *testing.T) {
	type tc struct {
		cl1, cl2 geom.Collection
		e        bool
	}

	fn := func(t *testing.T, tc tc) {
		if tc.e != CollectionerEqual(tc.cl1, tc.cl2) {
			t.Errorf("polygoner equal, expected %v got %v", tc.e, !tc.e)
		}
		if tc.e != GeometryEqual(tc.cl1, tc.cl2) {
			t.Errorf("geometry equal, expected %v got %v", tc.e, !tc.e)
		}
	}

	/***** TEST CASES ******/
	tests := map[string]tc{
		"0": tc{
			// Simple test.
			cl1: geom.Collection{geom.Point{0.0, 0.0}},
			cl2: geom.Collection{geom.Point{0.0, 0.0}},
			e:   true,
		},
		"1": tc{
			// Simple test.
			cl1: geom.Collection{geom.Point{0.0, 0.0}},
			cl2: geom.Collection{geom.Point{1.0, 0.0}},
			e:   false,
		},
		"2": tc{
			// Simple test.
			cl1: geom.Collection{geom.Point{0.0, 0.0}},
			cl2: geom.Collection{},
			e:   false,
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestGeometry(t *testing.T) {
	// Unknown types of geometries are always unequal.
	if GeometryEqual(nil, nil) {
		t.Errorf(" unknown types, expected false, got true")
	}
}
