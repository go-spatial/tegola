package basic

import "github.com/go-spatial/tegola"

// ClonePoint will return a basic.Point for given tegola.Point.
func ClonePoint(pt tegola.Point) Point {
	return Point{pt.X(), pt.Y()}
}

// ClonePoint will return a basic.Point3 for given tegola.Point3.
func ClonePoint3(pt tegola.Point3) Point3 {
	return Point3{pt.X(), pt.Y(), pt.Z()}
}

// CloneMultiPoint will return a basic.MultiPoint for the given tegol.MultiPoint
func CloneMultiPoint(mpt tegola.MultiPoint) MultiPoint {
	var bmpt MultiPoint
	for _, pt := range mpt.Points() {
		bmpt = append(bmpt, ClonePoint(pt))
	}
	return bmpt
}

/*
// CloneMultiPoint3 will return a basic.MultiPoint3 for the given tegol.MultiPoint3
func CloneMultiPoint3(mpt tegola.MultiPoint3) MultiPoint3 {
	var bmpt MultiPoint3
	for _, pt := range mpt.Points() {
		bmpt = append(bmpt, ClonePoint3(pt))
	}
	return bmpt
}
*/

// CloneLine will return a basic.Line for a given tegola.LineString
func CloneLine(line tegola.LineString) (l Line) {
	for _, pt := range line.Subpoints() {
		l = append(l, Point{pt.X(), pt.Y()})
	}
	return l
}

// CloneMultiLine will return a basic.MultiLine for a given togola.MultiLine
func CloneMultiLine(mline tegola.MultiLine) (ml MultiLine) {
	for _, ln := range mline.Lines() {
		ml = append(ml, CloneLine(ln))
	}
	return ml
}

// ClonePolygon will return a basic.Polygon for a given tegola.Polygon
func ClonePolygon(polygon tegola.Polygon) (ply Polygon) {
	for _, ln := range polygon.Sublines() {
		ply = append(ply, CloneLine(ln))
	}
	return ply
}

// CloneMultiPolygon will return a basic.MultiPolygon for a given tegola.MultiPolygon.
func CloneMultiPolygon(mpolygon tegola.MultiPolygon) (mply MultiPolygon) {
	for _, ply := range mpolygon.Polygons() {
		mply = append(mply, ClonePolygon(ply))
	}
	return mply
}

func Clone(geo tegola.Geometry) Geometry {
	switch g := geo.(type) {
	case tegola.Point:
		return ClonePoint(g)
	case tegola.MultiPoint:
		return CloneMultiPoint(g)
	case tegola.LineString:
		return CloneLine(g)
	case tegola.MultiLine:
		return CloneMultiLine(g)
	case tegola.Polygon:
		return ClonePolygon(g)
	case tegola.MultiPolygon:
		return CloneMultiPolygon(g)
	}
	return nil
}
