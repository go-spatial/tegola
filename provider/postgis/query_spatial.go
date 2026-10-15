package postgis

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

func (p *Provider) resolveFeatureMetadata(ctx context.Context, f *featureProfile, conf dict.Dicter) error {
	geometry, ok := f.column(f.geometry)
	if !ok || geometry.generated != "" {
		return featureUnsupported("requires a direct ordinary geometry source column")
	}
	if f.format == "" {
		if geometry.typeName != "geometry" || geometry.extension != "postgis" || geometry.typmod < 0 {
			return featureUnsupported("native geometry requires authoritative PostGIS typmod metadata")
		}
		if err := p.pool.QueryRow(ctx, `SELECT n.nspname FROM pg_catalog.pg_extension e JOIN pg_catalog.pg_namespace n ON n.oid=e.extnamespace WHERE e.extname='postgis'`).Scan(&f.postgisSchema); err != nil {
			return err
		}
		prefix := pgQuoteIdent(f.postgisSchema) + "."
		var srid, dimensions int
		var kind string
		statement := "SELECT " + prefix + "postgis_typmod_srid($1)," + prefix + "postgis_typmod_dims($1)," + prefix + "postgis_typmod_type($1)"
		if err := p.pool.QueryRow(ctx, statement, geometry.typmod).Scan(&srid, &dimensions, &kind); err != nil {
			return err
		}
		kind = strings.ToUpper(kind)
		base := strings.TrimSuffix(kind, "Z")
		switch base {
		case "POINT", "LINESTRING", "POLYGON", "MULTIPOINT", "MULTILINESTRING", "MULTIPOLYGON", "GEOMETRYCOLLECTION":
		default:
			return featureUnsupported("native geometry family is outside the feature profile")
		}
		if srid > 0 && !f.crsExplicit {
			f.srid = uint64(srid)
		}
		if srid <= 0 || uint64(srid) != f.srid {
			return featureUnsupported("native source SRID differs from the resolved layer CRS")
		}
		nativeDimension := provider.DimensionXY
		if dimensions == 3 && strings.HasSuffix(kind, "Z") {
			nativeDimension = provider.DimensionXYZ
		} else if dimensions != 2 {
			return featureUnsupported("native dimensions lack proven XY/XYZ identity")
		}
		if _, explicit := conf.Interface("spatial_dimension"); explicit && f.spatial.Dimension != nativeDimension {
			return featureInvalid("spatial_dimension", "override disagrees with native typmod")
		}
		f.spatial.Dimension = nativeDimension
	} else {
		switch f.format {
		case codec.FormatWKB, codec.FormatMOS:
			if geometry.oid != 17 {
				return featureUnsupported("binary geometry requires a bytea column")
			}
		case codec.FormatWKT:
			if geometry.oid != 25 && geometry.oid != 1043 && geometry.oid != 1042 {
				return featureUnsupported("WKT geometry requires a text column")
			}
		default:
			return featureUnsupported("unknown geometry storage format")
		}
	}
	if f.format == codec.FormatMOS && f.spatial.Dimension != provider.DimensionXY {
		return featureUnsupported("MOS stores XY only")
	}
	if f.spatial.Dimension != provider.DimensionXY && f.spatial.VerticalCRS == "" {
		return featureUnsupported("height source requires an explicit vertical reference")
	}
	if f.spatial.VerticalCRS != "" && f.spatial.VerticalCRS != provider.CRS84h {
		return featureUnsupported("unsupported source height reference")
	}
	if err := f.spatial.Validate(); err != nil {
		return err
	}
	if f.spatial.Dimension != provider.DimensionXY {
		projection, err := crsconfig.NewHeightProjection(f.srid)
		if err != nil {
			return featureUnsupported("source height CRS is outside canonical height profiles")
		}
		f.height = projection
	} else {
		if _, err := basic.ToWebMercator(f.srid, geom.MultiPoint{}); err != nil {
			return featureUnsupported("unknown source CRS")
		}
	}
	for _, field := range []string{f.temporal.InstantField, f.temporal.StartField, f.temporal.EndField} {
		if field == "" {
			continue
		}
		column, ok := f.column(field)
		if !ok {
			return featureInvalid("temporal", "mapped output field does not exist")
		}
		if !featureInteger(column) {
			return featureUnsupported("temporal fields require ordinary integral POSIX columns")
		}
	}
	return nil
}

