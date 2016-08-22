package basic

import "github.com/terranodo/tegola"

// Polygon describes a basic polygon; made up of multiple lines.
type Polygon []Line

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
	return "Polygon"
}
