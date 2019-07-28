package simplify

import (
	"log"

	"github.com/go-spatial/tegola/maths"
)

// DouglasPeucker is a geometry simplifcation routine
// https://en.wikipedia.org/wiki/Ramer%E2%80%93Douglas%E2%80%93Peucker_algorithm
func DouglasPeucker(points []maths.Pt, tolerance float64) []maths.Pt {
	log.Printf("tolerance: %d", tolerance)

	if tolerance <= 0 || len(points) <= 2 {
		return points
	}

	epsilon := tolerance * tolerance

	// find the maximum distance from the end points.
	l := maths.Line{points[0], points[len(points)-1]}
	dmax := 0.0
	idx := 0
	for i := 1; i < len(points)-2; i++ {
		d := l.DistanceFromPoint(points[i])
		if d > dmax {
			dmax = d
			idx = i
		}
	}

	if dmax > epsilon {
		rec1 := DouglasPeucker(points[0:idx], epsilon)
		rec2 := DouglasPeucker(points[idx:], epsilon)

		newpts := append(rec1, rec2...)

		return newpts
	}

	return []maths.Pt{points[0], points[len(points)-1]}
}
