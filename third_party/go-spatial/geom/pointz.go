package geom

import (
	"errors"
	"math"
)

// ErrNilPointZ is thrown when a point is null but shouldn't be
var ErrNilPointZ = errors.New("geom: nil PointZ")

// Point describes a simple 3D point
type PointZ [3]float64

// XYZ returns an array of 3D coordinates
func (p PointZ) XYZ() [3]float64 {
	return p
}

// XY returns an array of 2D coordinates
func (p PointZ) XY() [2]float64 {
	return Point{
		p[0],
		p[1],
	}
}

// SetXYZ sets the three coordinates
func (p *PointZ) SetXYZ(xyz [3]float64) (err error) {
	if p == nil {
		return ErrNilPointZ
	}

	p[0] = xyz[0]
	p[1] = xyz[1]
	p[2] = xyz[2]
	return
}

// Magnitude of the point is the size of the point
func (p PointZ) Magnitude() float64 {
	return math.Sqrt((p[0] * p[0]) + (p[1] * p[1]) + (p[2] * p[2]))
}
