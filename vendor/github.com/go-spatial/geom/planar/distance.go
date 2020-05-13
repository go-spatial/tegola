package planar

import (
	"math"

	"github.com/go-spatial/geom"
)

// PointDistance returns the euclidean distance between two points.
func PointDistance(p1 geom.Pointer, p2 geom.Pointer) float64 {
	return math.Sqrt(PointDistance2(p1, p2))
}

/*
PointDistance2 returns the euclidean distance between two points squared.

This can be a useful optimization in some routines where d^2 is good
enough.
*/
func PointDistance2(p1 geom.Pointer, p2 geom.Pointer) float64 {
	v1 := p1.XY()[0] - p2.XY()[0]
	v2 := p1.XY()[1] - p2.XY()[1]
	return v1*v1 + v2*v2
}

/*
DistanceToLineSegment calculates the distance from point p to line segment
v, w.

Taken from: https://stackoverflow.com/questions/849211/shortest-distance-between-a-point-and-a-line-segment
*/
func DistanceToLineSegment(p geom.Pointer, v geom.Pointer, w geom.Pointer) float64 {

	// note that this is intentionally the distance^2, not distance.
	l2 := PointDistance2(v, w)
	if l2 == 0 {
		return PointDistance(p, v)
	}

	px := p.XY()[0]
	py := p.XY()[1]
	vx := v.XY()[0]
	vy := v.XY()[1]
	wx := w.XY()[0]
	wy := w.XY()[1]

	t := ((px-vx)*(wx-vx) + (py-vy)*(wy-vy)) / l2
	t = math.Max(0, math.Min(1, t))
	return PointDistance(p, geom.Point{vx + t*(wx-vx), vy + t*(wy-vy)})
}
