package region

import (
	"errors"

	"fmt"

	"github.com/terranodo/tegola/container/list/point/list"
	"github.com/terranodo/tegola/maths"
)

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
	if pt, doesIntersect = maths.Intersect(axisLine, line); !doesIntersect {
		return pt, doesIntersect
	}
	// Now we need to make sure that the point in between the end points of the line.
	if !line.InBetween(pt) {
		return pt, false
	}
	// Make sure the pt is on the Axis as well
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
func (a *Axis) staticCoord() float64 {
	switch a.idx % 4 {
	case 0, 2:
		return a.pt0.X
	case 1, 3:
		return a.pt0.Y
	}
	return 0
}

func (a *Axis) inside(pt maths.Pt) bool {

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
func (a *Axis) outside(pt maths.Pt) bool { return !a.inside(pt) }

var ErrNoDirection = errors.New("Line does not have direction on that coordinate.")

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

					3
				0pt   _____  3pt
				     |     |
				   0 |     | 2
				     |_____|
				1pt     1    2pt
	*/

	dx := p2.X - p1.X
	dy := p2.Y - p1.Y
	//log.Println("dx:", dx, "dy:", dy)
	switch a.winding {
	case maths.Clockwise:
		switch a.idx % 4 {
		case 0:
			if dx == 0 {
				return false, ErrNoDirection
			}
			return dx >= 0, nil
		case 2:
			if dx == 0 {
				return false, ErrNoDirection
			}
			return dx <= 0, nil
		case 1:
			if dy == 0 {
				return false, ErrNoDirection
			}
			return dy >= 0, nil
		case 3:
			if dy == 0 {
				return false, ErrNoDirection
			}
			return dy <= 0, nil
		}
	case maths.CounterClockwise:
		switch a.idx % 4 {
		case 0:
			if dx == 0 {
				return false, ErrNoDirection
			}
			return dx >= 0, nil
		case 1:
			if dy == 0 {
				return false, ErrNoDirection
			}
			return dy <= 0, nil
		case 2:
			if dx == 0 {
				return false, ErrNoDirection
			}
			return dx <= 0, nil
		case 3:
			if dy == 0 {
				return false, ErrNoDirection
			}
			return dy >= 0, nil
		}
	}
	return false, ErrNoDirection
}
