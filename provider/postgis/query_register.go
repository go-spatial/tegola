package postgis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/internal/featuresql"
	"github.com/jackc/pgx/v5"
)

type featureColumn struct {
	name                            string
	number                          int16
	oid                             uint32
	typmod                          int32
	generated, identity             string
	collation                       uint32
	notNull                         bool
	typeName, typeSchema, extension string
}

type featureProjection struct {
	output string
	column featureColumn
}

type featureProfile struct {
	queryables      provider.FeatureQueryables
	queryablesErr   error
	filterColumns   map[string]featureColumn
	projections     []featureProjection
	publicFields    []string
	identifierLimit int
	where           string
	whereArgs       []any
	crsExplicit     bool
	schema, table   string
	oid             uint32
	columns         []featureColumn
	id, geometry    string
	srid            uint64
	format          string
	mos             codec.MOSConfig
	private         map[string]bool
	spatial         provider.SpatialMetadata
	temporal        provider.TemporalMapping
	temporalScale   int64
	height          *crsconfig.HeightProjection
	crs             provider.FeatureCRSDefinition
	projection      *crsconfig.FeatureProjection
	queryProjection *crsconfig.FeatureProjection
	nativeCRSTuple  featureSRSTuple
	postgisSchema   string
	err             error
}

func (l Layer) FeatureQuerySupported() error {
	if l.feature == nil {
		return fmt.Errorf("postgis feature metadata is unavailable: %w", provider.ErrUnsupported)
	}
	return l.feature.err
}
func (l Layer) SpatialMetadata() (provider.SpatialMetadata, error) {
	if err := l.FeatureQuerySupported(); err != nil {
		return provider.SpatialMetadata{}, err
	}
	return l.feature.spatial, nil
}
func (l Layer) TemporalMapping() (provider.TemporalMapping, error) {
	if err := l.FeatureQuerySupported(); err != nil {
		return provider.TemporalMapping{}, err
	}
	return l.feature.temporal, nil
}

func featureInvalid(field, reason string) error {
	return provider.InvalidFeatureQueryError{Field: field, Reason: reason}
}
func featureUnsupported(reason string) error {
	return fmt.Errorf("postgis feature profile: %s: %w", reason, provider.ErrUnsupported)
}

type featureCatalogReader interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func inspectFeatureRelation(ctx context.Context, db featureCatalogReader, schema, table string) (*featureProfile, error) {
	result := &featureProfile{schema: schema, table: table}
	var kind, persistence string
	var policy, partition bool
	err := db.QueryRow(ctx, `SELECT c.oid,c.relkind::text,c.relpersistence::text,
 c.relrowsecurity OR EXISTS(SELECT 1 FROM pg_catalog.pg_policy p WHERE p.polrelid=c.oid),c.relispartition
 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname=$1 AND c.relname=$2`, schema, table).Scan(&result.oid, &kind, &persistence, &policy, &partition)
	if err != nil {
		return nil, err
	}
	if kind != "r" || persistence != "p" || policy || partition {
		return nil, featureUnsupported("requires a permanent ordinary table without row policies or partition inheritance")
	}
	var inherited, rules bool
	if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_inherits WHERE inhrelid=$1 OR inhparent=$1),
 EXISTS(SELECT 1 FROM pg_catalog.pg_rewrite WHERE ev_class=$1)`, result.oid).Scan(&inherited, &rules); err != nil {
		return nil, err
	}
	if inherited || rules {
		return nil, featureUnsupported("inheritance and rewrite rules are outside the source profile")
	}
	rows, err := db.Query(ctx, `SELECT a.attname,a.attnum,a.atttypid,a.atttypmod,a.attgenerated::text,a.attidentity::text,a.attcollation,a.attnotnull,
 t.typname,n.nspname,COALESCE(e.extname,'')
 FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_type t ON t.oid=a.atttypid JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace
 LEFT JOIN pg_catalog.pg_depend d ON d.classid='pg_catalog.pg_type'::pg_catalog.regclass AND d.objid=t.oid AND d.deptype='e'
 LEFT JOIN pg_catalog.pg_extension e ON e.oid=d.refobjid
 WHERE a.attrelid=$1 AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum`, result.oid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c featureColumn
		if err := rows.Scan(&c.name, &c.number, &c.oid, &c.typmod, &c.generated, &c.identity, &c.collation, &c.notNull, &c.typeName, &c.typeSchema, &c.extension); err != nil {
			return nil, err
		}
		result.columns = append(result.columns, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result.columns) == 0 {
		return nil, featureUnsupported("table has no visible source columns")
	}
	return result, nil
}

func featureInteger(c featureColumn) bool {
	return c.typeSchema == "pg_catalog" && (c.oid == 20 || c.oid == 21 || c.oid == 23) && c.generated == ""
}
func (p *featureProfile) column(name string) (featureColumn, bool) {
	if len(p.projections) > 0 {
		for _, projection := range p.projections {
			if projection.output == name {
				return projection.column, true
			}
		}
		return featureColumn{}, false
	}
	for _, c := range p.columns {
		if c.name == name {
			return c, true
		}
	}
	return featureColumn{}, false
}

func proveFeatureID(ctx context.Context, db featureCatalogReader, p *featureProfile) error {
	c, ok := p.column(p.id)
	if !ok || !featureInteger(c) {
		return featureUnsupported("identity requires an ordinary integer column")
	}
	var unique bool
	err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_index i WHERE i.indrelid=$1 AND i.indisunique AND i.indisvalid AND i.indisready AND i.indislive
 AND i.indimmediate AND i.indnkeyatts=1 AND i.indkey[0]=$2 AND i.indpred IS NULL AND i.indexprs IS NULL)`, p.oid, c.number).Scan(&unique)
	if err != nil {
		return err
	}
	if !unique {
		return featureUnsupported("identity requires a proven single-column unique key")
	}
	return nil
}

