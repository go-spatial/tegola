package geom

import (
	"errors"
)

// ErrNilPointZM is thrown when a point is null but shouldn't be
var ErrNilPointZM = errors.New("geom: nil PointZM")

// Point describes a simple 3D+1D point
type PointZM [4]float64

// XYZM returns an array of 3D+1D coordinates
func (p PointZM) XYZM() [4]float64 {
	return p
}

// XYZ returns an array of 3D coordinates
func (p PointZM) XYZ() [3]float64 {
	return PointZ{
		p[0],
		p[1],
		p[2],
	}
}

// M returns the metric related to the 2D point
func (p PointZM) M() float64 {
	return p[3]
}

// SetXYZM sets the three coordinates
func (p *PointZM) SetXYZM(xyzm [4]float64) (err error) {
	if p == nil {
		return ErrNilPointZM
	}

	p[0] = xyzm[0]
	p[1] = xyzm[1]
	p[2] = xyzm[2]
	p[3] = xyzm[3]
	return
}
