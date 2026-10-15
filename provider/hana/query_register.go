package hana

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	"github.com/alexeydott/tegola/provider/internal/featuresql"
)

type featureProjection struct{ Output, Physical string }

type featureSource struct {
	Catalog           featureCatalog
	Projections       []featureProjection
	ID, Geometry      string
	Temporal          provider.TemporalMapping
	TemporalScale     int64
	Spatial           provider.SpatialMetadata
	DimensionExplicit bool
	SRID              uint64
	Height            *crsconfig.HeightProjection
	CRS               provider.FeatureCRSDefinition
	Projection        *crsconfig.FeatureProjection
	Private           map[string]bool
	Public            map[string]bool
	BasePredicate     string
	BaseArgs          []any
	Queryables        provider.FeatureQueryables
	FilterColumns     map[string]featureColumn
	FilterVersion     string
	FilterError       error
}

func featureInvalid(field, reason string) error {
	return provider.InvalidFeatureQueryError{Field: field, Reason: reason}
}

func featureUnsupported(reason string) error {
	return fmt.Errorf("HANA feature profile: %s: %w", reason, provider.ErrUnsupported)
}

func configureFeatureSource(l *Layer, conf dict.Dicter) (*featureSource, error) {
	s := &featureSource{SRID: l.featureSRID, Spatial: provider.SpatialMetadata{Dimension: provider.DimensionXY}, Private: map[string]bool{}}
	if s.SRID == 0 {
		s.SRID = l.srid
	}
	if value, ok := conf.Interface("spatial_dimension"); ok {
		s.DimensionExplicit = true
		switch value {
		case "xy":
			s.Spatial.Dimension = provider.DimensionXY
		case "xyz":
			s.Spatial.Dimension = provider.DimensionXYZ
		case "mixed_xy_xyz":
			s.Spatial.Dimension = provider.DimensionMixedXYXYZ
		default:
			return nil, featureInvalid("spatial_dimension", "must be xy, xyz or mixed_xy_xyz")
		}
	}
	if value, ok := conf.Interface("vertical_crs"); ok {
		vertical, valid := value.(string)
		if !valid || strings.TrimSpace(vertical) == "" {
			return nil, featureInvalid("vertical_crs", "must be a nonblank string")
		}
		s.Spatial.VerticalCRS = vertical
	}
	if s.Spatial.Dimension == provider.DimensionXY && s.Spatial.VerticalCRS != "" {
		return nil, featureInvalid("vertical_crs", "XY source cannot declare height")
	}
	if s.Spatial.Dimension != provider.DimensionXY && s.Spatial.VerticalCRS == "" {
		return nil, featureInvalid("vertical_crs", "XYZ/mixed source requires a height reference")
	}
	keys := []string{"temporal_field", "temporal_start_field", "temporal_end_field", "temporal_storage"}
	values := make([]string, 4)
	present := make([]bool, 4)
	for i, key := range keys {
		if v, ok := conf.Interface(key); ok {
			value, valid := v.(string)
			if !valid || strings.TrimSpace(value) == "" {
				return nil, featureInvalid("temporal", "keys require nonblank strings")
			}
			values[i], present[i] = value, true
		}
	}
	s.Temporal = provider.TemporalMapping{InstantField: values[0], StartField: values[1], EndField: values[2]}
	if err := s.Temporal.Validate(); err != nil {
		return nil, err
	}
	if (s.Temporal != (provider.TemporalMapping{})) != present[3] {
		return nil, featureInvalid("temporal", "storage is required exactly with mapped fields")
	}
	if present[3] {
		switch values[3] {
		case "unix_seconds":
			s.TemporalScale = 1
		case "unix_milliseconds":
			s.TemporalScale = 1000
		case "unix_microseconds":
			s.TemporalScale = 1000000
		case "unix_nanoseconds":
			s.TemporalScale = 1000000000
		default:
			l.featureError = featureUnsupported("temporal storage unsupported")
		}
	}
	return s, nil
}

