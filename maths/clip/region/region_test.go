package region

import (
	"testing"

	"github.com/terranodo/tegola/container/list/point/list"
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
	for p, i := cr.Front(), 0; p != nil; p, i = p.Next(), i+1 {
		pt := p.(maths.Pointer).Point()
		if !expectedPt[i].IsEqual(pt) {
			t.Errorf("For clockwise point %d got %v expected %v", i, pt, expectedPt[i])
		}
		if !expectedPt[i].IsEqual(cr.sentinelPoints[i].Point()) {
			t.Errorf("For clockwise sentinel point %d got %v expected %v", i, pt, expectedPt[i])
		}
		if cr.aDownOrRight[i] != expectedDr[i] {
			t.Errorf("For clockwise down or right  %d got %v expected %v", i, cr.aDownOrRight[i], expectedDr[i])

		}
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
	for p, i := cr.Front(), 0; p != nil; p, i = p.Next(), i+1 {
		pt := p.(maths.Pointer).Point()
		if !expectedPt[i].IsEqual(pt) {
			t.Errorf("For counter clockwise point %d got %v expected %v", i, pt, expectedPt[i])
		}
		if !expectedPt[i].IsEqual(cr.sentinelPoints[i].Point()) {
			t.Errorf("For counter clockwise sentinel point %d got %v expected %v", i, pt, expectedPt[i])
		}
		if cr.aDownOrRight[i] != expectedDr[i] {
			t.Errorf("For counter clockwise down or right  %d got %v expected %v", i, cr.aDownOrRight[i], expectedDr[i])

		}
	}

	a0 := cr.Axis(0)
	if a0.region != cr {
		t.Errorf("Expected axis 0's region to be the same.")
	}
	if a0.idx != 0 {
		t.Errorf("Expected axis 0's index to be 0, go: %v", a0.idx)
	}
	if a0.downOrRight != cr.aDownOrRight[0] {
		t.Errorf("axis 0's downOrRight %v want: %v", a0.downOrRight, cr.aDownOrRight[0])
	}
	if a0.pt0 != cr.sentinelPoints[0] || a0.pt1 != cr.sentinelPoints[1] {
		t.Errorf("axis 0's (%v,%v) want (%v,%v)", a0.pt0, a0.pt1, cr.sentinelPoints[0], cr.sentinelPoints[1])
	}
	if a0.winding != cr.winding {
		t.Errorf("axis 0's winding (%v) want %v", a0.winding, cr.winding)
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
