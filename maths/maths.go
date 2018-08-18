/*
Package math contins generic math functions that we need for doing transforms.
this package will augment the go math library.
*/
package maths

import (
	"fmt"
	"math"

	"errors"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/maths/webmercator"
)

const (
	WebMercator = webmercator.SRID
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
		X: math.Trunc(pt.X),
		Y: math.Trunc(pt.Y),
	}
}

func round(f float64) float64 {
	i, r := math.Modf(f)
	if r > 0.5 {
		return i + 1
	}
	return i
}
func (pt Pt) Round() Pt {
	return Pt{
		round(pt.X),
		round(pt.Y),
	}
}

func (pt Pt) Delta(pt2 Pt) (d Pt) {
	return Pt{
		X: pt.X - pt2.X,
		Y: pt.Y - pt2.Y,
	}
}

func (pt Pt) String() string {
	return fmt.Sprintf("{%v,%v}", pt.X, pt.Y)
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

func AreaOfRing(points ...Pt) (area float64) {
	n := len(points)
	for i := range points {
		j := (i + 1) % n
		area += points[i].X * points[j].Y
		area -= points[j].X * points[i].Y
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

// Intersect find the intersection point (x,y) between two lines if there is one. Ok will be true if it found an intersection point, and false if it did not.
func Intersect(l1, l2 Line) (pt Pt, ok bool) {
	/*
		if !l1.DoesIntersect(l2) {
			return pt, false
		}
	*/

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

	// if the slopes are the same then they are parallel so, they don't intersect.
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

// Contains takes a subject (made up of point pairs) and a pt, and returns weather the pt is contained by the subject.
func Contains(subject []float64, pt Pt) (bool, error) {
	segments, err := NewSegments(subject)
	if err != nil {
		return false, err
	}
	count := 0
	ray := Line{pt, Pt{0, pt.Y}}
	// eliminate segments we don't need touch.
	for i := range segments {

		line := segments[i]

		deltaY := line[1].Y - line[0].Y
		// If the line is horizontal skipp it.
		if deltaY == 0 {
			continue
		}
		// if both points are greater or equal to the pts x we can remove it.
		if line[0].X >= pt.X && line[1].X >= pt.X {
			continue
		}
		// if the line is above ray, we don't need to consider it.
		if line[0].Y <= pt.Y && line[1].Y <= pt.Y {
			continue
		}
		ray[1].X = line[0].X
		if ray[1].X > line[1].X {
			ray[1].X = line[1].X
		}
		// move the point out by 10
		ray[1].X -= 10

		pt, ok := Intersect(ray, line)
		if !ok || !line.InBetween(pt) || !ray.InBetween(pt) {
			continue
		}

		count++
	}
	// Even means outside, odd means the point is contained.
	return count%2 != 0, nil
}

func XYOrder(pt1, pt2 Pt) int {

	switch {

	// Test the x-coord first
	case pt1.X > pt2.X:
		return 1
	case pt1.X < pt2.X:
		return -1

		// Test the y-coord second
	case pt1.Y > pt2.Y:
		return 1
	case pt1.Y < pt2.Y:
		return -1

	}

	// when you exclude all other possibilities, what remains  is...
	return 0 // they are the same point
}

func YXorder(pt1, pt2 Pt) int {

	// Test the y-coord first
	switch {
	case pt1.Y > pt2.Y:
		return 1
	case pt1.Y < pt2.Y:
		return -1

		// Test the x-coord second
	case pt1.X > pt2.X:
		return 1
	case pt1.X < pt2.X:
		return -1
	}

	// when you exclude all other possibilities, what remains  is...
	return 0 // they are the same point
}

// Powers of 2
func Exp2(p uint64) uint64 {
	// this mimics behavior from casting
	// a math.Exp2 which should overflow
	if p > 63 {
		p = 63
	}
	return uint64(1) << p
}

// Minimum of uints
func Min(x, y uint) uint {
	if x < y {return x}
	return y
}