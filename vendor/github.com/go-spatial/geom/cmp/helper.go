package cmp

type Lesser interface {
	// Is the number of elements in an Element
	Len() int

	// Less reports whether the element with
	// index i should sort before the element with index j.
	Less(i, j int) bool
}

// FindMinIdx given a slice will return the min index accourding to the Less function.
func FindMinIdx(ln Lesser) (min int) {
	for i := 1; i < ln.Len(); i++ {
		if ln.Less(i, min) {
			min = i
		}
	}
	return min
}

// XYLessPoint
func XYLessPoint(pt1, pt2 [2]float64) bool {
	if pt1[0] != pt2[0] {
		return pt1[0] < pt2[0]
	}
	return pt1[1] < pt2[1]
}

// FindMinPointIdx given a slice of points, it will return the index to the smallest point
// according to XYLessPoint
func FindMinPointIdx(ln [][2]float64) (min int) {
	for i := range ln[1:] {
		// Adjust for the slice.
		if XYLessPoint(ln[i+1], ln[min]) {
			min = i + 1
		}
	}
	return min
}

// RotateToLeftMostPoint will rotate the points in the linestring so that the smallest
// point (as defined by XYLessPoint) is the first point in the linestring.
func RotateToLeftMostPoint(ln [][2]float64) {
	if len(ln) == 0 {
		return
	}
	idx := FindMinPointIdx(ln)
	tmp := make([][2]float64, len(ln))
	copy(tmp, ln[idx:])
	copy(tmp[len(ln[idx:]):], ln)
	copy(ln, tmp)
}
