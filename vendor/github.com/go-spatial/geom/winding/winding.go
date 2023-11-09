// Package winding provides primitives for determining the winding order of a
// set of points
package winding

import (
	"fmt"
	"log"
	"math"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/cmp"
	"github.com/go-spatial/geom/encoding/wkt"
)

// Winding is the clockwise direction of a set of points.
type Winding int8

const (

	// Clockwise indicates that the winding order is in the clockwise direction
	Clockwise Winding = -1
	// Colinear indicates that the points are colinear to each other
	Colinear Winding = 0
	// CounterClockwise indicates that the winding order is in the counter clockwise direction
	CounterClockwise Winding = 1

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
		return fmt.Sprintf("unknown(%v)", int8(w))
	}
}

func (w Winding) ShortString() string {
	switch w {
	case Clockwise:
		return "⟳"
	case Colinear:
		return "O"
	case CounterClockwise:
		return "⟲"
	default:
		return fmt.Sprintf("{%v}", int8(w))
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
	return Winding(-1 * int8(w))
}

// reference: https://stackoverflow.com/questions/1165647/how-to-determine-if-a-list-of-polygon-points-are-in-clockwise-order

/*
func shoelace(pts ...[2]float64) (sum float64) {
	for i := range pts {
		j := (i + 1) % len(pts)
		prd := (pts[j][0] - pts[i][0]) * (pts[j][1] + pts[i][1])
		sum += prd
		if debug {
			log.Printf("sum(%v,%v): %g  -- %g", i, j, sum, prd)
		}
	}
	if debug {
		log.Printf("sum:%g", sum)
	}
	return sum
}

func openlayers2(pts ...[2]float64) (area float64) {
	for i := range pts {
		j := (i + 1) % len(pts)
		area += pts[i][0] * pts[j][1]
		area -= pts[j][0] * pts[i][1]

		if debug {
			log.Printf("area(%v,%v): %g", i, j, area)
		}
	}
	if debug {
		log.Printf("area:%g", area)
	}
	return area

}
*/
func xprod(pts ...[2]float64) float64 {
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
	return sum

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
	sum := xprod(pts...)
	if sum == 0.0 {
		return 0 // colinear
	}
	if math.Signbit(sum) {
		return -1 // counter clockwise
	}
	return 1 // clockwise
}

// Orientation returns the orientation of the set of the points given the
// direction of the positive values of the y axis
func Orientation(yPositiveDown bool, pts ...[2]float64) Winding {

	if len(pts) < 3 {
		return Colinear
	}

	mul := int8(1)
	if yPositiveDown {
		mul = -1
	}

	adjusted := make([][2]float64, len(pts))
	for i := range pts {
		adjusted[i] = [2]float64{pts[i][0] - pts[0][0], pts[i][1] - pts[0][1]}
	}

	return Winding(mul * Orient(adjusted...))
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

func (order Order) ThreePointsAreColinear(pt1, pt2, pt3 geom.Point) bool {

	// Ware are using the area of the triangle
	a := pt1[0] - pt2[0]
	b := pt2[0] - pt3[0]
	c := pt1[1] - pt2[1]
	d := pt2[1] - pt3[1]
	e := a * d
	f := b * c

	area := .5 * (e - f)
	if debug {
		log.Printf("points: %v %v %v", wkt.MustEncode(pt1), wkt.MustEncode(pt2), wkt.MustEncode(pt3))
		log.Println("area of triangle: ", area)
	}
	return cmp.Float(area, 0.0)

}

// OfPoints returns the winding order of the given points
func OfPoints(pts ...[2]float64) Winding { return Order{}.OfPoints(pts...) }

// OfGeomPoints is the same as OfPoints, just a convenience to unwrap geom.Point
func OfGeomPoints(points ...geom.Point) Winding { return Order{}.OfGeomPoints(points...) }
