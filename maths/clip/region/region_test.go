package region

import (
	"context"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola/container/singlelist/point/list"
	"github.com/terranodo/tegola/maths"
)

func TestNewRegion(t *testing.T) {
	cr := New(maths.Clockwise, maths.Pt{0, 0}, maths.Pt{10, 10})
	// Check the basic ones first.
	if cr.WindingOrder() != maths.Clockwise {
		t.Errorf("For winding order got: %v, expected clockwise.", cr.WindingOrder())
	}
	if !(maths.Pt{0, 0}).IsEqual(cr.Min()) ||
		!(maths.Pt{10, 10}).IsEqual(cr.Max()) {
		t.Errorf("For clockwise Min,Max got (%v,%v) expected ( (0 0),(10 10))", cr.Min(), cr.Max())
	}
	expectedPt := []maths.Pt{maths.Pt{0, 10}, maths.Pt{0, 0}, maths.Pt{10, 0}, maths.Pt{10, 10}}
	expectedDr := []bool{false, true, true, false}
	{
		ctx, cancel := context.WithCancel(context.Background())
		i := 0
		for e := range cr.Range(ctx) {
			pt := e.(maths.Pointer).Point()
			if !expectedPt[i].IsEqual(pt) {
				t.Errorf("For clockwise point %d got %v expected %v", i, pt, expectedPt[i])
			}
			if !expectedPt[i].IsEqual(cr.sentinelPoints[i].Point()) {
				t.Errorf("For clockwise sentinel point %d got %v expected %v", i, pt, expectedPt[i])
			}
			if cr.aDownOrRight[i] != expectedDr[i] {
				t.Errorf("For clockwise down or right  %d got %v expected %v", i, cr.aDownOrRight[i], expectedDr[i])
			}
			i++
		}
		cancel()
	}

	cr = New(maths.CounterClockwise, maths.Pt{0, 0}, maths.Pt{10, 10})
	// Check the basic ones first.
	if cr.WindingOrder() != maths.CounterClockwise {
		t.Errorf("For winding order got: %v, expected counter clockwise.", cr.WindingOrder())
	}
	if !(maths.Pt{0, 0}).IsEqual(cr.Min()) ||
		!(maths.Pt{10, 10}).IsEqual(cr.Max()) {
		t.Errorf("For counter clockwise Min,Max got (%v,%v) expected ( (0 0),(10 10))", cr.Min(), cr.Max())
	}
	expectedPt = []maths.Pt{maths.Pt{0, 0}, maths.Pt{0, 10}, maths.Pt{10, 10}, maths.Pt{10, 0}}
	expectedDr = []bool{true, true, false, false}
	{
		ctx, cancel := context.WithCancel(context.Background())
		i := 0
		for e := range cr.Range(ctx) {
			pt := e.(maths.Pointer).Point()
			if !expectedPt[i].IsEqual(pt) {
				t.Errorf("For counter clockwise point %d got %v expected %v", i, pt, expectedPt[i])
			}
			if !expectedPt[i].IsEqual(cr.sentinelPoints[i].Point()) {
				t.Errorf("For counter clockwise sentinel point %d got %v expected %v", i, pt, expectedPt[i])
			}
			if cr.aDownOrRight[i] != expectedDr[i] {
				t.Errorf("For counter clockwise down or right  %d got %v expected %v", i, cr.aDownOrRight[i], expectedDr[i])

			}
			i++
		}
		cancel()
	}

	a0 := cr.Axis(0)
	if a0.region != cr {
		t.Errorf("Expected Axis 0's region to be the same.")
	}
	if a0.idx != 0 {
		t.Errorf("Expected Axis 0's index to be 0, go: %v", a0.idx)
	}
	if a0.downOrRight != cr.aDownOrRight[0] {
		t.Errorf("Axis 0's downOrRight %v want: %v", a0.downOrRight, cr.aDownOrRight[0])
	}
	if a0.pt0 != cr.sentinelPoints[0] || a0.pt1 != cr.sentinelPoints[1] {
		t.Errorf("Axis 0's (%v,%v) want (%v,%v)", a0.pt0, a0.pt1, cr.sentinelPoints[0], cr.sentinelPoints[1])
	}
	if a0.winding != cr.winding {
		t.Errorf("Axis 0's winding (%v) want %v", a0.winding, cr.winding)
	}
	a0.PushInBetween(list.NewPoint(0, 5))
	expectedPt = []maths.Pt{maths.Pt{0, 0}, maths.Pt{0, 5}, maths.Pt{0, 10}, maths.Pt{10, 10}, maths.Pt{10, 0}}
	cr.ForEachPt(func(i int, pt maths.Pt) (cont bool) {
		if !expectedPt[i].IsEqual(pt) {
			t.Errorf("For counter clockwise point %d got %v expected %v", i, pt, expectedPt[i])
		}
		return true
	})
}

