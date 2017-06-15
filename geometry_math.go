package tegola

const (
	WebMercator = 3857
	WGS84       = 4326
)

func LineAsPointPairs(l LineString) (pp []float64) {
	spts := l.Subpoints()
	pp = make([]float64, 0, len(spts)*2)
	for _, pt := range spts {
		pp = append(pp, pt.X(), pt.Y())
	}
	return pp
}

/*
This is causing an import cycle. Seems maths is relying on tegola. Maths library should work on []float64 point pairs I think.
func LineAsSegments(l LineString) (lines []maths.Line) {
	spts := l.Subpoints()
	lpt := spts[len(spts)-1]

	for _, pt := range spts {
		lines = append(lines, maths.NewLine(lpt.X(), lpt.Y(), pt.X(), pt.Y()))
		lpt = pt
	}
	return lines
}
*/
