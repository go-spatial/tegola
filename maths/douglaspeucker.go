package maths

func DouglasPeucker(points []Pt, epsilon float64) []Pt {
	if epsilon <= 0 {
		return points
	}
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
		return append(rec1, rec2...)
	}
	return []Pt{points[0], points[len(points)-1]}
}
