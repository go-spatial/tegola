package basic

import (
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/maths"
)

// Polygon describes a basic polygon; made up of multiple lines.
type Polygon []Line

func PolygonsEqual(p1, p2 Polygon, delta float64) bool {
	if len(p1.Sublines()) != len(p2.Sublines()) {
		return false
	}

	for i := 0; i < len(p1.Sublines()); i++ {
		l1 := p1.Sublines()[i].(Line)
		l2 := p2.Sublines()[i].(Line)
		if !LinesEqual(l1, l2, delta) {
			return false
		}
	}

	return true
}

// Just to make basic collection only usable with basic types.
func (Polygon) basicType() {}

// Sublines returns the lines that make up the polygon.
func (p Polygon) Sublines() (slines []tegola.LineString) {
	slines = make([]tegola.LineString, 0, len(p))
	for i := range p {
		slines = append(slines, p[i])
	}
	return slines
}

func (Polygon) String() string {
	return "Polygon"
}

// MultiPolygon describes a set of polygons.
type MultiPolygon []Polygon

// Checks for mp1 == mp2 with coordinate values within delta
func MultiPolygonsEqual(mp1, mp2 MultiPolygon, delta float64) bool {
	if len(mp1.Polygons()) != len(mp2.Polygons()) {
		return false
	}

	for i := 0; i < len(mp1.Polygons()); i++ {
		p1 := mp1.Polygons()[i].(Polygon)
		p2 := mp2.Polygons()[i].(Polygon)
		if !PolygonsEqual(p1, p2, delta) {
			return false
		}
	}

	return true
}

// Just to make basic collection only usable with basic types.
func (MultiPolygon) basicType() {}

// Polygons retuns the polygons that make up the set.
func (mp MultiPolygon) Polygons() (polygons []tegola.Polygon) {
	polygons = make([]tegola.Polygon, 0, len(mp))
	for i := range mp {
		polygons = append(polygons, mp[i])
	}
	return polygons
}
func (MultiPolygon) String() string {
	return "MultiPolygon"
}

func NewPolygon(main []maths.Pt, clines ...[]maths.Pt) Polygon {
	p := Polygon{NewLineFromPt(main...)}
	for _, l := range clines {
		p = append(p, NewLineFromPt(l...))
	}
	return p
}
func NewPolygonFromSubLines(lines ...tegola.LineString) (p Polygon) {
	p = make(Polygon, 0, len(lines))
	for i := range lines {
		l := NewLineFromSubPoints(lines[i].Subpoints()...)
		p = append(p, l)
	}
	return p
}

func NewMultiPolygonFromPolygons(polygons ...tegola.Polygon) (mp MultiPolygon) {
	mp = make(MultiPolygon, 0, len(polygons))
	for i := range polygons {
		p := NewPolygonFromSubLines(polygons[i].Sublines()...)
		mp = append(mp, p)
	}
	return mp
}
