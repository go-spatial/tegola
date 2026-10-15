package geometrycodec

import (
	"fmt"

	"github.com/alexeydott/geom"
)

// TransformFeatureSpatialGeometry transforms XY on an independently owned copy
// while preserving Z exactly. All source and output children are validated,
// including query-frame polygon planarity. The callback must not invent height.
func TransformFeatureSpatialGeometry(
	g geom.Geometry,
	transform func([2]float64) ([2]float64, error),
) (geom.Geometry, error) {
	if transform == nil {
		return nil, fmt.Errorf("feature spatial transform is nil")
	}
	if err := ValidateFeatureSpatialGeometry(g); err != nil {
		return nil, err
	}
	out, err := transformSpatialGeometry(g, transform)
	if err != nil {
		return nil, err
	}
	if err := ValidateFeatureSpatialGeometry(out); err != nil {
		return nil, err
	}
	return out, nil
}

func transformSpatialPoints[P ~[2]float64 | ~[3]float64](
	points []P,
	transform func([2]float64) ([2]float64, error),
) ([]P, error) {
	out := make([]P, len(points))
	for i, point := range points {
		xy, err := transform([2]float64{point[0], point[1]})
		if err != nil {
			return nil, fmt.Errorf("feature coordinate transform: %w", err)
		}
		out[i] = point
		out[i][0], out[i][1] = xy[0], xy[1]
	}
	return out, nil
}

func transformSpatialGeometry(g geom.Geometry, transform func([2]float64) ([2]float64, error)) (geom.Geometry, error) {
	switch v := g.(type) {
	case nil:
		return nil, nil
	case geom.Point:
		out, err := transformSpatialPoints([][2]float64{v}, transform)
		if err != nil {
			return nil, err
		}
		return geom.Point(out[0]), nil
	case geom.PointZ:
		out, err := transformSpatialPoints([][3]float64{v}, transform)
		if err != nil {
			return nil, err
		}
		return geom.PointZ(out[0]), nil
	case geom.MultiPoint:
		out, err := transformSpatialPoints([][2]float64(v), transform)
		return geom.MultiPoint(out), err
	case geom.MultiPointZ:
		out, err := transformSpatialPoints([][3]float64(v), transform)
		return geom.MultiPointZ(out), err
	case geom.LineString:
		out, err := transformSpatialPoints([][2]float64(v), transform)
		return geom.LineString(out), err
	case geom.LineStringZ:
		out, err := transformSpatialPoints([][3]float64(v), transform)
		return geom.LineStringZ(out), err
	case geom.MultiLineString:
		out, err := transformSpatialRings(v, transform)
		return geom.MultiLineString(out), err
	case geom.MultiLineStringZ:
		out, err := transformSpatialRings(v, transform)
		return geom.MultiLineStringZ(out), err
	case geom.Polygon:
		out, err := transformSpatialRings(v, transform)
		return geom.Polygon(out), err
	case geom.PolygonZ:
		out, err := transformSpatialRings(v, transform)
		return geom.PolygonZ(out), err
	case geom.MultiPolygon:
		out := make(geom.MultiPolygon, len(v))
		for i, polygon := range v {
			var err error
			out[i], err = transformSpatialRings(polygon, transform)
			if err != nil {
				return nil, err
			}
		}
		return out, nil
	case MultiPolygonZ:
		out := make(MultiPolygonZ, len(v))
		for i, polygon := range v {
			var err error
			out[i], err = transformSpatialRings(polygon, transform)
			if err != nil {
				return nil, err
			}
		}
		return out, nil
	case geom.Collection:
		out := make(geom.Collection, len(v))
		for i, child := range v {
			var err error
			out[i], err = transformSpatialGeometry(child, transform)
			if err != nil {
				return nil, err
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%w: %T", ErrUnsupportedFeatureSpatialGeometry, g)
	}
}

func transformSpatialRings[P ~[2]float64 | ~[3]float64](
	rings [][]P,
	transform func([2]float64) ([2]float64, error),
) ([][]P, error) {
	out := make([][]P, len(rings))
	for i, ring := range rings {
		var err error
		out[i], err = transformSpatialPoints(ring, transform)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
