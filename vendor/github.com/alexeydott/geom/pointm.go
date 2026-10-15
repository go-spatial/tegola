package geom

import (
	"errors"
)

// ErrNilPointM is thrown when a point is null but shouldn't be
var ErrNilPointM = errors.New("geom: nil PointM")

// Point describes a simple 2D+1D point
type PointM [3]float64

// XYM returns an array of 2D+1D coordinates
func (p PointM) XYM() [3]float64 {
	return p
}

// XY returns an array of 2D coordinates
func (p PointM) XY() [2]float64 {
	return Point{
		p[0],
		p[1],
	}
}

// M returns the metric related to the 2D point
func (p PointM) M() float64 {
	return p[2]
}

// SetXYM sets the three coordinates
func (p *PointM) SetXYM(xym [3]float64) (err error) {
	if p == nil {
		return ErrNilPointM
	}

	p[0] = xym[0]
	p[1] = xym[1]
	p[2] = xym[2]
	return
}
