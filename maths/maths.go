/*
Package math contins generic math functions that we need for doing transforms.
this package will augment the go math library.
*/
package maths

import (
	"fmt"
	"math"

	"github.com/terranodo/tegola"
)

const (
	WebMercator = tegola.WebMercator
	WGS84       = tegola.WGS84
	Deg2Rad     = math.Pi / 180
	Rad2Deg     = 180 / math.Pi
	PiDiv2      = math.Pi / 2.0
	PiDiv4      = math.Pi / 4.0
)

// WindingOrder the direction the line strings.
type WindingOrder uint8

const (
	_ WindingOrder = iota
	Clockwise
	CounterClockwise
)

func (w WindingOrder) String() string {
	switch w {
	case Clockwise:
		return "clockwise"
	case CounterClockwise:
		return "counter clockwise"
	}
	return "unknown"
}

func (w WindingOrder) IsClockwise() bool        { return w == Clockwise }
func (w WindingOrder) IsCounterClockwise() bool { return w == CounterClockwise }

// Pt describes a 2d Point.
type Pt struct {
	X, Y float64
}

func (pt Pt) XCoord() float64   { return pt.X }
func (pt Pt) YCoord() float64   { return pt.X }
func (pt Pt) Coords() []float64 { return []float64{pt.X, pt.Y} }

func (pt Pt) IsEqual(pt2 Pt) bool {
	return pt.X == pt2.X && pt.Y == pt2.Y
}

func (pt Pt) Delta(pt2 Pt) (d Pt) {
	return Pt{
		X: pt.X - pt2.X,
		Y: pt.Y - pt2.Y,
	}
}

func (pt Pt) String() string {
	return fmt.Sprintf("(%v,%v)", pt.X, pt.Y)
}

type Pointer interface {
	Point() Pt
}

type Line [2]Pt

// InBetween will check to see if the given point lies the line provided inbetween the endpoints.
func (l Line) InBetween(pt Pt) bool {
	lx, gx := l[0].X, l[1].X
	if l[0].X > l[1].X {
		lx, gx = l[1].X, l[0].X
	}
	ly, gy := l[0].Y, l[1].Y
	if l[0].Y > l[1].Y {
		ly, gy = l[1].Y, l[0].Y
	}
	return lx <= pt.X && pt.X <= gx && ly <= pt.Y && pt.Y <= gy

}
func (l Line) ExInBetween(pt Pt) bool {
	lx, gx := l[0].X, l[1].X
	if l[0].X > l[1].X {
		lx, gx = l[1].X, l[0].X
	}
	ly, gy := l[0].Y, l[1].Y
	if l[0].Y > l[1].Y {
		ly, gy = l[1].Y, l[0].Y
	}
	return lx < pt.X && pt.X < gx && ly < pt.Y && pt.Y < gy

}

//Clamp will return a point that is on the line based on pt. It will do this by restricting each of the coordiantes to the line.
func (l Line) Clamp(pt Pt) (p Pt) {
	p = pt
	lx, gx := l[0].X, l[1].X
	if l[0].X > l[1].X {
		lx, gx = l[1].X, l[0].X
	}
	ly, gy := l[0].Y, l[1].Y
	if l[0].Y > l[1].Y {
		ly, gy = l[1].Y, l[0].Y
	}

	if pt.X < lx {
		p.X = lx
	}
	if pt.X > gx {
		p.X = gx
	}
	if pt.Y < ly {
		p.Y = ly
	}
	if pt.Y > gy {
		p.Y = gy
	}
	return p
}

// AreaOfPolygon will calculate the Area of a polygon using the surveyor's formula
// (https://en.wikipedia.org/wiki/Shoelace_formula)
func AreaOfPolygon(p tegola.Polygon) (area float64) {
	var points []tegola.Point
	for _, l := range p.Sublines() {
		points = append(points, l.Subpoints()...)
	}
	n := len(points)
	for i := range points {
		j := (i + 1) % n
		area += points[i].X() * points[j].Y()
		area -= points[j].X() * points[i].Y()
	}
	return math.Abs(area) / 2.0
}

func RadToDeg(rad float64) float64 {
	return rad * Rad2Deg
}

func DegToRad(deg float64) float64 {
	return deg * Deg2Rad
}

// SlopeIntercept will find the slop (if there is one) and the intercept of the two provided lines. If there isn't a slope because the lines are verticle, the slopeDefined will be false.
func (l Line) SlopeIntercep() (m, b float64, defined bool) {
	dx := l[1].X - l[0].X
	dy := l[1].Y - l[0].Y
	if dx == 0 || dy == 0 {
		// if dx == 0 then m == 0; and the intercept is y.
		// However if the lines are verticle then the slope is not defined.
		return 0, l[0].Y, dx != 0
	}
	m = dy / dx
	// b = y - mx
	b = l[0].Y - (m * l[0].X)
	return m, b, true
}

// Intersect find the intersection point (x,y) between two lines if there is one. Ok will be true if it found an intersection point, and false if it did not.
func Intersect(l1, l2 Line) (pt Pt, ok bool) {
	if l1[0].X == l1[1].X {
		if l1[0].X == l2[0].X {
			return Pt{X: l1[0].X, Y: l2[0].Y}, true
		}
		if l1[0].X == l2[1].X {
			return Pt{X: l1[0].X, Y: l2[1].Y}, true
		}
	}
	if l1[0].Y == l1[1].Y {
		if l1[0].Y == l2[0].Y {
			return Pt{X: l2[0].X, Y: l1[0].Y}, true
		}
		if l1[0].Y == l2[1].Y {
			return Pt{X: l2[1].X, Y: l1[0].Y}, true
		}
	}
	m1, b1, sdef1 := l1.SlopeIntercep()
	m2, b2, sdef2 := l2.SlopeIntercep()

	// if the slopes are the smae then they are parallel so, they don't intersect.
	if sdef1 == sdef2 && m1 == m2 {
		return Pt{}, false
	}

	// line1 is horizontal. We have a value for x, need a value for y.
	if !sdef1 {
		x := l1[0].X
		if m2 == 0 {
			return Pt{X: x, Y: b2}, true
		}
		y := (m2 * x) + b2
		return Pt{X: x, Y: y}, true
	}
	// line2 is horizontal. We have a value for x, need a value for y.
	if !sdef2 {
		x := l2[0].X
		if m1 == 0 {
			return Pt{X: x, Y: b1}, true
		}
		y := (m1 * x) + b1
		return Pt{X: x, Y: y}, true
	}
	if m1 == 0 {
		y := l1[0].Y
		x := (y - b2) / m2
		return Pt{X: x, Y: y}, true
	}
	if m2 == 0 {
		y := l2[0].Y
		x := (y - b1) / m1
		return Pt{X: x, Y: y}, true
	}
	dm := m1 - m2
	db := b2 - b1
	x := db / dm
	y := (m1 * x) + b1
	return Pt{X: x, Y: y}, true
}
