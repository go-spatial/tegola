package tegola

func LineAsPointPairs(l LineString) (pp []float64) {
	spts := l.Subpoints()
	pp = make([]float64, 0, len(spts)*2)
	for _, pt := range spts {
		pp = append(pp, pt.X(), pt.Y())
	}
	return pp
}
