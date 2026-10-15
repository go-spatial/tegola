package geom

import "errors"

// ErrNilMultiPointM is thrown when a MultiPointM is nil but shouldn't be
var ErrNilMultiPointM = errors.New("geom: nil MultiPointM")

// MultiPointM is a geometry with multiple 2+1D points.
type MultiPointM [][3]float64

// Points returns the coordinates for the 2+1D points
func (mpm MultiPointM) Points() [][3]float64 {
	return mpm
}

// SetPoints modifies the array of 2+1D coordinates
func (mpm *MultiPointM) SetPoints(input [][3]float64) (err error) {
	if mpm == nil {
		return ErrNilMultiPointM
	}

	*mpm = append((*mpm)[:0], input...)
	return
}

// Get the simple 2D multipoint
func (mpm MultiPointM) MultiPoint() MultiPoint {
	var mpv [][2]float64
	var mp MultiPoint

	points := mpm.Points()
	for i := 0; i < len(points); i++ {
		mpv = append(mpv, [2]float64{points[i][0], points[i][1]})
	}

	mp.SetPoints(mpv)
	return mp
}
