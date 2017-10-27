package points

import (
	"math"

	"github.com/terranodo/tegola/maths"
)

func SinArea(pts []maths.Pt) (a float64) {
	if len(pts) < 3 {
		return a
	}
	for i := range pts[:len(pts)-1] {
		a += (pts[i].X * pts[i+1].Y) - (pts[i+1].X * pts[i].Y)
	}
	return a / 2
}

func Area(pts []maths.Pt) (a float64) {
	return math.Abs(SinArea(pts))
}

func Centroid(pts []maths.Pt) (center maths.Pt) {
	if len(pts) == 0 {
		return center
	}
	if len(pts) == 1 {
		return pts[0]
	}
	// TODO: Optimize for small rings and lines.
	var a, aa, cx, cy float64
	for i := range pts[:len(pts)-1] {
		pt, npt := pts[i], pts[i+1]
		aa = (pt.X * npt.Y) - (npt.X * pt.Y)
		a += aa
		cx += (pt.X + npt.X) * aa
		cy += (pt.Y + npt.Y) * aa
	}

	cx = cx / (3 * a)
	cy = cy / (3 * a)
	return maths.Pt{cx, cy}
}
