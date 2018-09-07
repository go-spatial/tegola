package geom

import "errors"

// ErrNilMultiPoint is thrown when a MultiPoint is nil but shouldn't be
var ErrNilMultiPoint = errors.New("geom: nil MultiPoint")

// MultiPoint is a geometry with multiple points.
type MultiPoint [][2]float64

// Points returns the coordinates for the points
func (mp MultiPoint) Points() [][2]float64 {
	return mp
}

// SetPoints modifies the array of 2D coordinates
func (mp *MultiPoint) SetPoints(input [][2]float64) (err error) {
	if mp == nil {
		return ErrNilMultiPoint
	}

	*mp = append((*mp)[:0], input...)
	return
}
