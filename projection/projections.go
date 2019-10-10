package projection

import (
	"fmt"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/proj"
	"github.com/go-spatial/proj/core"
	"github.com/go-spatial/proj/support"
)

var p4326InvOp3857 = get4326InvOp("+proj=merc +a=6378137 +b=6378137 +lat_ts=0.0 +lon_0=0.0 +x_0=0.0 +y_0=0 +k=1.0")

//TODO (meilinger): Can remove this and use the proj.Inverse when PR (https://github.com/go-spatial/proj/pull/32) is approved
func get4326InvOp(str string) func(...float64) ([]float64, error) {
	ps, err := support.NewProjString(str)
	if err != nil {
		panic(err)
	}
	_, opx, err := core.NewSystem(ps)
	if err != nil {
		panic(err)
	}
	op := opx.(core.IConvertLPToXY)

	fn := func(c ...float64) ([]float64, error) {
		input := &core.CoordXY{X: c[0], Y: c[1]}
		output, err := op.Inverse(input)

		return []float64{support.RToDD(output.Lam), support.RToDD(output.Phi)}, err
	}

	return fn
}

func convertWrapper(destSRID uint64) func(...float64) ([]float64, error) {
	return func(c ...float64) ([]float64, error) {
		return proj.Convert(proj.EPSGCode(destSRID), []float64{c[0], c[1]})
	}
}

//ConvertGeom will project/unproject the given geometry from the sourceSRID, using 4326 as an intermediary step -- as necessary, to the destSRID
func ConvertGeom(destSRID uint64, sourceSRID uint64, geometry geom.Geometry) (geom.Geometry, error) {
	if sourceSRID == destSRID {
		return CloneGeometry(geometry)
	}

	switch sourceSRID {
	default:
		return nil, fmt.Errorf("don't know how to convert from %v to %v.", sourceSRID, destSRID)
	case 4326:
		return ApplyToPoints(geometry, convertWrapper(destSRID))
	case 3857:
		geom4326, err := ApplyToPoints(geometry, p4326InvOp3857)

		if err != nil {
			panic(err)
		}

		if destSRID != 4326 {
			return ApplyToPoints(geom4326, convertWrapper(destSRID))
		}

		return geom4326, err
	}
}

// ApplyToPoints applys the given function to each point in the geometry and any sub geometries, return a new transformed geometry.
func ApplyToPoints(geometry geom.Geometry, f func(coords ...float64) ([]float64, error)) (geom.Geometry, error) {
	switch geo := geometry.(type) {
	default:
		return nil, fmt.Errorf("unknown Geometry: %T", geometry)

	case geom.Point:
		c, err := f(geo.X(), geo.Y())
		if err != nil {
			return nil, err
		}
		if len(c) < 2 {
			return nil, fmt.Errorf("function did not return minimum number of coordinates got %v expected 2", len(c))
		}
		return geom.Point{c[0], c[1]}, nil

	case geom.MultiPoint:
		pts := make(geom.MultiPoint, len(geo))

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

	case geom.LineString:
		line := make(geom.LineString, len(geo))
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

	case geom.MultiLineString:
		lines := make(geom.MultiLineString, len(geo))

		for i, line := range geo {
			// getting a geometry interface back
			linei, err := ApplyToPoints(geom.LineString(line), f)
			if err != nil {
				return nil, fmt.Errorf("got error converting line(%v) of multiline: %v", i, err)
			}

			// get the value
			linev, ok := linei.(geom.LineString)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			lines[i] = linev
		}
		return lines, nil

	case geom.Polygon:
		poly := make(geom.Polygon, len(geo))

		for i, line := range geo {
			// getting a geometry inteface back
			linei, err := ApplyToPoints(geom.LineString(line), f)
			if err != nil {
				return nil, fmt.Errorf("got error converting line(%v) of polygon: %v", i, err)
			}

			// get the value
			linev, ok := linei.(geom.LineString)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			poly[i] = linev
		}
		return poly, nil

	case geom.MultiPolygon:
		mpoly := make(geom.MultiPolygon, len(geo))

		for i, poly := range geo {
			// getting a geometry inteface back
			polyi, err := ApplyToPoints(geom.Polygon(poly), f)
			if err != nil {
				return nil, fmt.Errorf("got error converting poly(%v) of multipolygon: %v", i, err)
			}

			// get the value
			polyv, ok := polyi.(geom.Polygon)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			mpoly[i] = polyv
		}
		return mpoly, nil
	}
}

// CloneGeomtry returns a deep clone of the Geometry.
func CloneGeometry(geometry geom.Geometry) (geom.Geometry, error) {
	switch geo := geometry.(type) {
	default:
		return nil, fmt.Errorf("unknown Geometry: %T", geometry)

	case geom.Point:
		return geom.Point{geo.X(), geo.Y()}, nil

	case geom.MultiPoint:
		pts := make(geom.MultiPoint, len(geo))
		for i, pt := range geo {
			pts[i] = pt
		}
		return pts, nil

	case geom.LineString:
		line := make(geom.LineString, len(geo))
		for i, pt := range geo {
			line[i] = pt
		}
		return line, nil

	case geom.MultiLineString:
		lines := make(geom.MultiLineString, len(geo))
		for i, line := range geo {
			// getting a geometry interface back
			linei, err := CloneGeometry(geom.LineString(line))
			if err != nil {
				return nil, fmt.Errorf("got error converting line(%v) of multiline: %v", i, err)
			}

			// get the value
			linev, ok := linei.(geom.LineString)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			lines[i] = linev
		}
		return lines, nil

	case geom.Polygon:
		// getting a geometry inteface back
		poly := make(geom.Polygon, len(geo))
		for i, line := range geo {
			linei, err := CloneGeometry(geom.LineString(line))
			if err != nil {
				return nil, fmt.Errorf("got error converting line(%v) of polygon: %v", i, err)
			}

			// get the value
			linev, ok := linei.(geom.LineString)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			poly[i] = linev
		}
		return poly, nil

	case geom.MultiPolygon:
		mpoly := make(geom.MultiPolygon, len(geo))
		for i, poly := range geo {
			// getting a geometry inteface back
			polyi, err := CloneGeometry(geom.Polygon(poly))
			if err != nil {
				return nil, fmt.Errorf("got error converting polygon(%v) of multipolygon: %v", i, err)
			}

			// get the value
			polyv, ok := polyi.(geom.Polygon)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			mpoly[i] = polyv
		}
		return mpoly, nil
	}
}
