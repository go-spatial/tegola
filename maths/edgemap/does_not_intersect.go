package edgemap

// DoesIntersect does a quick intersect check using the saddle method.
func doesNotIntersect(s1x0, s1y0, s1x1, s1y1, s2x0, s2y0, s2x1, s2y1 float64) bool {

	var swap float64
	//var deltaS1X, deltaS1Y, deltaS2X, deltaS2Y float64

	// Put line 1 points in order.
	if s1x0 > s1x1 {

		swap = s1x0
		s1x0 = s1x1
		s1x1 = swap

		swap = s1y0
		s1y0 = s1y1
		s1y1 = swap
	} else {
		if s1x0 == s1x1 && s1y0 > s1y1 {
			swap = s1x0
			s1x0 = s1x1
			s1x1 = swap

			swap = s1y0
			s1y0 = s1y1
			s1y1 = swap
		}
	}
	// Put line 2 points in order.
	if s2x0 > s2x1 {
		swap = s2x0
		s2x0 = s2x1
		s2x1 = swap

		swap = s2y0
		s2y0 = s2y1
		s2y1 = swap
	} else {
		if s2x0 == s2x1 && s2y0 > s2y1 {
			swap = s2x0
			s2x0 = s2x1
			s2x1 = swap

			swap = s2y0
			s2y0 = s2y1
			s2y1 = swap
		}
	}
	/*
		deltaS1X = s1x1 - s1x0
		deltaS1Y = s1y1 - s1y0
		deltaS2X = s2x1 - s2x0
		deltaS2Y = s2y1 - s2y1

		if (((deltaS1X * (s2y0 - s1y0)) - (deltaS1Y * (s2x0 - s1x0))) * ((deltaS1X * (s2y1 - s1y0)) - (deltaS1Y * (s2x1 - s1x0)))) > 0 {
			return true
		}
		if (((deltaS2X * (s1y0 - s2y0)) - (deltaS2Y * (s1x0 - s2x0))) * ((deltaS2X * (s1y1 - s2y0)) - (deltaS2Y * (s1x1 - s2x0)))) > 0 {
			return true
		}
	*/
	if ((((s1x1 - s1x0) * (s2y0 - s1y0)) - ((s1y1 - s1y0) * (s2x0 - s1x0))) * (((s1x1 - s1x0) * (s2y1 - s1y0)) - ((s1y1 - s1y0) * (s2x1 - s1x0)))) > 0 {
		return true
	}
	if ((((s2x1 - s2x0) * (s1y0 - s2y0)) - ((s2y1 - s2y0) * (s1x0 - s2x0))) * (((s2x1 - s2x0) * (s1y1 - s2y0)) - ((s2y1 - s2y0) * (s1x1 - s2x0)))) > 0 {
		return true
	}

	return false

}
