package cmp

import (
	"fmt"
	"testing"

	"github.com/gdey/tbltest"
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

func TestPoint(t *testing.T) {
	type tc struct {
		p1 [2]float64
		p2 [2]float64
		e  bool
	}

	fn := func(idx int, tc tc) {
		if tc.e != PointEqual(tc.p1, tc.p2) {
			t.Errorf("[%v] Points are same, Expected %v Got %v", idx, tc.e, !tc.e)
		}
	}

	tbltest.Cases(
		tc{
			p1: [2]float64{1, 2},
			p2: [2]float64{1, 2},
			e:  true,
		},
		tc{
			p1: [2]float64{1, 1},
			p2: [2]float64{1, 2},
			e:  false,
		},
	).Run(fn)
}

func TestMultiPoint(t *testing.T) {
	type tc struct {
		l1 [][2]float64
		l2 [][2]float64
		e  bool
	}

	fn := func(idx int, tc tc) {
		if tc.e != MultiPointEqual(tc.l1, tc.l2) {
			t.Errorf("[%v] MultiPoint are same, Expected %v Got %v", idx, tc.e, !tc.e)
		}
	}

	tbltest.Cases(
		tc{
			// Simple test.
			l1: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			l2: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			e:  true,
		},
		tc{
			// Simple test.
			l1: [][2]float64{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
			l2: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			e:  true,
		},
		tc{
			// Simple test.
			l1: [][2]float64{{1, 4}, {1, 5}, {1, 2}, {1, 3}},
			l2: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			e:  true,
		},
		tc{
			// Simple test.
			l1: [][2]float64{},
			l2: [][2]float64{},
			e:  true,
		},
		tc{
			// Simple test.
			l1: nil,
			l2: [][2]float64{},
			e:  true,
		},
		tc{
			// Simple test.
			l1: nil,
			l2: nil,
			e:  true,
		},
		tc{
			// Simple test.
			l1: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			l2: [][2]float64{{1, 5}, {1, 2}, {1, 4}, {1, 4}},
			e:  false,
		},
		tc{
			// Simple test.
			l1: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			l2: [][2]float64{{1, 2}, {1, 3}, {1, 4}},
			e:  false,
		},
	).Run(fn)
}

func TestLineString(t *testing.T) {
	type tc struct {
		l1 [][2]float64
		l2 [][2]float64
		e  bool
	}

	fn := func(idx int, tc tc) {
		if tc.e != LineStringEqual(tc.l1, tc.l2) {
			t.Errorf("[%v] LineString are same, Expected %v Got %v", idx, tc.e, !tc.e)
		}
	}

	tbltest.Cases(
		tc{
			// Simple test.
			l1: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			l2: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			e:  true,
		},
		tc{
			// Simple test.
			l1: [][2]float64{{1, 5}, {1, 2}, {1, 3}, {1, 4}},
			l2: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			e:  true,
		},
		tc{
			// Simple test.
			l1: [][2]float64{{1, 4}, {1, 5}, {1, 2}, {1, 3}},
			l2: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			e:  true,
		},
		tc{
			// Simple test.
			l1: [][2]float64{},
			l2: [][2]float64{},
			e:  true,
		},
		tc{
			// Simple test.
			l1: nil,
			l2: [][2]float64{},
			e:  true,
		},
		tc{
			// Simple test.
			l1: nil,
			l2: nil,
			e:  true,
		},
		tc{
			// Simple test.
			l1: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			l2: [][2]float64{{1, 2}, {1, 3}, {1, 4}},
			e:  false,
		},
		tc{
			// Simple test.
			l1: [][2]float64{{1, 2}, {1, 3}, {1, 4}, {1, 5}},
			l2: [][2]float64{{1, 5}, {1, 2}, {1, 4}, {1, 4}},
			e:  false,
		},
	).Run(fn)
}

func TestPolygon(t *testing.T) {
	type tc struct {
		ply1, ply2 [][][2]float64
		e          bool
	}

	fn := func(idx int, tc tc) {
		if tc.e != PolygonEqual(tc.ply1, tc.ply2) {
			t.Errorf("[%v] Polygon are same, Expected %v Got %v", idx, tc.e, !tc.e)
		}
	}

	/***** TEST CASES ******/
	tbltest.Cases(
		tc{
			// Simple test.
			ply1: [][][2]float64{{{1, 2}, {1, 3}, {1, 4}, {1, 5}}},
			ply2: [][][2]float64{{{1, 2}, {1, 3}, {1, 4}, {1, 5}}},
			e:    true,
		},
		tc{
			// Simple test.
			ply1: [][][2]float64{{{1, 5}, {1, 2}, {1, 3}, {1, 4}}},
			ply2: [][][2]float64{{{1, 2}, {1, 3}, {1, 4}, {1, 5}}},
			e:    true,
		},
		tc{
			// Simple test.
			ply1: [][][2]float64{},
			ply2: [][][2]float64{},
			e:    true,
		},
		tc{
			// Simple test.
			ply1: nil,
			ply2: [][][2]float64{},
			e:    true,
		},
		tc{
			// Simple test.
			ply1: nil,
			ply2: nil,
			e:    true,
		},
	).Run(fn)
}
