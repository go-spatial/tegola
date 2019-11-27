package planar

// PointsCentriod returns the center of the given pts
func PointsCentriod(pts ...[2]float64) (center [2]float64) {
	if len(pts) == 0 {
		return center
	}
	if len(pts) == 1 {
		return pts[0]
	}
	var a, aa, cx, cy float64
	for i := range pts[:len(pts)-1] {
		pt, npt := pts[i], pts[i+1]
		aa = (pt[0] * npt[1]) - (npt[0] * pt[1])
		a += aa
		cx += (pt[0] + npt[0]) * aa
		cy += (pt[1] + npt[1]) * aa
	}
	cx = cx / (3 * aa)
	cy = cy / (3 * aa)
	return [2]float64{cx, cy}
}
