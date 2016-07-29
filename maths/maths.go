/*
Package math contins generic math functions that we need for doing transforms.
this package will augment the go math library.
*/
package maths

import (
	"math"

	"github.com/terranodo/tegola"
)

const (
	WebMercator = tegola.WebMercator
	WGS84       = tegola.WGS84
	Deg2Rad     = math.Pi / 180
	Rad2Deg     = 180 / math.Pi
	PiDiv2      = math.Pi / 2.0
)

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
