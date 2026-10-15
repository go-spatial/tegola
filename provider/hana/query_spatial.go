package hana

import (
	"errors"
	"fmt"
	"github.com/alexeydott/geom"
	"github.com/alexeydott/proj"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

func validateSourceDimension(g geom.Geometry, dimension provider.CoordinateDimension) error {
	if g == nil || spatialGeometryEmpty(g) {
		return nil
	}
	if children, ok := g.(geom.Collection); ok {
		for _, child := range children {
			if err := validateSourceDimension(child, dimension); err != nil {
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
	if (dimension == provider.DimensionXY && xyz) || (dimension == provider.DimensionXYZ && !xyz) {
		return featureInvalid("spatial_dimension", "source body disagrees with declared dimension")
	}
	return codec.ValidateFeatureSpatialGeometry(g)
}

func spatialGeometryEmpty(g geom.Geometry) bool {
	switch v := g.(type) {
	case nil:
		return true
	case geom.MultiPoint:
		return len(v) == 0
	case geom.MultiPointZ:
		return len(v) == 0
	case geom.LineString:
		return len(v) == 0
	case geom.LineStringZ:
		return len(v) == 0
	case geom.Polygon:
		return len(v) == 0
	case geom.PolygonZ:
		return len(v) == 0
	case geom.MultiPolygon:
		for _, p := range v {
			if len(p) != 0 {
				return false
			}
		}
		return true
	case codec.MultiPolygonZ:
		for _, p := range v {
			if len(p) != 0 {
				return false
			}
		}
		return true
	case geom.MultiLineString:
		for _, l := range v {
			if len(l) != 0 {
				return false
			}
		}
		return true
	case geom.MultiLineStringZ:
		for _, l := range v {
			if len(l) != 0 {
				return false
			}
		}
		return true
	case geom.Collection:
		for _, c := range v {
			if !spatialGeometryEmpty(c) {
				return false
			}
		}
		return true
	}
	return false
}

type spatialQuery struct {
	source, target             *crsconfig.HeightProjection
	frozenSource, frozenTarget *crsconfig.FeatureProjection
}

func newSpatialQuery(layer *Layer, query provider.FeatureQuery) (spatialQuery, error) {
	result := spatialQuery{source: layer.feature.Height}
	if len(query.Bounds3D) != 0 && query.BoundsVerticalCRS != provider.CRS84h {
		return result, fmt.Errorf("query height reference: %w", provider.ErrUnsupported)
	}
	if query.BoundsCRSDefinition != "" {
		if layer.feature.Projection == nil {
			return result, featureUnsupported("immutable source CRS proof unavailable")
		}
		target, err := crsconfig.NewFeatureProjection(query.BoundsCRSDefinition)
		if err != nil {
			return result, featureUnsupported("immutable query CRS profile unsupported")
		}
		result.frozenSource, result.frozenTarget = layer.feature.Projection, target
		return result, nil
	}
	if result.source != nil && (len(query.Bounds) != 0 || len(query.Bounds3D) != 0) {
		target, err := crsconfig.NewHeightProjection(query.BoundsSRID)
		if err != nil {
			return result, fmt.Errorf("query height projection: %w: %v", provider.ErrUnsupported, err)
		}
		result.target = target
	}
	return result, nil
}

func (s spatialQuery) matches(g geom.Geometry, source uint64, query provider.FeatureQuery) (bool, error) {
	if len(query.Bounds) == 0 && len(query.Bounds3D) == 0 {
		return true, wrapSpatialError(codec.ValidateFeatureSpatialGeometry(g))
	}
	transformed := g
	var err error
	if s.frozenTarget != nil && !s.frozenSource.Equivalent(s.frozenTarget) {
		transformed, err = codec.TransformFeatureSpatialGeometry(g, func(point [2]float64) ([2]float64, error) {
			ll, err := s.frozenSource.Inverse(point[:])
			if err != nil {
				return [2]float64{}, err
			}
			xy, err := s.frozenTarget.Forward(ll)
			if err != nil {
				return [2]float64{}, err
			}
			return [2]float64{xy[0], xy[1]}, nil
		})
	} else if s.frozenTarget == nil && source != query.BoundsSRID {
		if s.source != nil {
			transformed, err = codec.TransformFeatureSpatialGeometry(g, func(p [2]float64) ([2]float64, error) {
				ll, err := s.source.Inverse(p[:])
				if err != nil {
					return [2]float64{}, err
				}
				xy, err := s.target.Forward(ll)
				if err != nil {
					return [2]float64{}, err
				}
				return [2]float64{xy[0], xy[1]}, nil
			})
		} else {
			transformed, err = transformQueryGeometry(g, source, query.BoundsSRID)
		}
		if err != nil {
			return false, wrapSpatialError(err)
		}
	}
	if err != nil {
		return false, wrapSpatialError(err)
	}
	matched := false
	for _, b := range query.Bounds3D {
		box := [6]float64(b)
		ok, err := codec.FeatureGeometryIntersectsExtent3D(transformed, &box)
		if err != nil {
			return false, wrapSpatialError(err)
		}
		matched = matched || ok
	}
	if len(query.Bounds) != 0 {
		xy, err := codec.FeatureGeometryXYProjection(transformed)
		if err != nil {
			return false, wrapSpatialError(err)
		}
		for _, b := range query.Bounds {
			extent := b
			ok, err := codec.FeatureGeometryIntersectsExtent(xy, &extent)
			if err != nil {
				return false, err
			}
			matched = matched || ok
		}
	}
	return matched, nil
}

func wrapSpatialError(err error) error {
	if errors.Is(err, codec.ErrUnsupportedFeatureSpatialGeometry) || errors.Is(err, codec.ErrUnsupportedRawGeometry) {
		return errors.Join(provider.ErrUnsupported, err)
	}
	return err
}

func validateQueryCRS(srid uint64) error {
	if srid == 4326 {
		return nil
	}
	code := proj.EPSGCode(srid)
	if srid == 0 || code < 0 || uint64(code) != srid || !proj.IsKnownConversionSRID(code) {
		return fmt.Errorf("HANA query CRS profile: %w", provider.ErrUnsupported)
	}
	// Empty coordinate arrays construct the existing engine without testing
	// an arbitrary location outside a restricted geographic domain.
	converted, err := proj.Convert(code, []float64{})
	if err != nil {
		return fmt.Errorf("HANA query CRS conversion: %w: %v", provider.ErrUnsupported, err)
	}
	if _, err := proj.Inverse(code, converted); err != nil {
		return fmt.Errorf("HANA query CRS inverse conversion: %w: %v", provider.ErrUnsupported, err)
	}
	return nil
}

func transformQueryGeometry(geometry geom.Geometry, source, target uint64) (geom.Geometry, error) {
	if geometry == nil {
		return nil, nil
	}
	if source == target {
		return geometry, nil
	}
	mercator, err := basic.ToWebMercator(source, geometry)
	if err != nil {
		return nil, err
	}
	return basic.FromWebMercator(target, mercator)
}