func TestRegion_UniqueIntersections(t *testing.T) {
	type testcase struct {
		line          maths.Line
		Intersections []Intersect
		winding       maths.WindingOrder
	}

	test := tbltest.Cases(
		testcase{ // 0 : Both internal.
			line: maths.Line{maths.Pt{25, 25}, maths.Pt{75, 75}},
		},
		testcase{ // 1 : Horizontal
			line: maths.Line{maths.Pt{50, 50}, maths.Pt{150, 50}},
			Intersections: []Intersect{
				{
					Pt:     maths.Pt{100, 50},
					Inward: false,
				},
			},
		},
		testcase{ // 2 : Horizontal Inward
			line: maths.Line{maths.Pt{150, 50}, maths.Pt{50, 50}},
			Intersections: []Intersect{
				{
					Pt:     maths.Pt{100, 50},
					Inward: true,
				},
			},
		},
		testcase{ // 3 : Vertical
			line: maths.Line{maths.Pt{50, 50}, maths.Pt{50, 150}},
			Intersections: []Intersect{
				{
					Pt:     maths.Pt{50, 100},
					Inward: false,
				},
			},
		},
		testcase{ // 4 : Vertical Inward
			line: maths.Line{maths.Pt{50, 150}, maths.Pt{50, 50}},
			Intersections: []Intersect{
				{
					Pt:     maths.Pt{50, 100},
					Inward: true,
				},
			},
		},
		testcase{ // 5 : Diagonal
			line: maths.Line{maths.Pt{50, 50}, maths.Pt{150, 150}},
			Intersections: []Intersect{
				{
					Pt:     maths.Pt{100, 100},
					Inward: false,
				},
				{
					Pt:     maths.Pt{100, 100},
					Inward: false,
				},
			},
		},
		testcase{ // 6 : Diagonal Inward
			line: maths.Line{maths.Pt{150, 150}, maths.Pt{50, 50}},
			Intersections: []Intersect{
				{
					Pt:     maths.Pt{100, 100},
					Inward: true,
				},
				{
					Pt:     maths.Pt{100, 100},
					Inward: true,
				},
			},
		},

		// Both points outside.

		testcase{ // 7 : Not Special case of 7
			line: maths.Line{maths.Pt{0, 0}, maths.Pt{50, 50}},
			Intersections: []Intersect{
				{
					Pt:     maths.Pt{0, 0},
					Inward: true,
				},
				{
					Pt:     maths.Pt{0, 0},
					Inward: true,
				},
			},
		},

		testcase{ // 8 : Vertical on Border
			line: maths.Line{maths.Pt{0, -50}, maths.Pt{0, 150}},
		},
		testcase{ // 9 : Horizontal on Border
			line: maths.Line{maths.Pt{-50, 0}, maths.Pt{150, 0}},
		},

		testcase{ // 10 : Vertical
			line: maths.Line{maths.Pt{50, -50}, maths.Pt{50, 150}},
			Intersections: []Intersect{
				{
					Pt:     maths.Pt{50, 0},
					Inward: true,
				},
				{
					Pt:     maths.Pt{50, 100},
					Inward: false,
				},
			},
		},
		testcase{ // 11 : Vertical all outside
			line: maths.Line{maths.Pt{-50, -50}, maths.Pt{-50, 150}},
		},
		testcase{ // 12 : Horizontal
			line: maths.Line{maths.Pt{-50, 50}, maths.Pt{150, 50}},
			Intersections: []Intersect{
				{
					Pt:     maths.Pt{0, 50},
					Inward: true,
				},
				{
					Pt:     maths.Pt{100, 50},
					Inward: false,
				},
			},
		},
		testcase{ // 13 : Horizontal all outside.
			line: maths.Line{maths.Pt{-50, -50}, maths.Pt{150, -50}},
		},
		testcase{ // 14 : diagonal
			line: maths.Line{maths.Pt{-50, 75}, maths.Pt{75, -50}},
			Intersections: []Intersect{
				{
					Pt:     maths.Pt{0, 25},
					Inward: true,
				},
				{
					Pt:     maths.Pt{25, 0},
					Inward: false,
				},
			},
		},
		testcase{ // 15 : tangential diagonal
			line: maths.Line{maths.Pt{-50, 50}, maths.Pt{50, -50}},
			Intersections: []Intersect{
				{
					Pt:     maths.Pt{0, 0},
					Inward: true,
				},
				{
					Pt:     maths.Pt{0, 0},
					Inward: false,
				},
			},
		},
		testcase{ // 16 : diagonal completely outside.
			line: maths.Line{maths.Pt{-50, 5}, maths.Pt{10, -50}},
		},

		testcase{ // 17 : diagonal through the center.
			line: maths.Line{maths.Pt{0, 100}, maths.Pt{100, 0}},
			Intersections: []Intersect{
				{
					Pt:     maths.Pt{0, 100},
					Inward: true,
				},
				{
					Pt:     maths.Pt{100, 0},
					Inward: false,
				},
				{
					Pt:     maths.Pt{100, 0},
					Inward: false,
				},
				{
					Pt:     maths.Pt{0, 100},
					Inward: true,
				},
			},
		},
		testcase{ // 18 : diagonal through the center.
			line: maths.Line{maths.Pt{-50, 50}, maths.Pt{100, 0}},
			Intersections: []Intersect{
				{
					Pt:     maths.Pt{0, 33.333333333333336},
					Inward: true,
				},
				{
					Pt:     maths.Pt{100, 0},
					Inward: false,
				},
				{
					Pt:     maths.Pt{100, 0},
					Inward: false,
					Idx:    2,
				},
			},
		},
	)
	//test.RunOrder = "19"
	test.Run(func(idx int, tc testcase) {
		r := New(tc.winding, maths.Pt{0, 0}, maths.Pt{100, 100})
		got, _, _ := r.Intersections(tc.line)
		if len(tc.Intersections) != len(got) {
			t.Errorf("Test(%v) incorrect number of intersections got %v [%#v] want %v", idx, len(got), got, len(tc.Intersections))
			return
		}
		for i, inter := range tc.Intersections {
			if !inter.Pt.IsEqual(got[i].Pt) || inter.Inward != got[i].Inward {
				t.Errorf("Test(%v) Incorrect Intersection (%v) got %#v want %#v", idx, i, got[i], inter)
			}
		}
	})
}
