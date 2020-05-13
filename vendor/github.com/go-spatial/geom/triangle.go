package geom

import (
	"fmt"
	"math"
)

const tolerance = 0.000001

// Float64 compares two floats to see if they are within the given tolerance.
func cmpFloat(f1, f2 float64) bool {
	if math.IsInf(f1, 1) {
		return math.IsInf(f2, 1)
	}
	if math.IsInf(f2, 1) {
		return math.IsInf(f1, 1)
	}
	if math.IsInf(f1, -1) {
		return math.IsInf(f2, -1)
	}
	if math.IsInf(f2, -1) {
		return math.IsInf(f1, -1)
	}
	return math.Abs(f1-f2) < tolerance
}
func pointEqual(p1, p2 [2]float64) bool { return cmpFloat(p1[0], p2[0]) && cmpFloat(p1[1], p2[1]) }

// Triangle is a array representation of a geometry trinagle.
type Triangle [3][2]float64

// Center returns a point at the center of the triangle.
func (t Triangle) Center() (pt [2]float64) {
	pt[0] = (t[0][0] + t[1][0] + t[2][0]) / 3
	pt[1] = (t[0][1] + t[1][1] + t[2][1]) / 3
	return pt
}

// LinearRings returns the coordinates of the linear rings
func (t Triangle) LinearRings() [][][2]float64 {
	return [][][2]float64{t[:]}
}

// ThirdPoint takes 2 points and checks which point is the 3rd in the Triangle
func (t Triangle) ThirdPoint(p1, p2 [2]float64) [2]float64 {
	switch {
	case (pointEqual(t[0], p1) && pointEqual(t[1], p2)) ||
		(pointEqual(t[1], p1) && pointEqual(t[0], p2)):
		return t[2]
	case (pointEqual(t[0], p1) && pointEqual(t[2], p2)) ||
		(pointEqual(t[2], p1) && pointEqual(t[0], p2)):
		return t[1]
	default:
		return t[0]
	}
}

// NewTriangleFromPolygon takes the first three points from the outer ring of a polygon to create a triangle.
func NewTriangleFromPolygon(py [][][2]float64) Triangle {
	// Assume we are getting triangles from the function.
	if debug && len(py) != 1 {
		panic(fmt.Sprintf("Step   3 : assumption invalid for triangle. %v", py))
	}
	if debug && len(py[0]) < 3 {
		panic(fmt.Sprintf("Step   3 : assumption invalid for triangle. %v", py))
	}
	t := Triangle{py[0][0], py[0][1], py[0][2]}
	return t
}

// Area reaturns twice the area of the oriented triangle (a,b,c), i.e.
// the area is positive if the triangle is oriented counterclockwise.
func (t Triangle) Area() float64 {
	a, b, c := t[0], t[1], t[2]
	return (b[0]-a[0])*(c[1]-a[1]) - (b[1]-a[1])*(c[0]-a[0])
}

// NewTriangleContaining returns a triangle that is large enough to contain the
// given points
func NewTriangleContaining(pts ...Point) Triangle {
	const buff = 10
	ext := NewExtentFromPoints(pts...)
	tri, err := NewTriangleForExtent(ext, buff)
	if err != nil {
		panic(err)
	}
	return tri
}

func NewTriangleContainingPoints(pts ...[2]float64) Triangle {
	const buff = 10
	ext := NewExtent(pts...)
	tri, err := NewTriangleForExtent(ext, buff)
	if err != nil {
		panic(err)
	}
	return tri
}

func NewTriangleForExtent(ext *Extent, buff float64) (Triangle, error) {
	if ext == nil {
		return Triangle{EmptyPoint, EmptyPoint, EmptyPoint},
			fmt.Errorf("extent is nil")
	}

	xlen := ext[2] - ext[0]
	ylen := ext[3] - ext[1]
	x2len := xlen / 2

	nx := ext[0] - (x2len * buff) - buff
	cx := ext[0] + x2len
	xx := ext[2] + (x2len * buff) + buff

	ny := ext[1] - (ylen * buff) - buff
	xy := ext[3] + (2 * ylen * buff) + buff
	return Triangle{
		{nx, ny},
		{cx, xy},
		{xx, ny},
	}, nil

}
