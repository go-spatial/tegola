package basic

import (
	"fmt"
	"log"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/maths/webmercator"
)

// ApplyToPoints applys the given function to each point in the geometry and any sub geometries, return a new transformed geometry.
func ApplyToPoints(geometry tegola.Geometry, f func(coords ...float64) ([]float64, error)) (tegola.Geometry, error) {
	switch geo := geometry.(type) {
	default:
		return nil, fmt.Errorf("Unknown Geometry: %+v", geometry)
	case tegola.Point:
		c, err := f(geo.X(), geo.Y())
		if err != nil {
			return nil, err
		}
		if len(c) < 2 {
			return nil, fmt.Errorf("Function did not return minimum number of coordinates got %v expected 2", len(c))
		}
		return Point{c[0], c[1]}, nil
	case tegola.Point3:
		c, err := f(geo.X(), geo.Y(), geo.Z())
		if err != nil {
			return nil, err
		}
		if len(c) < 3 {
			return nil, fmt.Errorf("Function did not return minimum number of coordinates got %v expected 3", len(c))
		}
		return Point3{c[0], c[1], c[2]}, nil
	case tegola.MultiPoint:
		var pts MultiPoint
		for _, pt := range geo.Points() {
			c, err := f(pt.X(), pt.Y())
			if err != nil {
				return nil, err
			}
			if len(c) < 2 {
				return nil, fmt.Errorf("Function did not return minimum number of coordinates got %v expected 2", len(c))
			}
			pts = append(pts, Point{c[0], c[1]})
		}
		return pts, nil
	case tegola.LineString:
		var line Line
		for _, ptGeo := range geo.Subpoints() {
			c, err := f(ptGeo.X(), ptGeo.Y())
			if err != nil {
				return nil, err
			}
			if len(c) < 2 {
				return nil, fmt.Errorf("Function did not return minimum number of coordinates got %v expected 2", len(c))
			}
			line = append(line, Point{c[0], c[1]})
		}
		return line, nil
	case tegola.MultiLine:
		var line MultiLine
		for i, lineGeo := range geo.Lines() {
			geoLine, err := ApplyToPoints(lineGeo, f)
			if err != nil {
				return nil, fmt.Errorf("Got error converting line(%v) of multiline: %v", i, err)
			}
			ln, ok := geoLine.(Line)
			if !ok {
				log.Printf("We did not get the conversion we were expecting: %t", geoLine)
				panic("We did not get the conversion we were expecting")
			}
			line = append(line, ln)
		}
		return line, nil
	case tegola.Polygon:
		var poly Polygon
		for i, line := range geo.Sublines() {
			geoLine, err := ApplyToPoints(line, f)
			if err != nil {
				return nil, fmt.Errorf("Got error converting line(%v) of polygon: %v", i, err)
			}
			ln, ok := geoLine.(Line)
			if !ok {
				panic("We did not get the conversion we were expecting")
			}

			poly = append(poly, ln)
		}
		return poly, nil
	case tegola.MultiPolygon:
		var mpoly MultiPolygon
		for i, poly := range geo.Polygons() {
			geoPoly, err := ApplyToPoints(poly, f)
			if err != nil {
				return nil, fmt.Errorf("Got error converting poly(%v) of multipolygon: %v", i, err)
			}
			p, ok := geoPoly.(Polygon)
			if !ok {
				panic("We did not get the conversion we were expecting")
			}
			mpoly = append(mpoly, p)
		}
		return mpoly, nil
	}
}

// CloneGeomtry returns a deep clone of the Geometry.
func CloneGeometry(geometry tegola.Geometry) (tegola.Geometry, error) {
	switch geo := geometry.(type) {
	default:
		return nil, fmt.Errorf("Unknown Geometry: %t", geometry)
	case tegola.Point:
		return Point{geo.X(), geo.Y()}, nil
	case tegola.Point3:
		return Point3{geo.X(), geo.Y(), geo.Z()}, nil
	case tegola.MultiPoint:
		var pts MultiPoint
		for i, pt := range geo.Points() {
			geopt, err := CloneGeometry(pt)
			if err != nil {
				return nil, fmt.Errorf("Failed to clone point(%v) of multipoint: %v", i, err)
			}
			p, ok := geopt.(Point)
			if !ok {
				return nil, fmt.Errorf("Failed to clone point(%v) of multipoint: %t", i, geopt)
			}
			pts = append(pts, p)
		}
		return pts, nil
	case tegola.LineString:
		var line Line
		for i, pt := range geo.Subpoints() {
			geopt, err := CloneGeometry(pt)
			if err != nil {
				return nil, fmt.Errorf("Failed to clone point(%v) of line: %v", i, err)
			}
			p, ok := geopt.(Point)
			if !ok {
				return nil, fmt.Errorf("Failed to clone point(%v) of line", i)
			}
			line = append(line, p)
		}
		return line, nil
	case tegola.MultiLine:
		var line MultiLine
		for i, lineGeo := range geo.Lines() {
			geoLine, err := CloneGeometry(lineGeo)
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
			geoLine, err := CloneGeometry(line)
			if err != nil {
				return nil, fmt.Errorf("Got error converting line(%v) of polygon: %v", i, err)
			}
			ln, _ := geoLine.(Line)
			poly = append(poly, ln)
		}
		return poly, nil
	case tegola.MultiPolygon:
		var mpoly MultiPolygon
		for i, poly := range geo.Polygons() {
			geoPoly, err := CloneGeometry(poly)
			if err != nil {
				return nil, fmt.Errorf("Got error converting poly(%v) of multipolygon: %v", i, err)
			}
			p, _ := geoPoly.(Polygon)
			mpoly = append(mpoly, p)
		}
		return mpoly, nil
	}
}

