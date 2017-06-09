/*
Package math contins generic math functions that we need for doing transforms.
this package will augment the go math library.
*/
package maths

import (
	"fmt"
	"math"

	"errors"

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

// Pt describes a 2d Point.
type Pt struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (pt Pt) XCoord() float64   { return pt.X }
func (pt Pt) YCoord() float64   { return pt.X }
func (pt Pt) Coords() []float64 { return []float64{pt.X, pt.Y} }

func (pt Pt) IsEqual(pt2 Pt) bool {
	return pt.X == pt2.X && pt.Y == pt2.Y
}

func (pt Pt) Truncate() Pt {
	return Pt{
		X: float64(int64(pt.X)),
		Y: float64(int64(pt.Y)),
	}
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
func (pt *Pt) GoString() string {
	if pt == nil {
		return "(nil)"
	}
	return fmt.Sprintf("[%v,%v]", pt.X, pt.Y)
}

type Pointer interface {
	Point() Pt
}

func NewPoints(f []float64) (pts []Pt, err error) {
	if len(f)%2 != 0 {
		return pts, errors.New("Expected even number of points.")
	}
	for x, y := 0, 1; y < len(f); x, y = x+2, y+2 {
		pts = append(pts, Pt{f[x], f[y]})
	}
	return pts, nil
}
func NewSegments(f []float64) (lines []Line, err error) {
	if len(f)%2 != 0 {
		return lines, errors.New("Expected even number of points.")
	}
	lx, ly := len(f)-2, len(f)-1
	for x, y := 0, 1; y < len(f); x, y = x+2, y+2 {
		lines = append(lines, NewLine(f[lx], f[ly], f[x], f[y]))
		lx, ly = x, y
	}
	return lines, nil
}

type Line [2]Pt

func NewLine(x1, y1, x2, y2 float64) Line {
	return Line{
		Pt{x1, y1},
		Pt{x2, y2},
	}
}

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

	goodx, goody := lx < pt.X && pt.X < gx, ly < pt.Y && pt.Y < gy
	if gx-lx == 0 {
		goodx = true
	}
	if gy-ly == 0 {
		goody = true
	}

	//log.Println(l, pt, ":", lx, "<", pt.X, "&&", pt.X, "<", gx, "&&", ly, "<", pt.Y, "&&", pt.Y, "<", gy, goodx, goody)
	return goodx && goody

}

func (l Line) IsVertical() bool {
	return l[0].X == l[1].X
}
func (l Line) IsHorizontal() bool {
	return l[0].Y == l[1].Y
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

// DistanceFromPoint will return the perpendicular distance from the point.
func (l Line) DistanceFromPoint(pt Pt) float64 {

	deltaX := l[1].X - l[0].X
	deltaY := l[1].Y - l[0].Y
	//log.Println("delta X/Y :  pt - line", deltaX, deltaY, pt, l)
	denom := math.Abs((deltaY * pt.X) - (deltaX * pt.Y) + (l[1].X * l[0].Y) - (l[1].Y * l[0].X))
	num := math.Sqrt(math.Pow(deltaY, 2) + math.Pow(deltaX, 2))
	//log.Println("denim/num", denom, num)
	if num == 0 {
		return 0
	}
	return denom / num
}

// AreaOfPolygon will calculate the Area of a polygon using the surveyor's formula
// (https://en.wikipedia.org/wiki/Shoelace_formula)
func AreaOfPolygon(p tegola.Polygon) (area float64) {
	sublines := p.Sublines()
	if len(sublines) == 0 {
		return 0
	}
	// Only care about the outer ring.
	return AreaOfPolygonLineString(sublines[0])
}

func AreaOfPolygonLineString(line tegola.LineString) (area float64) {
	// Only care about the outer ring.
	points := line.Subpoints()

	n := len(points)
	for i := range points {
		j := (i + 1) % n
		area += points[i].X() * points[j].Y()
		area -= points[j].X() * points[i].Y()
	}
	return math.Abs(area) / 2.0
}

// DistOfLine will calculate the Manhattan distance of a line.
func DistOfLine(l tegola.LineString) (dist float64) {
	points := l.Subpoints()
	if len(points) == 0 {
		return 0
	}
	for i, j := 0, 1; j < len(points); i, j = i+1, j+1 {
		dist += math.Abs(points[j].X()-points[i].X()) + math.Abs(points[j].Y()-points[i].Y())
	}
	return dist
}

// DistOfLine will calculate the

func RadToDeg(rad float64) float64 {
	return rad * Rad2Deg
}

func DegToRad(deg float64) float64 {
	return deg * Deg2Rad
}

// SlopeIntercept will find the slop (if there is one) and the intercept of the two provided lines. If there isn't a slope because the lines are verticle, the slopeDefined will be false.
func (l Line) SlopeIntercept() (m, b float64, defined bool) {
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

	// if the l1 is vertical.
	if l1.IsVertical() {

		if l2.IsVertical() {
			return pt, false
		}

		if l1[0].X == l2[0].X {
			return Pt{X: l1[0].X, Y: l2[0].Y}, true
		}
		if l1[0].X == l2[1].X {
			return Pt{X: l1[0].X, Y: l2[1].Y}, true
		}
	}
	if l1.IsHorizontal() {

		if l2.IsHorizontal() {
			return pt, false
		}
		if l1[0].Y == l2[0].Y {
			return Pt{X: l2[0].X, Y: l1[0].Y}, true
		}
		if l1[0].Y == l2[1].Y {
			return Pt{X: l2[1].X, Y: l1[0].Y}, true
		}
	}
	m1, b1, sdef1 := l1.SlopeIntercept()
	m2, b2, sdef2 := l2.SlopeIntercept()

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
