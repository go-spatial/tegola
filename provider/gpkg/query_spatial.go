//go:build cgo

package gpkg

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

func (p *Provider) registerSpatial(layer *Layer, conf dict.Dicter) error {
	meta := provider.SpatialMetadata{Dimension: provider.DimensionXY}
	dimension, explicitDimension := conf.Interface("spatial_dimension")
	vertical, explicitVertical := conf.Interface("vertical_crs")
	if explicitDimension {
		switch dimension {
		case "xy":
			meta.Dimension = provider.DimensionXY
		case "xyz":
			meta.Dimension = provider.DimensionXYZ
		case "mixed_xy_xyz":
			meta.Dimension = provider.DimensionMixedXYXYZ
		default:
			return invalidQuery("spatial_dimension", "must be xy, xyz or mixed_xy_xyz")
		}
	}
	if explicitVertical {
		var ok bool
		meta.VerticalCRS, ok = vertical.(string)
		if !ok || strings.TrimSpace(meta.VerticalCRS) == "" {
			return invalidQuery("vertical_crs", "must be a nonblank CRS string")
		}
	}
	if explicitDimension && meta.Dimension != provider.DimensionXY && !explicitVertical {
		return invalidQuery("vertical_crs", "explicit XYZ/mixed source requires a height reference")
	}
	if (explicitDimension || (layer.geometryFormat != "" && layer.geometryFormat != GeometryFormatGPKG)) && meta.Dimension == provider.DimensionXY && explicitVertical {
		return invalidQuery("vertical_crs", "XY source cannot declare height")
	}
	native := layer.geometryFormat == "" || layer.geometryFormat == GeometryFormatGPKG
	if native && layer.tablename != "" {
		var z, m int
		var sourceSRID sql.NullInt64
		err := p.db.QueryRow("SELECT z,m,srs_id FROM gpkg_geometry_columns WHERE table_name=? AND column_name=? COLLATE NOCASE", layer.tablename, layer.geomFieldname).Scan(&z, &m, &sourceSRID)
		if err != nil || z < 0 || z > 2 || m < 0 || m > 2 {
			layer.spatialError = fmt.Errorf("gpkg native dimensional schema unknown or invalid: %w", provider.ErrUnsupported)
			return nil
		}
		layer.nativeZ, layer.nativeM = z, m
		inferred := []provider.CoordinateDimension{provider.DimensionXY, provider.DimensionXYZ, provider.DimensionMixedXYXYZ}[z]
		if explicitDimension && meta.Dimension != inferred {
			return invalidQuery("spatial_dimension", "override disagrees with native Z schema")
		}
		meta.Dimension = inferred
		if inferred != provider.DimensionXY && !layer.crsExplicit && (!sourceSRID.Valid || sourceSRID.Int64 <= 0) {
			layer.spatialError = fmt.Errorf("gpkg height source horizontal CRS is unknown: %w", provider.ErrUnsupported)
		}
		if m == 1 {
			layer.spatialError = fmt.Errorf("gpkg mandatory measure profile: %w", provider.ErrUnsupported)
		}
	}
	if meta.Dimension == provider.DimensionXY && explicitVertical {
		return invalidQuery("vertical_crs", "XY source cannot declare height")
	}
	if meta.Dimension != provider.DimensionXY && !explicitVertical {
		layer.spatialError = errors.Join(layer.spatialError, fmt.Errorf("gpkg height source lacks vertical reference: %w", provider.ErrUnsupported))
	} else if meta.VerticalCRS != "" && meta.VerticalCRS != provider.CRS84h {
		layer.spatialError = errors.Join(layer.spatialError, fmt.Errorf("gpkg source height reference: %w", provider.ErrUnsupported))
	} else if err := meta.Validate(); err != nil {
		return err
	}
	if layer.geometryFormat == GeometryFormatMOS && meta.Dimension != provider.DimensionXY {
		layer.spatialError = errors.Join(layer.spatialError, fmt.Errorf("MOS stores XY only: %w", provider.ErrUnsupported))
	}
	if meta.Dimension != provider.DimensionXY && layer.spatialError == nil {
		projection, err := crsconfig.NewHeightProjection(resolvedLayerSRID(layer))
		if err != nil {
			layer.spatialError = fmt.Errorf("gpkg source height projection: %w: %v", provider.ErrUnsupported, err)
		} else {
			layer.heightProjection = projection
		}
	}
	layer.spatialMetadata = meta
	return nil
}

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
		return invalidQuery("spatial_dimension", "source body disagrees with declared dimension")
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

