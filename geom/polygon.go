package geom

import "math"

type Polygon struct {
	Points []Point
}

//	calculate the area of the polygone using the surveyor's formula (https://en.wikipedia.org/wiki/Shoelace_formula)
func (p *Polygon) Area() float64 {
	var area float64
	//	number of vertices
	n := len(p.Points)

	for i := range p.Points {
		j := (i + 1) % n
		area += p.Points[i].X * p.Points[j].Y
		area -= p.Points[j].X * p.Points[i].Y

	}

	return math.Abs(area) / 2.0
}
