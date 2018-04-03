package region

import (
	"errors"
	"fmt"

	"github.com/go-spatial/tegola/container/singlelist/point/list"
	"github.com/go-spatial/tegola/maths"
)

// IntersectionCode encodes weather the intersect point found has the following points.
// 1. Is there an intersection point.
// 2. If the there is an intersection point, is it inward bound for that axises
// 3. Is it contained within the region.
type IntersectionCode uint8

type IntersectionPt struct {
	Code IntersectionCode
}

type Axis struct {
	region      *Region
	idx         int
	downOrRight bool
	pt0, pt1    *list.Pt
	winding     maths.WindingOrder
}

func (a *Axis) GoString() string {
	return fmt.Sprintf("[%v,%v]-(%v)-(%v){%v}", a.pt0, a.pt1, a.downOrRight, a.idx, a.region.GoString())
}

// Next returns the next Axis, or nil if there aren't anymore.
func (a *Axis) Next() *Axis {
	if a.idx == 3 {
		return nil
	}
	return a.region.Axis(a.idx + 1)
}

// PushInBetween inserts the pt into the region list on this Axis.
func (a *Axis) PushInBetween(pt list.ElementerPointer) bool {
	return a.region.PushInBetween(a.pt0, a.pt1, pt)
}

// AsLine returns the Axis as a line.
func (a *Axis) AsLine() maths.Line { return maths.Line{a.pt0.Point(), a.pt1.Point()} }

// Intersect finds the intersections point if one exists with the line described by pt0,pt1. This point will be clamped to the line of the clipping region.
func (a *Axis) Intersect(line maths.Line) (pt maths.Pt, doesIntersect bool) {
	axisLine := a.AsLine()
	var ok bool
	pt, ok = maths.Intersect(axisLine, line)
	if !ok {
		return pt, ok
	}

	// Now we need to make sure that the point in between the end points of the line.
	if !line.InBetween(pt) {
		return pt, false
	}
	// Make sure the pt is on the Axis as well
	if !axisLine.ExInBetween(pt) {

		// We need to check if the intersecting line is parallel to the axis.

		if (axisLine.IsHorizontal() && line.IsVertical()) ||
			(axisLine.IsVertical() && line.IsHorizontal()) ||
			(!axisLine.InBetween(pt)) {
			return pt, false
		}

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

// inside determines if the pt provided on the Inside side of the axises.
// For example give a cLockwise region and axises 0
//   the area to the left of and including the axis is outside, while the area to the right is inside.
func (a *Axis) inside(pt maths.Pt) bool {

	/*

		Legend:
			I = Inside
			o = Outside
			|: Axis 0,2
			-: Axis 1,3

		For Axis 0:
		   I | O
		For Axis 2:
		   O | I
		For Axis 3 clockwise, or 1 Counter:
		     I
		     -
		     O
		For Axis 1 clockwise or 3 Counter:
		     O
		     -
		     I

	*/

	switch a.idx % 4 {
	case 0:
		return pt.X > a.pt0.X
	case 1:
		if a.winding.IsClockwise() {
			return pt.Y > a.pt0.Y
		}
		return pt.Y < a.pt0.Y
	case 2:
		return pt.X < a.pt0.X
	case 3:
		if a.winding.IsClockwise() {
			return pt.Y < a.pt0.Y
		}
		return pt.Y > a.pt0.Y
	}
	return false
}

//func (a *Axis) outside(pt maths.Pt) bool { return !a.inside(pt) }

var ErrNoDirection = errors.New("Line does not have direction on that coordinate.")

type PlacementCode uint8

const (
	PCInside      PlacementCode = 0x00               // 0000
	PCBottom                    = 0x01               // 0001
	PCTop                       = 0x02               // 0010
	PCRight                     = 0x04               // 0100
	PCLeft                      = 0x08               // 1000
	PCTopRight                  = PCTop | PCRight    // 0110
	PCTopLeft                   = PCTop | PCLeft     // 1010
	PCBottomRight               = PCBottom | PCRight // 0101
	PCBottomLeft                = PCBottom | PCLeft  // 1001

	PCAllAround = PCTop | PCLeft | PCRight | PCBottom // 1111
)

// Placement returns where according to the region axis the point is.
//
// 		                     0010
//
// 			       pt   ______  pt
// 			           |      |
// 			    1000   | 0000 |    0100
// 			           |______|
// 			       pt           pt
// 			             0001
//
func (a *Axis) Placement(pt maths.Pt) PlacementCode {

	idx := a.idx % 4
	switch {
	case idx == 0 && pt.X <= a.pt0.X:
		return PCLeft
	case idx == 2 && pt.X >= a.pt0.X:
		return PCRight

	case ((a.winding.IsClockwise() && a.idx == 3) || a.idx == 1) && pt.Y <= a.pt0.Y:
		return PCTop
	case ((a.winding.IsClockwise() && a.idx == 1) || a.idx == 3) && pt.Y >= a.pt0.Y:
		return PCBottom
	default:
		return PCInside
	}

}

// IsInward returns weather the line described by pt1,pt2 is headed inward with respect to the Axis.
func (a *Axis) IsInward(line maths.Line) (bool, error) {
	p1, p2 := line[0], line[1]

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
				             0010
				              3
				      0pt   ______  3pt
				           |      |
				    1000 0 | 0000 | 2  0100
				           |______|
				      1pt     1     2pt
				             0001
	*/

	dx := p2.X - p1.X
	dy := p2.Y - p1.Y
	//log.Println("dx:", dx, "dy:", dy)
	idx := a.idx % 4
	switch idx {
	case 0, 2:
		if dx == 0 {
			return false, ErrNoDirection
		}
		if idx == 0 {
			return dx > 0, nil
		}
		return dx < 0, nil

	case 1, 3:
		if dy == 0 {
			return false, ErrNoDirection
		}
		// Flip the index.
		if a.winding.IsCounterClockwise() {
			if idx == 1 {
				idx = 3
			} else {
				idx = 1
			}
		}
		if idx == 1 {
			return dy > 0, nil
		}
		return dy < 0, nil
	}
	return false, ErrNoDirection
}
