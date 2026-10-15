package geom

import "errors"

// ErrNilMultiPointZ is thrown when a MultiPointZ is nil but shouldn't be
var ErrNilMultiPointZ = errors.New("geom: nil MultiPointZ")

// MultiPointZ is a geometry with multiple 3D points.
type MultiPointZ [][3]float64

// Points returns the coordinates for the 3D points
func (mpz MultiPointZ) Points() [][3]float64 {
	return mpz
}

// SetPoints modifies the array of 3D coordinates
func (mpz *MultiPointZ) SetPoints(input [][3]float64) (err error) {
	if mpz == nil {
		return ErrNilMultiPointZ
	}

	*mpz = append((*mpz)[:0], input...)
	return
}

// Get the simple 2D multipoint
func (mpz MultiPointZ) MultiPoint() MultiPoint {
	var mpv [][2]float64
	var mp MultiPoint

	points := mpz.Points()
	for i := 0; i < len(points); i++ {
		mpv = append(mpv, [2]float64{points[i][0], points[i][1]})
	}

	mp.SetPoints(mpv)
	return mp
}
