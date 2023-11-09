package geom

import "fmt"

// ApplyToPoints applys the given function to each point in the geometry and any sub geometries, return a new transformed geometry.
func ApplyToPoints(geometry Geometry, f func(coords ...float64) ([]float64, error)) (Geometry, error) {
	switch geo := geometry.(type) {
	default:
		return nil, fmt.Errorf("unknown Geometry: %T", geometry)

	case Point:
		c, err := f(geo.X(), geo.Y())
		if err != nil {
			return nil, err
		}
		if len(c) < 2 {
			return nil, fmt.Errorf("function did not return minimum number of coordinates got %v expected 2", len(c))
		}
		return Point{c[0], c[1]}, nil

	case MultiPoint:
		pts := make(MultiPoint, len(geo))

		for i, pt := range geo {
			c, err := f(pt[:]...)
			if err != nil {
				return nil, err
			}
			if len(c) < 2 {
				return nil, fmt.Errorf("function did not return minimum number of coordinates got %v expected 2", len(c))
			}
			pts[i][0], pts[i][1] = c[0], c[1]
		}
		return pts, nil

	case LineString:
		line := make(LineString, len(geo))
		for i, pt := range geo {
			c, err := f(pt[:]...)
			if err != nil {
				return nil, err
			}
			if len(c) < 2 {
				return nil, fmt.Errorf("function did not return minimum number of coordinates got %v expected 2", len(c))
			}
			line[i][0], line[i][1] = c[0], c[1]
		}
		return line, nil

	case MultiLineString:
		lines := make(MultiLineString, len(geo))

		for i, line := range geo {
			// getting a geometry interface back
			linei, err := ApplyToPoints(LineString(line), f)
			if err != nil {
				return nil, fmt.Errorf("got error converting line(%v) of multiline: %v", i, err)
			}

			// get the value
			linev, ok := linei.(LineString)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			lines[i] = linev
		}
		return lines, nil

	case Polygon:
		poly := make(Polygon, len(geo))

		for i, line := range geo {
			// getting a geometry inteface back
			linei, err := ApplyToPoints(LineString(line), f)
			if err != nil {
				return nil, fmt.Errorf("got error converting line(%v) of polygon: %v", i, err)
			}

			// get the value
			linev, ok := linei.(LineString)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			poly[i] = linev
		}
		return poly, nil

	case MultiPolygon:
		mpoly := make(MultiPolygon, len(geo))

		for i, poly := range geo {
			// getting a geometry inteface back
			polyi, err := ApplyToPoints(Polygon(poly), f)
			if err != nil {
				return nil, fmt.Errorf("got error converting poly(%v) of multipolygon: %v", i, err)
			}

			// get the value
			polyv, ok := polyi.(Polygon)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			mpoly[i] = polyv
		}
		return mpoly, nil
	}
}

// Clone returns a deep clone of the Geometry.
func Clone(geometry Geometry) (Geometry, error) {
	switch geo := geometry.(type) {
	default:
		return nil, fmt.Errorf("unknown Geometry: %T", geometry)

	case Point:
		return Point{geo.X(), geo.Y()}, nil

	case MultiPoint:
		pts := make(MultiPoint, len(geo))
		for i, pt := range geo {
			pts[i] = pt
		}
		return pts, nil

	case LineString:
		line := make(LineString, len(geo))
		for i, pt := range geo {
			line[i] = pt
		}
		return line, nil

	case MultiLineString:
		lines := make(MultiLineString, len(geo))
		for i, line := range geo {
			// getting a geometry interface back
			linei, err := Clone(LineString(line))
			if err != nil {
				return nil, fmt.Errorf("got error converting line(%v) of multiline: %v", i, err)
			}

			// get the value
			linev, ok := linei.(LineString)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			lines[i] = linev
		}
		return lines, nil

	case Polygon:
		// getting a geometry inteface back
		poly := make(Polygon, len(geo))
		for i, line := range geo {
			linei, err := Clone(LineString(line))
			if err != nil {
				return nil, fmt.Errorf("got error converting line(%v) of polygon: %v", i, err)
			}

			// get the value
			linev, ok := linei.(LineString)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			poly[i] = linev
		}
		return poly, nil

	case MultiPolygon:
		mpoly := make(MultiPolygon, len(geo))
		for i, poly := range geo {
			// getting a geometry inteface back
			polyi, err := Clone(Polygon(poly))
			if err != nil {
				return nil, fmt.Errorf("got error converting polygon(%v) of multipolygon: %v", i, err)
			}

			// get the value
			polyv, ok := polyi.(Polygon)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			mpoly[i] = polyv
		}
		return mpoly, nil
	}
}
