package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/proj"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/internal/featuresql"
)

func (f *featureProfile) registerSpatial(conf dict.Dicter, layer Layer) error {
	f.spatial.Dimension = provider.DimensionXY
	if raw, exists := conf.Interface("spatial_dimension"); exists {
		switch raw {
		case "xy":
		case "xyz":
			f.spatial.Dimension = provider.DimensionXYZ
		case "mixed_xy_xyz":
			f.spatial.Dimension = provider.DimensionMixedXYXYZ
		default:
			return featureInvalid("spatial_dimension", "must be xy, xyz or mixed_xy_xyz")
		}
	}
	if raw, exists := conf.Interface("vertical_crs"); exists {
		vertical, ok := raw.(string)
		if !ok || strings.TrimSpace(vertical) == "" {
			return featureInvalid("vertical_crs", "must be a nonblank string")
		}
		f.spatial.VerticalCRS = vertical
	}
	if f.spatial.Dimension == provider.DimensionXY && f.spatial.VerticalCRS != "" {
		return featureInvalid("vertical_crs", "XY source has no height reference")
	}
	if f.spatial.Dimension != provider.DimensionXY {
		if f.spatial.VerticalCRS == "" {
			return featureInvalid("vertical_crs", "XYZ/mixed requires height reference")
		}
		if f.spatial.VerticalCRS != provider.CRS84h {
			return featureUnsupported("vertical reference")
		}
		if f.format == GeometryFormatMOS {
			return featureUnsupported("MOS is XY")
		}
		if basic.IsSyntheticSRID(f.srid) {
			return featureUnsupported("custom height CRS")
		}
		height, err := crsconfig.NewHeightProjection(f.srid)
		if err != nil {
			return errors.Join(featureUnsupported("height projection"), err)
		}
		f.height = height
	}
	return f.spatial.Validate()
}

func (f *featureProfile) registerNative(ctx context.Context, db *sql.DB, flavor string, explicitCRS bool) error {
	sourceGeometry, exists := f.sourceColumn(f.geometry)
	column, ok := f.schema.column(sourceGeometry)
	if !ok || !exists {
		return featureUnsupported("geometry lineage")
	}
	nativeType := false
	switch strings.ToLower(column.dataType) {
	case "geometry", "point", "linestring", "polygon", "multipoint", "multilinestring", "multipolygon", "geometrycollection":
		nativeType = true
	}
	if f.format == GeometryFormatAuto {
		if !nativeType {
			return featureUnsupported("raw format must be explicit")
		}
		f.format = flavor
	}
	if f.format != GeometryFormatMySQL && f.format != GeometryFormatMariaDB {
		kind := featureColumnMetadata(column).Kind
		switch f.format {
		case GeometryFormatWKB, GeometryFormatMOS:
			if kind != featuresql.BinaryColumn {
				return featureUnsupported("raw binary storage requires binary column")
			}
		case GeometryFormatWKT:
			if kind != featuresql.StringColumn && kind != featuresql.BinaryColumn {
				return featureUnsupported("raw WKT storage requires text/binary column")
			}
		default:
			return featureUnsupported("unknown geometry storage")
		}
		return nil
	}
	if f.format != flavor {
		return featureUnsupported("native format disagrees with server flavor")
	}
	if !nativeType || f.spatial.Dimension != provider.DimensionXY {
		return featureUnsupported("native geometry requires proven XY storage")
	}
	var version string
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		return err
	}
	majorText := strings.SplitN(version, ".", 2)[0]
	major, err := strconv.Atoi(majorText)
	if err != nil {
		return featureUnsupported("unknown native server version")
	}
	// Initial native admission is bounded to independently documented server
	// families whose geometry storage is XY, not to a sampled row dimension.
	if flavor == GeometryFormatMySQL {
		if major != 8 {
			return featureUnsupported("native MySQL version not admitted")
		}
		f.nativeAxisOption = codec.IsGeographicSRID(f.srid)
		if !explicitCRS {
			srid := column.srid
			if !srid.Valid || srid.Int64 <= 0 {
				return featureUnsupported("native source CRS unconstrained")
			}
			f.srid = uint64(srid.Int64)
			f.nativeAxisOption = codec.IsGeographicSRID(f.srid)
		}
	} else {
		if !featureNativeMariaDBVersion(version) {
			return featureUnsupported("native MariaDB version not admitted")
		}
		if !explicitCRS {
			return featureUnsupported("native MariaDB requires explicit immutable CRS")
		}
	}
	f.native = true
	f.nativeSRIDLabel = "__tegola_feature_srid"
	for {
		_, collision := f.sourceColumn(f.nativeSRIDLabel)
		if !collision {
			break
		}
		f.nativeSRIDLabel += "_"
	}
	return nil
}

func featureNativeMariaDBVersion(version string) bool {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) != 3 {
		return false
	}
	patchText := strings.SplitN(parts[2], "-", 2)[0]
	for _, component := range []string{parts[0], parts[1], patchText} {
		if component == "" {
			return false
		}
		for _, digit := range component {
			if digit < '0' || digit > '9' {
				return false
			}
		}
	}
	major, majorErr := strconv.Atoi(parts[0])
	minor, minorErr := strconv.Atoi(parts[1])
	patch, patchErr := strconv.Atoi(patchText)
	if majorErr != nil || minorErr != nil || patchErr != nil || patch < 0 {
		return false
	}
	return (major == 10 && minor == 11) || (major == 11 && minor == 4) || (major == 10 && minor == 7 && patch == 4)
}

