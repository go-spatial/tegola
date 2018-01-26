package points

import (
	"math"
	"sort"

	"github.com/terranodo/tegola/maths"
)

// Extent describes a retangular region.
type Extent [2][2]float64

func (e Extent) TopLeft() [2]float64    { return e[0] }
func (e Extent) TopRight() [2]float64   { return [2]float64{e[1][0], e[0][1]} }
func (e Extent) LowerRight() [2]float64 { return e[1] }
func (e Extent) LowerLeft() [2]float64  { return [2]float64{e[0][0], e[1][1]} }

// Verticies return the verticies of the Extent.
func (e Extent) Verticies() [][2]float64 {
	return [][2]float64{
		e.TopLeft(),
		e.TopRight(),
		e.LowerRight(),
		e.LowerLeft(),
	}
}

// Edges return in clockwise order the edges that make up this extent. And edge is
// a line made up of two points.
func (e Extent) Edges() [][2][2]float64 {
	return [][2][2]float64{
		{e.TopLeft(), e.TopRight()},
		{e.TopRight(), e.LowerRight()},
		{e.LowerRight(), e.LowerLeft()},
		{e.LowerLeft(), e.TopLeft()},
	}
}

// LREdges returns the edges of the region so that the upper left most edges to the lower right edges are returned. Basically sorting by the x values first then the y values.
func (e Extent) LREdges() [][2][2]float64 {
	return [][2][2]float64{
		{e.TopLeft(), e.TopRight()},
		{e.TopLeft(), e.LowerLeft()},
		{e.LowerLeft(), e.LowerRight()},
		{e.TopRight(), e.LowerRight()},
	}
}

// Contains return weather the point is contained by the Extent.
func (e Extent) Contains(pt [2]float64) bool {
	return e[0][0] <= pt[0] && pt[0] <= e[1][0] &&
		e[0][1] <= pt[1] && pt[1] <= e[1][1]
}

// ContainsPoints returns weather all the given points are contained by the Extent.
func (e Extent) ContainsPoints(pts ...[2]float64) bool {
	for i := range pts {
		if !e.Contains(pts[i]) {
			return false
		}
	}
	return true
}

// ContainsLine returns weather both points of the line are contained by the Extent.
func (e Extent) ContainsLine(line [2][2]float64) bool {
	return e.Contains(line[0]) && e.Contains(line[1])
}
func (e Extent) InclusiveContainsLine(line [2][2]float64) bool {
	return e.Contains(line[0]) || e.Contains(line[1])
}

// ContainsExtent returns weather the points of the second extent are containted by the first extent.
func (e Extent) ContainsExtent(ee Extent) bool { return e.Contains(ee[1]) && e.Contains(ee[1]) }

// Area returns the are of the Extent.
func (e Extent) Area() float64 { return math.Abs(e[1][0]-e[0][0]) * (e[1][1] - e[0][1]) }

// IntersectPt returns the intersect point if one exists.
func (e Extent) IntersectPt(ln [2][2]float64) (pts [][2]float64, ok bool) {
	lln := maths.NewLineWith2Float64(ln)
loop:
	for _, edge := range e.Edges() {
		eln := maths.NewLineWith2Float64(edge)
		if pt, ok := maths.Intersect(eln, lln); ok {
			// Only add if the point is actually on the line segment.
			if !eln.InBetween(pt) || !lln.InBetween(pt) {
				continue loop
			}

			// Only add if we have not see this point.
			for i := range pts {
				if pts[i][0] == pt.X && pts[i][1] == pt.Y {
					continue loop
				}
			}
			pts = append(pts, [2]float64{pt.X, pt.Y})
		}
	}
	sort.Sort(byxy(pts))
	return pts, len(pts) > 0
}

type byxy [][2]float64

func (b byxy) Len() int      { return len(b) }
func (b byxy) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byxy) Less(i, j int) bool {
	if b[i][0] != b[j][0] {
		return b[i][0] < b[j][0]
	}
	return b[i][1] < b[j][1]
}