func (p *Provider) registerFeatureSource(l *Layer, conf dict.Dicter, providerType string) error {
	s, err := configureFeatureSource(l, conf)
	if err != nil {
		return err
	}
	l.feature = s
	var plan *featuresql.Plan
	if value, present := conf.Interface("feature_sql"); present {
		text, valid := value.(string)
		if !valid || strings.TrimSpace(text) == "" {
			return featureInvalid("feature_sql", "requires a nonblank string")
		}
		plan, err = featuresql.Parse(text, featuresql.HANA)
		if err != nil {
			mapped := featureSQLError(err)
			var invalid provider.InvalidFeatureQueryError
			if errors.As(mapped, &invalid) {
				return mapped
			}
			l.featureError = mapped
			return nil
		}
	}
	if providerType == MVTProviderType {
		l.featureError = featureUnsupported("MVT provider is not a raw feature source")
		return nil
	}
	if s.Spatial.Dimension != provider.DimensionXY {
		if s.Spatial.VerticalCRS != provider.CRS84h {
			l.featureError = featureUnsupported("source height reference unsupported")
			return nil
		}
		if l.geometryFormat == "mos" {
			l.featureError = featureUnsupported("MOS stores XY only")
			return nil
		}
	}
	if plan == nil && l.featureTable == "" {
		l.featureError = featureUnsupported("custom tile SQL requires feature_sql")
		return nil
	}
	var schema, table string
	if plan != nil {
		parts := plan.Relation().Parts()
		table = hanaSQLKey(parts[len(parts)-1])
		if len(parts) == 2 {
			schema = hanaSQLKey(parts[0])
		}
	} else {
		schema, table, err = splitQualifiedTableName(l.featureTable)
		if err != nil {
			return featureInvalid("tablename", "invalid physical relation identifier")
		}
	}
	ctx, cancel := NewInspectionContext(context.Background())
	defer cancel()
	catalog, err := readFeatureCatalog(ctx, p.pool.pool, schema, table)
	if err != nil {
		l.featureError = errors.Join(featureUnsupported("catalog proof unavailable"), err)
		return nil
	}
	s.Catalog = catalog
	if plan != nil {
		if err := resolveFeatureSQL(l, s, plan); err != nil {
			var invalid provider.InvalidFeatureQueryError
			if errors.As(err, &invalid) {
				return err
			}
			l.featureError = err
			return nil
		}
	} else {
		for _, col := range catalog.Columns {
			if !col.Hidden && !col.Masked {
				s.Projections = append(s.Projections, featureProjection{Output: col.Name, Physical: col.Name})
			}
		}
	}
	if plan == nil {
		fields, fieldErr := conf.StringSlice(ConfigKeyFields)
		if fieldErr != nil {
			return featureInvalid("fields", "requires a string array")
		}
		if len(fields) != 0 {
			s.Public = map[string]bool{}
			for _, field := range fields {
				name, err := featureConfigColumn(field)
				if err != nil {
					return err
				}
				physical := s.physical(name)
				if physical == "" {
					return featureInvalid("fields", "unknown projected property")
				}
				s.Public[name] = true
			}
		}
	}
	if err := resolveFeatureSource(l, s); err != nil {
		var invalid provider.InvalidFeatureQueryError
		if errors.As(err, &invalid) {
			return err
		}
		l.featureError = err
		return nil
	}
	if s.Spatial.Dimension != provider.DimensionXY {
		projection, err := crsconfig.NewHeightProjection(s.SRID)
		if err != nil {
			l.featureError = featureUnsupported("source height projection unsupported")
			return nil
		}
		s.Height = projection
	}
	registerFeatureQueryables(ctx, p.pool.pool, s)
	freezeFeatureCRS(s)
	return nil
}

func featureConfigColumn(name string) (string, error) {
	if name == "" {
		return "", nil
	}
	parts, err := parseIdentParts(name)
	if err != nil || len(parts) != 1 {
		return "", featureInvalid("field", "requires one physical/output column identifier")
	}
	return parts[0], nil
}

