package geom

import (
	"errors"
	"math"
)

// ErrNilPoint is thrown when a point is null but shouldn't be
var ErrNilPoint = errors.New("geom: nil Point")

var nan = math.NaN()

// EmptyPoint describes an empty 2D point object.
var EmptyPoint = Point{nan, nan}

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

// Subtract will return a new point that is the subtraction of pt from p
func (p Point) Subtract(pt Point) Point {
	return Point{
		p[0] - pt[0],
		p[1] - pt[1],
	}
}

// Multiply will return a new point that is the multiplication of pt and p
func (p Point) Multiply(pt Point) Point {
	return Point{
		p[0] * pt[0],
		p[1] * pt[1],
	}
}

// CrossProduct will return the cross product of the p and pt.
func (p Point) CrossProduct(pt Point) float64 {
	return float64((p[0] * pt[1]) - (p[1] * pt[0]))
}

// Magnitude of the point is the size of the point
func (p Point) Magnitude() float64 {
	return math.Sqrt((p[0] * p[0]) + (p[1] * p[1]))
}

// WithinCircle indicates weather the point p is contained
// the the circle defined by a,b,c
// REF: See Guibas and Stolf (1985) p.107
func (p Point) WithinCircle(a, b, c Point) bool {
	bcp := Triangle{[2]float64(b), [2]float64(c), [2]float64(p)}
	acp := Triangle{[2]float64(a), [2]float64(c), [2]float64(p)}
	abp := Triangle{[2]float64(a), [2]float64(b), [2]float64(p)}
	abc := Triangle{[2]float64(a), [2]float64(b), [2]float64(c)}

	return (a[0]*a[0]+a[1]*a[1])*bcp.Area()-
		(b[0]*b[0]+b[1]*b[1])*acp.Area()+
		(c[0]*c[0]+c[1]*c[1])*abp.Area()-
		(p[0]*p[0]+p[1]*p[1])*abc.Area() > 0

}
