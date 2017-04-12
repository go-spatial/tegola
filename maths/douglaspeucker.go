package maths

func DouglasPeucker(points []Pt, tolerence float64) []Pt {
	if tolerence <= 0 || len(points) <= 2 {
		return points
	}

	epsilon := tolerence * tolerence

	// find the maximum distance from the end points.
	l := Line{points[0], points[len(points)-1]}
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
		rec2 := DouglasPeucker(points[idx:len(points)-1], epsilon)

		newpts := append(rec1, rec2...)
		return newpts

	}

	return []Pt{points[0], points[len(points)-1]}
}