func featureConfiguredMetadata(conf dict.Dicter) (provider.SpatialMetadata, provider.TemporalMapping, int64, error) {
	spatial := provider.SpatialMetadata{Dimension: provider.DimensionXY}
	if raw, present := conf.Interface("spatial_dimension"); present {
		switch raw {
		case "xy":
			spatial.Dimension = provider.DimensionXY
		case "xyz":
			spatial.Dimension = provider.DimensionXYZ
		case "mixed_xy_xyz":
			spatial.Dimension = provider.DimensionMixedXYXYZ
		default:
			return spatial, provider.TemporalMapping{}, 0, featureInvalid("spatial_dimension", "must be xy, xyz or mixed_xy_xyz")
		}
	}
	if raw, present := conf.Interface("vertical_crs"); present {
		value, ok := raw.(string)
		if !ok || strings.TrimSpace(value) == "" {
			return spatial, provider.TemporalMapping{}, 0, featureInvalid("vertical_crs", "must be a nonblank string")
		}
		spatial.VerticalCRS = value
	}
	if _, explicit := conf.Interface("spatial_dimension"); explicit && spatial.Dimension != provider.DimensionXY && spatial.VerticalCRS == "" {
		return spatial, provider.TemporalMapping{}, 0, featureInvalid("vertical_crs", "explicit XYZ/mixed source requires a height reference")
	}
	mapping := provider.TemporalMapping{}
	fields := []*string{&mapping.InstantField, &mapping.StartField, &mapping.EndField}
	for i, key := range []string{"temporal_field", "temporal_start_field", "temporal_end_field"} {
		if raw, present := conf.Interface(key); present {
			value, ok := raw.(string)
			if !ok || strings.TrimSpace(value) == "" {
				return spatial, mapping, 0, featureInvalid("temporal", "fields must be nonblank strings")
			}
			*fields[i] = value
		}
	}
	if err := mapping.Validate(); err != nil {
		return spatial, mapping, 0, err
	}
	raw, hasStorage := conf.Interface("temporal_storage")
	if (mapping != (provider.TemporalMapping{})) != hasStorage {
		return spatial, mapping, 0, featureInvalid("temporal_storage", "required exactly when temporal fields are mapped")
	}
	scale := int64(0)
	if hasStorage {
		value, ok := raw.(string)
		if !ok || strings.TrimSpace(value) == "" {
			return spatial, mapping, 0, featureInvalid("temporal_storage", "must be a nonblank string")
		}
		switch value {
		case "unix_seconds":
			scale = 1
		case "unix_milliseconds":
			scale = 1000
		case "unix_microseconds":
			scale = 1000000
		case "unix_nanoseconds":
			scale = 1000000000
		default:
			return spatial, mapping, 0, featureUnsupported("temporal storage must be a declared integer POSIX unit")
		}
	}
	return spatial, mapping, scale, nil
}