// ToWebMercator takes a SRID and a geometry encode using that srid, and returns a geometry encoded as a WebMercator.
func ToWebMercator(SRID int, geometry tegola.Geometry) (tegola.Geometry, error) {
	switch SRID {
	default:
		return nil, fmt.Errorf("Don't know how to convert from %v to %v.", tegola.WebMercator, SRID)
	case tegola.WebMercator:
		// Instead of just returning the geometry, we are cloning it so that the user of the API can rely
		// on the result to alway be a copy. Instead of being a reference in the on instance that it's already
		// in the same SRID.
		return CloneGeometry(geometry)
	case tegola.WGS84:
		return ApplyToPoints(geometry, webmercator.PToXY)
	}
}

// FromWebMercator takes a geometry encoded with WebMercator, and returns a Geometry encodes to the given srid.
func FromWebMercator(SRID int, geometry tegola.Geometry) (tegola.Geometry, error) {
	switch SRID {
	default:
		return nil, fmt.Errorf("Don't know how to convert from %v to %v.", SRID, tegola.WebMercator)
	case tegola.WebMercator:
		// Instead of just returning the geometry, we are cloning it so that the user of the API can rely
		// on the result to alway be a copy. Instead of being a reference in the on instance that it's already
		// in the same SRID.
		return CloneGeometry(geometry)
	case tegola.WGS84:
		return ApplyToPoints(geometry, webmercator.PToLonLat)
	}
}

func ScaleGeometry(tile tegola.BoundingBox, extent float64, geo tegola.Geometry) (tegola.Geometry, error) {
	xspan := tile.Maxx - tile.Minx
	yspan := tile.Maxy - tile.Miny
	return ApplyToPoints(geo, func(coords ...float64) ([]float64, error) {
		nx := int64((coords[0] - tile.Minx) * extent / xspan)
		ny := int64((coords[1] - tile.Miny) * extent / yspan)
		return []float64{float64(nx), float64(ny)}, nil
	})
}

func Slope(p1, p2 tegola.Point) (float64, error) {
	dx := p2.X() - p1.X()
	if dx == 0 {
		return 0, fmt.Errorf("Line is vertical")
	}
	dy := p2.Y() - p1.Y()
	if dy == 0 {
		return 0, nil
	}
	return (dy / dx), nil

}

func simplifyLine(line tegola.LineString, connected bool) Line {
	var pts []float64
	points := line.Subpoints()
	if len(points) < 3 {
		for _, p := range points {
			pts = append(pts, p.X(), p.Y())
		}
		return NewLine(pts...)
	}
	stpos := 0
	stidx := stpos
	endpos := len(points) - 1

	if connected {
		stidx = endpos
	}
	spt := points[stidx]
	for i := stpos; i < endpos; i++ {
		m1, err1 := Slope(spt, points[i])
		m2, err2 := Slope(spt, points[i+1])
		// If the slope matches we skip the point.
		if err1 == err2 && m1 == m2 {
			continue
		}
		pts = append(pts, points[i].X(), points[i].Y())
		spt = points[i]
	}
	if len(pts) < 2 {
		// 1 point or few means the line should disappear.
		return nil
	}
	// Always add the last point.
	pts = append(pts, points[len(points)-1].X(), points[len(points)-1].Y())
	return NewLine(pts...)
}

func simplifyPolygon(geo tegola.Polygon) Polygon {
	var pol []Line
	for _, l := range geo.Sublines() {
		ln := simplifyLine(l, true)
		if ln != nil {
			pol = append(pol, ln)
		}
	}
	return Polygon(pol)
}

func SimplifyGeometry(geometry tegola.Geometry) (tegola.Geometry, error) {
	switch geo := geometry.(type) {
	//case tegola.Point, tegola.Point3, tegola.MultiPoint:
	default:
		// Nothing to simplify for Points.
		return CloneGeometry(geometry)
	case tegola.LineString:
		l := simplifyLine(geo, false)
		return l, nil
	case tegola.MultiLine:
		var ml []Line
		for _, l := range geo.Lines() {
			ln := simplifyLine(l, false)
			if ln != nil {
				ml = append(ml, ln)
			}
		}
		return MultiLine(ml), nil
	case tegola.Polygon:
		return simplifyPolygon(geo), nil
	case tegola.MultiPolygon:
		var mpol []Polygon
		for _, p := range geo.Polygons() {
			mpol = append(mpol, simplifyPolygon(p))
		}
		mpl := MultiPolygon(mpol)
		return &mpl, nil
	}
}