func resolveFeatureSource(l *Layer, s *featureSource) error {
	var err error
	s.ID, err = featureConfigColumn(l.idField)
	if err != nil {
		return err
	}
	s.Geometry, err = featureConfigColumn(l.geomField)
	if err != nil {
		return err
	}
	resolve := func(output string) (string, bool) {
		for _, projection := range s.Projections {
			if projection.Output == output {
				return projection.Physical, true
			}
		}
		return "", false
	}
	id, ok := resolve(s.ID)
	if !ok || !s.Catalog.uniqueInteger(id) {
		return featureUnsupported("requires a projected single-column unique integral identity")
	}
	geometry, ok := resolve(s.Geometry)
	if !ok {
		return featureUnsupported("geometry lineage is not projected")
	}
	geomColumn, ok := s.Catalog.column(geometry)
	if !ok || geomColumn.Hidden || geomColumn.Masked || featureComputed(geomColumn) {
		return featureUnsupported("geometry is not an ordinary physical column")
	}
	switch l.geometryFormat {
	case "":
		proven := false
		for _, native := range s.Catalog.Native {
			canonical, valid := featureCanonicalNativeSRID(native)
			if native.Name == geometry && valid && (native.SRID == s.SRID || canonical == s.SRID) && ((native.Dimensions == 2 && s.Spatial.Dimension == provider.DimensionXY) || (native.Dimensions == 4 && s.DimensionExplicit)) {
				proven = true
				s.SRID = canonical
			}
		}
		if !proven {
			return featureUnsupported("native source SRID and dimensional envelope must be proven; permissive envelope requires explicit dimension")
		}
	case "wkb", "mos":
		if geomColumn.Type != "BLOB" && geomColumn.Type != "VARBINARY" && geomColumn.Type != "BINARY" {
			return featureUnsupported("binary geometry storage is not proven")
		}
	case "wkt":
		if geomColumn.Type != "NVARCHAR" && geomColumn.Type != "VARCHAR" && geomColumn.Type != "NCLOB" && geomColumn.Type != "CLOB" {
			return featureUnsupported("text geometry storage is not proven")
		}
	default:
		return featureUnsupported("geometry storage unsupported")
	}
	fields := []*string{&s.Temporal.InstantField, &s.Temporal.StartField, &s.Temporal.EndField}
	seen := map[string]bool{}
	for _, field := range fields {
		if *field == "" {
			continue
		}
		*field, err = featureConfigColumn(*field)
		if err != nil {
			return err
		}
		physical, ok := resolve(*field)
		if !ok {
			return featureInvalid("temporal", "mapped field must be projected")
		}
		col, ok := s.Catalog.column(physical)
		if !ok || !featureIntegral(col.Type) || col.Hidden || col.Masked || featureComputed(col) {
			return featureUnsupported("temporal field is not an ordinary integral column")
		}
		if seen[physical] {
			return featureInvalid("temporal", "endpoints resolve to the same physical column")
		}
		seen[physical] = true
	}
	for _, field := range l.bboxFields {
		physical, err := featureConfigColumn(field)
		if err != nil {
			return err
		}
		if _, ok := s.Catalog.column(physical); ok {
			s.Private[physical] = true
		}
	}
	s.Private[id], s.Private[geometry] = true, true
	for _, column := range s.Catalog.Columns {
		name := featureASCIIUpper(column.Name)
		if name == "MIN_ZOOM" || name == "MAX_ZOOM" {
			s.Private[column.Name] = true
		}
	}
	for _, projection := range s.Projections {
		if s.Private[projection.Physical] || (s.Public != nil && !s.Public[projection.Output]) {
			continue
		}
		column, _ := s.Catalog.column(projection.Physical)
		if featureComputed(column) {
			return featureUnsupported("computed property determinism is unproven")
		}
		switch column.Type {
		case "TINYINT", "SMALLINT", "INTEGER", "BIGINT", "BOOLEAN", "REAL", "DOUBLE", "VARCHAR", "NVARCHAR", "CHAR", "NCHAR", "CLOB", "NCLOB", "TEXT", "BINARY", "VARBINARY", "BLOB", "DATE", "TIME", "TIMESTAMP", "SECONDDATE":
		case "DECIMAL":
			if column.Length <= 0 || column.Length > 38 || column.Scale < 0 || column.Scale > column.Length {
				return featureUnsupported("decimal property metadata unsupported")
			}
		default:
			return featureUnsupported("projected property storage is not JSON-proven")
		}
	}
	return nil
}

func featureASCIIUpper(value string) string {
	result := []byte(value)
	for i, b := range result {
		if b >= 'a' && b <= 'z' {
			result[i] = b - 'a' + 'A'
		}
	}
	return string(result)
}
