package tegola

import "math"

// AreaOfPolygon will calculate the Area of a polygon using the surveyor's formula
// (https://en.wikipedia.org/wiki/Shoelace_formula)
func AreaOfPolygon(p Polygon) (area float64) {
	var points []Point
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
