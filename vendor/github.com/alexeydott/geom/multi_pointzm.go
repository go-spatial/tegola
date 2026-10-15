package geom

import "errors"

// ErrNilMultiPointZM is thrown when a MultiPointZM is nil but shouldn't be
var ErrNilMultiPointZM = errors.New("geom: nil MultiPointZM")

// MultiPointZM is a geometry with multiple 3+1D points.
type MultiPointZM [][4]float64

// Points returns the coordinates for the 3+1D points
func (mpzm MultiPointZM) Points() [][4]float64 {
	return mpzm
}

// SetPoints modifies the array of 3+1D coordinates
func (mpzm *MultiPointZM) SetPoints(input [][4]float64) (err error) {
	if mpzm == nil {
		return ErrNilMultiPointZM
	}

	*mpzm = append((*mpzm)[:0], input...)
	return
}

// Get the simple 2D multipoint
func (mpzm MultiPointZM) MultiPoint() MultiPoint {
	var mpv [][2]float64
	var mp MultiPoint

	points := mpzm.Points()
	for i := 0; i < len(points); i++ {
		mpv = append(mpv, [2]float64{points[i][0], points[i][1]})
	}

	mp.SetPoints(mpv)
	return mp
}
