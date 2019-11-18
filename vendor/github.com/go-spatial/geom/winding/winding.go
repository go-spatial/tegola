// Package winding provides primitives for determining the winding order of a
// set of points
package winding

import (
	"log"

	"github.com/go-spatial/geom"
)

// Winding is the clockwise direction of a set of points.
type Winding uint8

const (

	// Clockwise indicates that the winding order is in the clockwise direction
	Clockwise Winding = 0
	// Colinear indicates that the points are colinear to each other
	Colinear Winding = 1
	// CounterClockwise indicates that the winding order is in the counter clockwise direction
	CounterClockwise Winding = 2

	// Collinear alternative spelling of Colinear
	Collinear = Colinear
)

// String implements the stringer interface
func (w Winding) String() string {
	switch w {
	case Clockwise:
		return "clockwise"
	case Colinear:
		return "colinear"
	case CounterClockwise:
		return "counter clockwise"
	default:
		return "unknown"
	}
}

// IsClockwise checks if winding is clockwise
func (w Winding) IsClockwise() bool { return w == Clockwise }

// IsCounterClockwise checks if winding is counter clockwise
func (w Winding) IsCounterClockwise() bool { return w == CounterClockwise }

// IsColinear check if the points are colinear
func (w Winding) IsColinear() bool { return w == Colinear }

// Not returns the inverse of the winding, clockwise <-> counter-clockwise, colinear is it's own
// inverse
func (w Winding) Not() Winding {
	switch w {
	case Clockwise:
		return CounterClockwise
	case CounterClockwise:
		return Clockwise
	default:
		return w
	}
}

// Orient will take the points and calculate the Orientation of the points. by
// summing the normal vectors. It will return 0 of the given points are colinear
// or 1, or -1 for clockwise and counter clockwise depending on the direction of
// the y axis. If the y axis increase as you go up on the graph then clockwise will
// be -1, otherwise it will be 1; vice versa for counter-clockwise.
func Orient(pts ...[2]float64) int8 {
	if len(pts) < 3 {
		return 0
	}
	var (
		sum = 0.0
		dop = 0.0
		li  = len(pts) - 1
	)

	if debug {
		log.Printf("pts: %v", pts)
	}
	for i := range pts {
		dop = (pts[li][0] * pts[i][1]) - (pts[i][0] * pts[li][1])
		sum += dop
		if debug {
			log.Printf("sum(%v,%v): %g  -- %g", li, i, sum, dop)
		}
		li = i
	}
	switch {
	case sum == 0:
		return 0
	case sum < 0:
		return -1
	default:
		return 1
	}
}

// Orientation returns the orientation of the set of the points given the
// direction of the positive values of the y axis
func Orientation(yPositiveDown bool, pts ...[2]float64) Winding {
	mul := int8(1)
	if yPositiveDown {
		mul = -1
	}
	switch mul * Orient(pts...) {
	case 0:
		return Colinear
	case 1:
		return Clockwise
	default: // -1
		return CounterClockwise
	}
}

// Order configures how the orientation of a set of points is determined
type Order struct {
	YPositiveDown bool
}

// OfPoints returns the winding of the given points
func (order Order) OfPoints(pts ...[2]float64) Winding {
	return Orientation(order.YPositiveDown, pts...)
}

// OfInt64Points returns the winding of the given int64 points
func (order Order) OfInt64Points(ipts ...[2]int64) Winding {
	pts := make([][2]float64, len(ipts))
	for i := range ipts {
		pts[i] = [2]float64{
			float64(ipts[i][0]),
			float64(ipts[i][1]),
		}
	}
	return Orientation(order.YPositiveDown, pts...)
}

// OfGeomPoints returns the winding of the given geom points
func (order Order) OfGeomPoints(points ...geom.Point) Winding {
	pts := make([][2]float64, len(points))
	for i := range points {
		pts[i] = [2]float64(points[i])
	}
	return order.OfPoints(pts...)
}

// RectifyPolygon will make sure that the rings are of the correct orientation, if not it will reverse them
// Colinear rings are dropped
func (order Order) RectifyPolygon(plyg2r [][][2]float64) [][][2]float64 {
	plyg := make([][][2]float64, 0, len(plyg2r))
	reverse := func(idx int) {
		for i := len(plyg[idx])/2 - 1; i >= 0; i-- {
			opp := len(plyg[idx]) - 1 - i
			plyg[idx][i], plyg[idx][opp] = plyg[idx][opp], plyg[idx][i]
		}
	}

	// Let's make sure each of the rings have the correct windingorder.

	for i := range plyg2r {

		wo := order.OfPoints(plyg2r[i]...)

		// Drop collinear rings
		if wo.IsColinear() {
			if i == 0 {
				return nil
			}
			continue
		}

		plyg = append(plyg, plyg2r[i])

		if (i == 0 && wo.IsCounterClockwise()) || (i != 0 && wo.IsClockwise()) {
			// 0 ring should be clockwise.
			// all others should be conterclockwise
			// reverse the ring.
			reverse(len(plyg) - 1)
		}
	}
	return plyg
}

// Clockwise returns a clockwise winding
func (Order) Clockwise() Winding { return Clockwise }

// CounterClockwise returns a counter clockwise winding
func (Order) CounterClockwise() Winding { return CounterClockwise }

// Colinear returns a colinear winding
func (Order) Colinear() Winding { return Colinear }

// Collinear is a alias for colinear
func (Order) Collinear() Winding { return Colinear }

// OfPoints returns the winding order of the given points
func OfPoints(pts ...[2]float64) Winding { return Order{}.OfPoints(pts...) }

// OfGeomPoints is the same as OfPoints, just a convenience to unwrap geom.Point
func OfGeomPoints(points ...geom.Point) Winding { return Order{}.OfGeomPoints(points...) }
