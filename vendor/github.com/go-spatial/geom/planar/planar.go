package planar

import (
	"math"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/cmp"
)

// Rad is the factor to go from pi to radians
const Rad = math.Pi / 180

// PointLineDistanceFunc is the abstract method to get the distance from point
// to a line depending on projection
type PointLineDistanceFunc func(line [2][2]float64, point [2]float64) float64

// PerpendicularDistance  provides the distance between a line and a point in Euclidean space.
// ref: https://en.wikipedia.org/wiki/Distance_from_a_point_to_a_line#Line_defined_by_two_points
func PerpendicularDistance(line [2][2]float64, point [2]float64) float64 {

	deltaX := line[1][0] - line[0][0]
	deltaY := line[1][1] - line[0][1]
	deltaXSq := deltaX * deltaX
	deltaYSq := deltaY * deltaY

	num := math.Abs((deltaY * point[0]) - (deltaX * point[1]) + (line[1][0] * line[0][1]) - (line[1][1] * line[0][0]))
	denom := math.Sqrt(deltaYSq + deltaXSq)
	if denom == 0 {
		return 0
	}
	return num / denom
}

// Slope — finds the Slope of a line
func Slope(line [2][2]float64) (m, b float64, defined bool) {
	dx := line[1][0] - line[0][0]
	dy := line[1][1] - line[0][1]
	if dx == 0 || dy == 0 {
		// if dx == 0 then m == 0; and the intercept is y.
		// However if the lines are vertical then the slope is not defined.
		return 0, line[0][1], dx != 0
	}
	m = dy / dx
	b = line[0][1] - (m * line[0][0])
	return m, b, true
}

// IsPointOnLine checks if pt is on the lines l1, l2 by checking slope and intersect form
func IsPointOnLine(pt [2]float64, l1, l2 [2]float64) bool {
	m, b, defined := Slope([2][2]float64{l1, l2})
	switch {
	case !defined:
		// line is vertical, so if we the x values are the same it's on the line.
		return cmp.Float(pt[0], l1[0])
	case m == 0:
		// line is horizontal, so if the y values are the same it's on the line.
		return cmp.Float(pt[1], l1[1])
	default:
		y := (m * pt[0]) + b
		return cmp.Float(pt[1], y)
	}
}

// IsPointOnLineSegment checks if pt is on the line segment (seg)
func IsPointOnLineSegment(pt geom.Point, seg geom.Line) bool {
	// first order the x and y of the line.
	minx, miny, maxx, maxy := seg[0][0], seg[0][1], seg[1][0], seg[1][1]
	if minx > maxx {
		minx, maxx = maxx, minx
	}
	if miny > maxy {
		miny, maxy = maxy, miny
	}
	if minx > pt[0] || maxx < pt[0] || miny > pt[1] || maxy < pt[1] {
		// Outside the extent of the line
		return false
	}
	return IsPointOnLine(pt, seg[0], seg[1])
}

// PointOnLineAt will return a point on the given line at the distance from the
// origin of the line
func PointOnLineAt(ln geom.Line, distance float64) geom.Point {

	lineDist := math.Sqrt(ln.LengthSquared())
	ratio := distance / lineDist
	var x, y float64

	x = ln[0][0] + (ratio * (ln[1][0] - ln[0][0]))
	y = ln[0][1] + (ratio * (ln[1][1] - ln[0][1]))
	return geom.Point{x, y}
}
