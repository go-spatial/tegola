package geom

import "errors"

var ErrNilPoint = errors.New("geom: nil Point")

// Point describes a simple 2D point
type Point [2]float64

// XY returns an array of 2D coordinates
func (p Point) XY() [2]float64 {
	return p
}

// SetXY sets a pair of coordinates
func (p *Point) SetXY(xy [2]float64) (err error) {
	if p == nil {
		return ErrNilPoint
	}

	p[0] = xy[0]
	p[1] = xy[1]
	return
}

// Points returns a slice of XY values
func (p Point) Points() [][2]float64 {
	return [][2]float64{
		{p[0], p[1]},
	}
}
