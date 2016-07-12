package basic

import (
	"fmt"

	"github.com/terranodo/tegola"
)

// RehomeGeometry will make sure to normalize all points to the ulx and uly coordinates.
func RehomeGeometry(geometery tegola.Geometry, ulx, uly float64) (tegola.Geometry, error) {
	switch geo := geometery.(type) {
	default:
		return nil, fmt.Errorf("Unknown Geometry: %+v", geometery)
	case tegola.Point:
		return &Point{geo.X() - ulx, geo.Y() - uly}, nil
	case tegola.Point3:
		return &Point3{geo.X() - ulx, geo.Y() - uly, geo.Z()}, nil
	case tegola.MultiPoint:
		var pts MultiPoint
		for _, pt := range geo.Points() {
			pts = append(pts, Point{pt.X() - ulx, pt.Y() - uly})
		}
		return pts, nil
	case tegola.LineString:
		var line Line
		for _, ptGeo := range geo.Subpoints() {
			line = append(line, Point{ptGeo.X() - ulx, ptGeo.Y() - uly})
		}
		return &line, nil
	case tegola.MultiLine:
		var line MultiLine
		for i, lineGeo := range geo.Lines() {
			geoLine, err := RehomeGeometry(lineGeo, ulx, uly)
			if err != nil {
				return nil, fmt.Errorf("Got error converting line(%v) of multiline: %v", i, err)
			}
			ln, _ := geoLine.(Line)
			line = append(line, ln)
		}
		return line, nil
	case tegola.Polygon:
		var poly Polygon
		for i, line := range geo.Sublines() {
			geoLine, err := RehomeGeometry(line, ulx, uly)
			if err != nil {
				return nil, fmt.Errorf("Got error converting line(%v) of polygon: %v", i, err)
			}
			ln, _ := geoLine.(Line)
			poly = append(poly, ln)
		}
		return &poly, nil
	case tegola.MultiPolygon:
		var mpoly MultiPolygon
		for i, poly := range geo.Polygons() {
			geoPoly, err := RehomeGeometry(poly, ulx, uly)
			if err != nil {
				return nil, fmt.Errorf("Got error converting poly(%v) of multipolygon: %v", i, err)
			}
			p, _ := geoPoly.(Polygon)
			mpoly = append(mpoly, p)
		}
		return &mpoly, nil
	}
}
