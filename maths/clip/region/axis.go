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
func (a *axis) Intersect(pt0, pt1 maths.Pointer) (x, y float64, doesIntersect bool) {
	var pt maths.Pt
	line := maths.Line{pt0.Point(), pt1.Point()}
	axisLine := a.AsLine()
	if pt, doesIntersect = maths.Intersect(axisLine, line); !doesIntersect {
		return pt.X, pt.Y, doesIntersect
	}
	// Now we need to make sure that the point in between the end points of the line.
	if !line.InBetween(pt) {
		return pt.X, pt.Y, false
	}
	// Clamp the point to the axis that it crosses…
	pt = axisLine.Clamp(pt)
	return pt.X, pt.Y, true
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

// IsInward returns weather the line described by pt1,pt2 is headed inward with respect to the axis.
func (a *axis) IsInward(pt1, pt2 maths.Pointer) bool {
	p1, p2 := pt1.Point(), pt2.Point()
	switch a.idx % 4 {
	// There is no change in x for case 0, and 2 as they are the y axises for either clockwise or counter clockwise.
	case 0:
		// check to see if the p1.X is less then a.pts[0].X and p2.X is greater then a.pts[0].X
		return p1.X <= a.pt0.X && a.pt1.X <= p2.X
	case 2:
		// for case two it's oppsite of case 1.
		return p2.X <= a.pt0.X && a.pt1.X <= p1.X
	//
	case 1:
		if a.winding.IsClockwise() {
			return p1.Y <= a.pt0.Y && a.pt1.Y <= p2.Y
		}
		return p2.Y <= a.pt0.Y && a.pt1.Y <= p1.Y
	case 3:
		if a.winding.IsClockwise() {
			return p2.Y <= a.pt0.Y && a.pt1.Y <= p1.Y
		}
		return p1.Y <= a.pt0.Y && a.pt1.Y <= p2.Y
	default:
		return false
	}
}