func decodeRawGeometryValue(value any, layer *Layer) (geom.Geometry, error) {
	var geometry geom.Geometry
	var err error
	switch layer.geometryFormat {
	case GeometryFormatWKB:
		geometry, err = codec.DecodeRawWKB(value)
	case GeometryFormatWKT:
		geometry, err = codec.DecodeRawWKT(value)
	case GeometryFormatMOS:
		_, geometry, err = decodeGeometryValue(value, layer.geometryFormat, layer.mosConfig)
	default:
		data, ok := value.([]byte)
		if !ok {
			return nil, fmt.Errorf("native raw geometry requires bytes")
		}
		header, headerErr := NewBinaryHeader(data)
		if headerErr != nil {
			return nil, headerErr
		}
		if !header.IsStandardGeometry() {
			return nil, fmt.Errorf("extended native geometry: %w", provider.ErrUnsupported)
		}
		for i, value := range header.Envelope() {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, invalidQuery("geometry", "native envelope is nonfinite")
			}
			if i%2 == 0 && value > header.Envelope()[i+1] {
				return nil, invalidQuery("geometry", "native envelope minimum exceeds maximum")
			}
		}
		geometry, err = codec.DecodeRawWKB(data[header.Size():])
		if err != nil {
			return nil, err
		}
		if header.IsGeometryEmpty() != spatialGeometryEmpty(geometry) {
			return nil, invalidQuery("geometry", "native empty flag disagrees with body")
		}
		if header.EnvelopeType() == EnvelopeTypeXYM || header.EnvelopeType() == EnvelopeTypeXYZM {
			return nil, fmt.Errorf("native measure envelope: %w", provider.ErrUnsupported)
		}
		if header.EnvelopeType() == EnvelopeTypeXYZ && !spatialGeometryEmpty(geometry) {
			if err := validateSourceDimension(geometry, provider.DimensionXYZ); err != nil {
				return nil, err
			}
		}
	}
	if err != nil {
		return nil, err
	}
	if err := validateSourceDimension(geometry, layer.spatialMetadata.Dimension); err != nil {
		return nil, err
	}
	if err := codec.ValidateFeatureSpatialGeometry(geometry); err != nil {
		return nil, wrapSpatialError(err)
	}
	if spatialGeometryEmpty(geometry) {
		return nil, nil
	}
	return geometry, nil
}

type spatialQuery struct {
	source, target             *crsconfig.HeightProjection
	pinnedSource, pinnedTarget *crsconfig.FeatureProjection
}

func newSpatialQuery(layer *Layer, query provider.FeatureQuery) (spatialQuery, error) {
	result := spatialQuery{source: layer.heightProjection}
	if len(query.Bounds3D) != 0 && query.BoundsVerticalCRS != provider.CRS84h {
		return result, fmt.Errorf("query height reference: %w", provider.ErrUnsupported)
	}
	if query.BoundsCRSDefinition != "" {
		if layer.featureCRSProjection == nil || layer.featureCRSError != nil {
			return result, fmt.Errorf("source CRS proof unavailable: %w", provider.ErrUnsupported)
		}
		target, err := crsconfig.NewFeatureProjection(query.BoundsCRSDefinition)
		if err != nil {
			return result, fmt.Errorf("query CRS definition unsupported: %w", provider.ErrUnsupported)
		}
		result.pinnedSource, result.pinnedTarget = layer.featureCRSProjection, target
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
	transform := source != query.BoundsSRID
	if s.pinnedSource != nil {
		transform = !s.pinnedSource.Equivalent(s.pinnedTarget)
	}
	if transform {
		// A single vertex outside the target's finite forward domain (the
		// poles in Web Mercator) must not fail the whole bbox predicate.
		// Retry it clamped to the exact canonical-extent latitude; only this
		// predicate copy is clamped, never the delivered geometry.
		forwardWithDomainClamp := func(forward func([]float64) ([]float64, error), srid uint64, ll []float64) ([]float64, error) {
			xy, err := forward(ll)
			if err == nil {
				return xy, nil
			}
			clamped, ok := crsconfig.ClampForwardDomain(srid, ll)
			if !ok {
				return nil, err
			}
			if xy, cerr := forward(clamped); cerr == nil {
				return xy, nil
			}
			return nil, err
		}
		if s.pinnedSource != nil {
			transformed, err = codec.TransformFeatureSpatialGeometry(g, func(p [2]float64) ([2]float64, error) {
				ll, err := s.pinnedSource.Inverse(p[:])
				if err != nil {
					return [2]float64{}, err
				}
				xy, err := forwardWithDomainClamp(s.pinnedTarget.Forward, s.pinnedTarget.CanonicalSRID(), ll)
				if err != nil {
					return [2]float64{}, err
				}
				return [2]float64{xy[0], xy[1]}, nil
			})
		} else if s.source != nil {
			transformed, err = codec.TransformFeatureSpatialGeometry(g, func(p [2]float64) ([2]float64, error) {
				ll, err := s.source.Inverse(p[:])
				if err != nil {
					return [2]float64{}, err
				}
				xy, err := forwardWithDomainClamp(s.target.Forward, query.BoundsSRID, ll)
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

// NaN point tuples are decoded absence even with misleading raw bounds. Inspect
// exponent and nonzero mantissa bits without SQL floating-point conversion.
func rawEmptyPointSQL(field string) string {
	nan := func(offset int, little bool) string {
		h := fmt.Sprintf("hex(substr(%s,%d,8))", field, offset)
		if little {
			return "(substr(" + h + ",15,2) IN ('7F','FF') AND substr(" + h + ",13,1)='F' AND (substr(" + h + ",14,1)<>'0' OR substr(" + h + ",1,12)<>'000000000000'))"
		}
		return "(substr(" + h + ",1,2) IN ('7F','FF') AND substr(" + h + ",3,1)='F' AND substr(" + h + ",4,13)<>'0000000000000')"
	}
	return "(hex(substr(" + field + ",1,5))='0101000000' AND " + nan(6, true) + " AND " + nan(14, true) + ") OR (hex(substr(" + field + ",1,5))='0000000001' AND " + nan(6, false) + " AND " + nan(14, false) + ")"
}
