package region

import (
	"github.com/terranodo/tegola/container/list/point/list"
	"github.com/terranodo/tegola/maths"
)

type axis struct {
	region      *Region
	idx         int
	downOrRight bool
	pt0, pt1    *list.Pt
	winding     maths.WindingOrder
}

// Next returns the next axis, or nil if there aren't anymore.
func (a *axis) Next() *axis {
	if a.idx == 3 {
		return nil
	}
	return a.region.Axis(a.idx + 1)
}

// PushInBetween inserts the pt into the region list on this axis.
func (a *axis) PushInBetween(pt list.ElementerPointer) bool {
	return a.region.PushInBetween(a.pt0, a.pt1, pt)
}

// AsLine returns the axis as a line.
func (a *axis) AsLine() maths.Line { return maths.Line{a.pt0.Point(), a.pt1.Point()} }

// Intersect finds the intersections point if one exists with the line described by pt0,pt1. This point will be clamped to the line of the clipping region.
func (a *axis) Intersect(line maths.Line) (pt maths.Pt, doesIntersect bool) {
	axisLine := a.AsLine()
	if pt, doesIntersect = maths.Intersect(axisLine, line); !doesIntersect {
		return pt, doesIntersect
	}
	// Now we need to make sure that the point in between the end points of the line.
	if !line.InBetween(pt) {
		return pt, false
	}
	// Make sure the pt is on the axis as well
	if !axisLine.InBetween(pt) {
		return pt, false
	}
	return pt, true
}

/*
  Winding order:

  Clockwise

		        1
		1pt   _____  2pt
		     |     |
		   0 |     | 2
		     |_____|
		0pt     3    3pt

  Counter Clockwise

		        3
		0pt   _____  3pt
		     |     |
		   0 |     | 2
		     |_____|
		1pt     1    2pt
*/
func (a *axis) staticCoord() float64 {
	switch a.idx % 4 {
	case 0, 2:
		return a.pt0.X
	case 1, 3:
		return a.pt0.Y
	}
	return 0
}

func (a *axis) inside(pt maths.Pt) bool {

	switch a.idx % 4 {
	case 0:
		return pt.X >= a.staticCoord()
	case 1:
		if a.winding.IsClockwise() {
			return pt.Y >= a.staticCoord()
		}
		return pt.Y <= a.staticCoord()
	case 2:
		return pt.X <= a.staticCoord()
	case 3:
		if a.winding.IsClockwise() {
			return pt.Y <= a.staticCoord()
		}
		return pt.Y >= a.staticCoord()
	}
	return false
}
func (a *axis) outside(pt maths.Pt) bool { return !a.inside(pt) }

// IsInward returns weather the line described by pt1,pt2 is headed inward with respect to the axis.
func (a *axis) IsInward(line maths.Line) bool {
	p1, p2 := line[0], line[1]
	p1out, p2in := a.outside(p1), a.inside(p2)
	return p1out && p2in
}
