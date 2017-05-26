package maths

//https://en.wikipedia.org/wiki/Ramer%E2%80%93Douglas%E2%80%93Peucker_algorithm

func DouglasPeucker(points []Pt, epsilon float64, simplify bool) []Pt {
	//log.Println("In DP simplify")
	if epsilon <= 0 || len(points) <= 2 || !simplify {
		//log.Println("Not doing simplification")
		return points
	}

	//epsilon := tolerence * tolerence
	// If the last and first point is the same.
	if points[0].IsEqual(points[len(points)-1]) {
		points = points[:len(points)-1]
	}

	// find the maximum distance from the end points.
	l := Line{points[0], points[len(points)-1]}
	dmax := 0.0
	idx := 0
	for i := 1; i < len(points)-2; i++ {
		d := l.DistanceFromPoint(points[i])
		//log.Printf("d(%v) > dmax(%v)", d, dmax)
		if d > dmax {
			dmax = d
			idx = i
		}
	}
	//log.Printf("dmax(%v) > epsilon(%v)", dmax, epsilon)
	if dmax > epsilon {
		rec1 := DouglasPeucker(points[0:idx], epsilon, simplify)
		rec2 := DouglasPeucker(points[idx:], epsilon, simplify)
		//rec1 = rec1[:len(rec1)-1]
		newpts := append(rec1, rec2...)
		//log.Printf("Dmax(%v) > epsilon(%v) returns len(pts):%v", dmax, epsilon, len(newpts))
		return newpts
	}
	//log.Println("Just returning the endpoints.")
	return []Pt{points[0], points[len(points)-1]}
}
