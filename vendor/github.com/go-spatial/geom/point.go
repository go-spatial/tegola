package geom

import "errors"

// ErrNilPoint is thrown when a point is null but shouldn't be
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

// X is the x coordinate of a point in the projection
func (p Point) X() float64 { return p[0] }

// Y is the y coordinate of a point in the projection
func (p Point) Y() float64 { return p[1] }

// MaxX is the same as X
func (p Point) MaxX() float64 { return p[0] }

// MinX is the same as X
func (p Point) MinX() float64 { return p[0] }

// MaxY is the same as y
func (p Point) MaxY() float64 { return p[1] }

// MinY is the same as y
func (p Point) MinY() float64 { return p[1] }

// Area of a point is always 0
func (p Point) Area() float64 { return 0 }