func (f *featureProfile) decodeFeatureGeometry(value any) (geom.Geometry, error) {
	if value == nil {
		return nil, nil
	}
	var geometry geom.Geometry
	var err error
	switch f.format {
	case codec.FormatWKT:
		geometry, err = codec.DecodeRawWKT(value)
	case codec.FormatMOS:
		geometry, err = decodeGeometryValue(value, f.format, f.mos)
	default:
		geometry, err = codec.DecodeRawWKB(value)
	}
	if err != nil {
		return nil, err
	}
	if err := validateFeatureDimension(geometry, f.spatial.Dimension); err != nil {
		return nil, err
	}
	if err := codec.ValidateFeatureSpatialGeometry(geometry); err != nil {
		return nil, err
	}
	if codec.FeatureSpatialGeometryEmpty(geometry) {
		return nil, nil
	}
	return geometry, nil
}

func validateFeatureDimension(g geom.Geometry, dimension provider.CoordinateDimension) error {
	if g == nil {
		return nil
	}
	if children, ok := g.(geom.Collection); ok {
		for _, child := range children {
			if err := validateFeatureDimension(child, dimension); err != nil {
				return err
			}
		}
		return nil
	}
	xyz := false
	switch g.(type) {
	case geom.PointZ, geom.MultiPointZ, geom.LineStringZ, geom.MultiLineStringZ, geom.PolygonZ, codec.MultiPolygonZ:
		xyz = true
	}
	if dimension == provider.DimensionXY && xyz || dimension == provider.DimensionXYZ && !xyz {
		return featureInvalid("geometry", "body disagrees with declared source dimensions")
	}
	return nil
}

func (f *featureProfile) matchesFeatureBounds(g geom.Geometry, q provider.FeatureQuery) (bool, error) {
	if len(q.Bounds)+len(q.Bounds3D) == 0 {
		return true, nil
	}
	transformed := g
	if g != nil && f.queryProjection != nil && !f.projection.Equivalent(f.queryProjection) {
		var err error
		transformed, err = codec.TransformFeatureSpatialGeometry(g, func(point [2]float64) ([2]float64, error) {
			ll, err := f.projection.Inverse(point[:])
			if err != nil {
				return [2]float64{}, err
			}
			xy, err := f.queryProjection.Forward(ll)
			if err != nil {
				return [2]float64{}, err
			}
			return [2]float64{xy[0], xy[1]}, nil
		})
		if err != nil {
			return false, fmt.Errorf("postgis frozen query-frame transform: %w", err)
		}
	} else if g != nil && f.queryProjection == nil && f.srid != q.BoundsSRID {
		var err error
		if f.height != nil {
			target, projectionErr := crsconfig.NewHeightProjection(q.BoundsSRID)
			if projectionErr != nil {
				return false, featureUnsupported("query height CRS is unsupported")
			}
			transformed, err = codec.TransformFeatureSpatialGeometry(g, func(point [2]float64) ([2]float64, error) {
				ll, err := f.height.Inverse(point[:])
				if err != nil {
					return [2]float64{}, err
				}
				xy, err := target.Forward(ll)
				if err != nil {
					return [2]float64{}, err
				}
				return [2]float64{xy[0], xy[1]}, nil
			})
		} else {
			mercator, transformErr := basic.ToWebMercator(f.srid, g)
			if transformErr != nil {
				return false, transformErr
			}
			transformed, err = basic.FromWebMercator(q.BoundsSRID, mercator)
		}
		if err != nil {
			return false, fmt.Errorf("postgis exact query-frame transform: %w", err)
		}
	}
	matched := false
	for _, bounds := range q.Bounds3D {
		box := [6]float64(bounds)
		ok, err := codec.FeatureGeometryIntersectsExtent3D(transformed, &box)
		if err != nil {
			return false, err
		}
		matched = matched || ok
	}
	if len(q.Bounds) > 0 {
		xy, err := codec.FeatureGeometryXYProjection(transformed)
		if err != nil {
			return false, err
		}
		for _, bounds := range q.Bounds {
			extent := bounds
			ok, err := codec.FeatureGeometryIntersectsExtent(xy, &extent)
			if err != nil {
				return false, err
			}
			matched = matched || ok
		}
	}
	return matched, nil
}
