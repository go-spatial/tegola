package convert

import "github.com/go-spatial/tegola/maths"

// TODO (gdey): Remove once conversion is over.
func FromMathPoint(pt ...maths.Pt) (gpts [][2]float64) {
	gpts = make([][2]float64, len(pt))
	for i := range pt {
		gpts[i] = [2]float64{pt[i].X, pt[i].Y}
	}
	return gpts
}