func validateFeatureDimension(g geom.Geometry, dimension provider.CoordinateDimension) error {
	if err := codec.ValidateFeatureSpatialGeometry(g); err != nil {
		return err
	}
	if featureGeometryEmpty(g) {
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
	if (dimension == provider.DimensionXY && xyz) || (dimension == provider.DimensionXYZ && !xyz) {
		return featureInvalid("spatial_dimension", "body disagrees with source profile")
	}
	return nil
}

func featureGeometryEmpty(g geom.Geometry) bool {
	if g == nil {
		return true
	}
	switch v := g.(type) {
	case geom.Collection:
		for _, child := range v {
			if !featureGeometryEmpty(child) {
				return false
			}
		}
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
	case geom.MultiLineString:
		for _, child := range v {
			if len(child) != 0 {
				return false
			}
		}
		return true
	case geom.MultiLineStringZ:
		for _, child := range v {
			if len(child) != 0 {
				return false
			}
		}
		return true
	case geom.MultiPolygon:
		for _, child := range v {
			if len(child) != 0 {
				return false
			}
		}
		return true
	case codec.MultiPolygonZ:
		for _, child := range v {
			if len(child) != 0 {
				return false
			}
		}
		return true
	}
	return false
}

type featureSpatialQuery struct {
	source, target             *crsconfig.HeightProjection
	pinnedSource, pinnedTarget *crsconfig.FeatureProjection
}

func newFeatureSpatialQuery(f *featureProfile, query provider.FeatureQuery) (featureSpatialQuery, error) {
	spatial := featureSpatialQuery{source: f.height}
	if len(query.Bounds3D) != 0 && query.BoundsVerticalCRS != provider.CRS84h {
		return spatial, featureUnsupported("requested vertical reference")
	}
	if len(query.Bounds)+len(query.Bounds3D) == 0 {
		return spatial, nil
	}
	if query.BoundsCRSDefinition != "" {
		if f.crsProjection == nil {
			return spatial, featureCRSUnsupported()
		}
		target, err := crsconfig.NewFeatureProjection(query.BoundsCRSDefinition)
		if err != nil {
			return spatial, featureUnsupported("requested CRS definition")
		}
		spatial.pinnedSource, spatial.pinnedTarget = f.crsProjection, target
		return spatial, nil
	}
	if f.height != nil {
		target, err := crsconfig.NewHeightProjection(query.BoundsSRID)
		if err != nil {
			return spatial, errors.Join(featureUnsupported("requested height projection"), err)
		}
		spatial.target = target
	} else if f.srid != query.BoundsSRID {
		for _, srid := range []uint64{f.srid, query.BoundsSRID} {
			if srid == 4326 {
				continue
			}
			code := proj.EPSGCode(srid)
			if srid == 0 || code < 0 || uint64(code) != srid || !proj.IsKnownConversionSRID(code) {
				return spatial, featureUnsupported("requested CRS")
			}
			if _, err := proj.Convert(code, []float64{}); err != nil {
				return spatial, errors.Join(featureUnsupported("requested projection"), err)
			}
			if _, err := proj.Inverse(code, []float64{}); err != nil {
				return spatial, errors.Join(featureUnsupported("requested inverse projection"), err)
			}
		}
	}
	return spatial, nil
}

func (s featureSpatialQuery) matches(g geom.Geometry, source uint64, query provider.FeatureQuery) (bool, error) {
	if len(query.Bounds)+len(query.Bounds3D) == 0 {
		return true, nil
	}
	transformed := g
	transform := source != query.BoundsSRID
	if s.pinnedSource != nil {
		transform = !s.pinnedSource.Equivalent(s.pinnedTarget)
	}
	if transform {
		var err error
		transformed, err = codec.TransformFeatureSpatialGeometry(g, func(p [2]float64) ([2]float64, error) {
			ll := []float64{p[0], p[1]}
			var err error
			if s.pinnedSource != nil {
				ll, err = s.pinnedSource.Inverse(ll)
			} else if s.source != nil {
				ll, err = s.source.Inverse(ll)
			} else if source != 4326 {
				ll, err = proj.Inverse(proj.EPSGCode(source), ll)
			}
			if err != nil {
				return [2]float64{}, err
			}
			xy := ll
			if s.pinnedTarget != nil {
				xy, err = s.pinnedTarget.Forward(ll)
			} else if s.target != nil {
				xy, err = s.target.Forward(ll)
			} else if query.BoundsSRID != 4326 {
				xy, err = proj.Convert(proj.EPSGCode(query.BoundsSRID), ll)
			}
			if err != nil {
				return [2]float64{}, err
			}
			if len(xy) != 2 || math.IsNaN(xy[0]) || math.IsNaN(xy[1]) || math.IsInf(xy[0], 0) || math.IsInf(xy[1], 0) {
				return [2]float64{}, fmt.Errorf("nonfinite projection")
			}
			return [2]float64{xy[0], xy[1]}, nil
		})
		if err != nil {
			return false, errors.Join(featureUnsupported("requested geometry transform"), err)
		}
	}
	matched := false
	for _, bounds := range query.Bounds3D {
		box := [6]float64(bounds)
		ok, err := codec.FeatureGeometryIntersectsExtent3D(transformed, &box)
		if err != nil {
			return false, err
		}
		matched = matched || ok
	}
	if len(query.Bounds) != 0 {
		xy, err := codec.FeatureGeometryXYProjection(transformed)
		if err != nil {
			return false, err
		}
		for _, bounds := range query.Bounds {
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
