package planar

import (
	"context"

	"github.com/go-spatial/geom"
)

func simplifyPolygon(ctx context.Context, simplifer Simplifer, plg [][][2]float64, isClosed bool) (ret [][][2]float64, err error) {
	ret = make([][][2]float64, len(plg))
	for i := range plg {
		ls, err := simplifer.Simplify(ctx, plg[i], isClosed)
		if err != nil {
			return nil, err
		}
		if len(ls) > 2 || !isClosed {
			ret[i] = ls
		}
	}
	return ret, nil

}

// Simplify will simplify the provided geometry using the provided simplifer.
// If the simplifer is nil, no simplification will be attempted.
func Simplify(ctx context.Context, simplifer Simplifer, geometry geom.Geometry) (geom.Geometry, error) {

	if simplifer == nil {
		return geometry, nil
	}

	switch gg := geometry.(type) {

	case geom.Collectioner:

		geos := gg.Geometries()
		coll := make([]geom.Geometry, len(geos))
		for i := range geos {
			geo, err := Simplify(ctx, simplifer, geos[i])
			if err != nil {
				return nil, err
			}
			coll[i] = geo
		}
		return geom.Collection(coll), nil

	case geom.MultiPolygoner:

		plys := gg.Polygons()
		mply := make([][][][2]float64, len(plys))
		for i := range plys {
			ply, err := simplifyPolygon(ctx, simplifer, plys[i], true)
			if err != nil {
				return nil, err
			}
			mply[i] = ply
		}
		return geom.MultiPolygon(mply), nil

	case geom.Polygoner:

		ply, err := simplifyPolygon(ctx, simplifer, gg.LinearRings(), true)
		if err != nil {
			return nil, err
		}
		return geom.Polygon(ply), nil

	case geom.MultiLineStringer:

		mls, err := simplifyPolygon(ctx, simplifer, gg.LineStrings(), false)
		if err != nil {
			return nil, err
		}
		return geom.MultiLineString(mls), nil

	case geom.LineStringer:

		ls, err := simplifer.Simplify(ctx, gg.Vertices(), false)
		if err != nil {
			return nil, err
		}
		return geom.LineString(ls), nil

	default: // Points, MutliPoints or anything else.
		return geometry, nil

	}
}
