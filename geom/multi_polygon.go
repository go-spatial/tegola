package geom

import "errors"

var ErrNilMultiPolygon = errors.New("geom: nil MultiPolygon")

// MultiPolygon is a geometry of multiple polygons.
type MultiPolygon [][][][2]float64

// Polygons returns the array of polygons.
func (mp MultiPolygon) Polygons() [][][][2]float64 {
	return mp
}

// SetPolygons modifies the array of 2D coordinates
func (mp *MultiPolygon) SetPolygons(input [][][][2]float64) (err error) {
	if mp == nil {
		return ErrNilMultiPolygon
	}

	*mp = append((*mp)[:0], input...)
	return
}