func (p *Provider) registerFeatureProfile(conf dict.Dicter, l *Layer, table string, ordinary bool, mvt bool) error {
	spatial, temporal, scale, err := featureConfiguredMetadata(conf)
	profile := &featureProfile{srid: l.srid, crsExplicit: l.crsExplicit, format: l.geometryFormat, mos: l.mosConfig, id: l.idField, geometry: l.geomField, spatial: spatial, temporal: temporal, temporalScale: scale}
	l.feature = profile
	if err != nil {
		if errors.Is(err, provider.ErrUnsupported) {
			profile.err = err
			return nil
		}
		return err
	}
	if mvt {
		profile.err = featureUnsupported("MVT providers cannot expose raw features")
		return nil
	}
	var selection *featuresql.Plan
	if raw, present := conf.Interface("feature_sql"); present {
		value, ok := raw.(string)
		if !ok || strings.TrimSpace(value) == "" {
			return featureInvalid("feature_sql", "must be a nonblank string")
		}
		selection, err = featuresql.Parse(value, featuresql.PostgreSQL)
		if err != nil {
			if errors.Is(err, featuresql.ErrUnsupported) {
				profile.err = featureUnsupported("custom selection uses an unsupported SQL form")
				return nil
			}
			return featureInvalid("feature_sql", "invalid constrained selection")
		}
	}
	if !ordinary && selection == nil {
		profile.err = featureUnsupported("tile SQL requires a separate feature_sql selection")
		return nil
	}
	var schema, name string
	if selection == nil {
		schema, name, err = featureTableIdentity(table)
	} else {
		schema, name, err = featurePlanRelation(selection.Relation())
	}
	if err != nil {
		profile.err = err
		return nil
	}
	ctx, cancel := codec.NewInspectionContext()
	defer cancel()
	var encoding string
	if err := p.pool.QueryRow(ctx, "SHOW server_encoding").Scan(&encoding); err != nil {
		profile.err = fmt.Errorf("postgis feature server encoding: %w", err)
		return nil
	}
	if encoding != "UTF8" {
		profile.err = featureUnsupported("identifier folding requires UTF8 server encoding")
		return nil
	}
	var identifierLimit string
	if err := p.pool.QueryRow(ctx, "SHOW max_identifier_length").Scan(&identifierLimit); err != nil {
		profile.err = fmt.Errorf("postgis feature identifier limit: %w", err)
		return nil
	}
	profile.identifierLimit, err = strconv.Atoi(identifierLimit)
	if err != nil {
		profile.err = featureUnsupported("invalid server identifier limit")
		return nil
	}
	if profile.identifierLimit <= 0 || len(schema) > profile.identifierLimit || len(name) > profile.identifierLimit {
		return featureInvalid("feature_sql", "relation identifier exceeds server identifier limit")
	}
	qualified := false
	if selection != nil {
		qualified = len(selection.Relation().Parts()) == 2
	} else {
		parts, _ := parseIdentParts(table)
		qualified = len(parts) == 2
	}
	if !qualified {
		if err := p.pool.QueryRow(ctx, `SELECT n.nspname,c.relname FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE c.oid=pg_catalog.to_regclass($1)`, pgQuoteIdent(name)).Scan(&schema, &name); err != nil {
			profile.err = fmt.Errorf("postgis feature relation binding: %w", err)
			return nil
		}
	}
	inspected, err := inspectFeatureRelation(ctx, p.pool, schema, name)
	if err != nil {
		profile.err = fmt.Errorf("postgis feature catalog admission: %w", err)
		return nil
	}
	profile.schema, profile.table, profile.oid, profile.columns = inspected.schema, inspected.table, inspected.oid, inspected.columns
	if selection != nil {
		if err := p.resolveFeatureSelection(ctx, profile, selection); err != nil {
			if errors.Is(err, provider.ErrUnsupported) || errors.Is(err, featuresql.ErrUnsupported) {
				profile.err = featureUnsupported("custom selection schema or literal profile is unsupported")
				return nil
			}
			var invalid provider.InvalidFeatureQueryError
			if errors.Is(err, featuresql.ErrInvalid) || errors.As(err, &invalid) {
				return featureInvalid("feature_sql", "custom selection does not resolve against source metadata")
			}
			profile.err = err
			return nil
		}
	}
	if err := proveFeatureID(ctx, p.pool, profile); err != nil {
		profile.err = err
		return nil
	}
	if err := p.resolveFeatureMetadata(ctx, profile, conf); err != nil {
		if errors.Is(err, provider.ErrUnsupported) {
			profile.err = err
			return nil
		}
		return err
	}
	profile.private = map[string]bool{}
	for _, field := range l.bboxFields {
		profile.private[field] = true
		profile.private[featureFoldIdentifier(field)] = true
	}
	profile.private["min_zoom"] = true
	profile.private["max_zoom"] = true
	if selection == nil {
		profile.publicFields, err = conf.StringSlice(ConfigKeyFields)
		if err != nil {
			return featureInvalid("fields", "must be a string array")
		}
		profile.publicFields = append([]string(nil), profile.publicFields...)
	}
	if _, err := profile.featureFields(nil); err != nil {
		profile.err = err
		return nil
	}
	profile.freezeOptionalQueryables()
	p.freezeFeatureCRS(ctx, profile)
	return nil
}

// Fold only unquoted names. The public placeholder for an unqualified name is
// replaced by the server's resolved schema before catalog admission.
func featureTableIdentity(raw string) (string, string, error) {
	parts, err := parseIdentParts(raw)
	if err != nil || len(parts) < 1 || len(parts) > 2 {
		return "", "", featureUnsupported("requires one physical table identifier")
	}
	quoted := false
	start := 0
	index := 0
	for i := 0; i <= len(raw); i++ {
		if i < len(raw) && raw[i] == '"' {
			if quoted && i+1 < len(raw) && raw[i+1] == '"' {
				i++
				continue
			}
			quoted = !quoted
		}
		if i == len(raw) || (raw[i] == '.' && !quoted) {
			part := raw[start:i]
			if !strings.HasPrefix(part, `"`) {
				for _, ch := range part {
					if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '$') {
						return "", "", featureUnsupported("invalid physical table identifier")
					}
				}
				parts[index] = featureFoldIdentifier(parts[index])
			}
			index++
			start = i + 1
		}
	}
	if len(parts) == 1 {
		return "public", parts[0], nil
	}
	return parts[0], parts[1], nil
}
func (l Layer) FeatureSourceSRID() uint64 {
	if l.feature != nil {
		return l.feature.srid
	}
	return l.srid
}
