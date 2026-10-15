package postgis

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const featureCandidateChunk = 256

type featureSnapshot interface {
	featureCatalogReader
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// QueryFeatures owns one read-only repeatable-read snapshot. SQL LIMIT bounds
// candidate chunks; exact spatial/time predicates precede logical paging.
func (p *Provider) QueryFeatures(ctx context.Context, layerName string, q provider.FeatureQuery, fn func(*provider.Feature) error) (result provider.FeatureQueryResult, err error) {
	if err := q.Validate(); err != nil {
		return result, err
	}
	if fn == nil {
		return result, featureInvalid("callback", "must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	layer, exists := p.layers[layerName]
	if !exists {
		return result, provider.FeatureLayerNotFoundError{Layer: layerName}
	}
	if err := layer.FeatureQuerySupported(); err != nil {
		return result, err
	}
	f := layer.feature
	if len(q.IDs) > 512 || len(q.Bounds)+len(q.Bounds3D) > 128 {
		return result, featureUnsupported("query exceeds bounded predicate profile")
	}
	fields, err := f.featureFields(q.Fields)
	if err != nil {
		return result, err
	}
	if len(q.Bounds3D) > 0 && q.BoundsVerticalCRS != provider.CRS84h {
		return result, featureUnsupported("unsupported query height reference")
	}
	f, err = f.prepareFeatureCRS(q)
	if err != nil {
		return result, err
	}
	if q.BoundsCRSDefinition == "" && len(q.Bounds)+len(q.Bounds3D) > 0 {
		if f.height != nil {
			if _, err := crsconfig.NewHeightProjection(q.BoundsSRID); err != nil {
				return result, featureUnsupported("query height CRS is unsupported")
			}
		} else {
			if _, err := basic.FromWebMercator(q.BoundsSRID, geom.MultiPoint{}); err != nil {
				return result, featureUnsupported("query CRS is unsupported")
			}
		}
	}
	// Compile and validate before reserving a connection. Only the local profile
	// gains the filter; the caller query and immutable registered metadata do not.
	f, err = f.prepareFeatureFilter(q.Filter)
	if err != nil {
		return result, err
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return result, fmt.Errorf("postgis feature snapshot: %w", err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		closeErr := tx.Rollback(cleanup)
		if err == nil && closeErr != nil && !errors.Is(closeErr, pgx.ErrTxClosed) {
			err = fmt.Errorf("postgis feature snapshot cleanup: %w", closeErr)
		}
	}()
	return executeFeatureSnapshot(ctx, tx, f, q, fields, fn)
}

func (f *featureProfile) qualifiedTable() string {
	return pgQuoteIdent(f.schema) + "." + pgQuoteIdent(f.table)
}

func (f *featureProfile) privatePhysicalField(name string) bool {
	reserved := featureFoldIdentifier(name)
	return f.private[name] || reserved == "min_zoom" || reserved == "max_zoom"
}

func (f *featureProfile) featureFields(requested []string) ([]string, error) {
	eligible := map[string]bool{}
	var public []string
	projections := f.projections
	if len(projections) == 0 {
		for _, column := range f.columns {
			projections = append(projections, featureProjection{output: column.name, column: column})
		}
	}
	for _, projection := range projections {
		column := projection.column
		if projection.output == f.geometry || projection.output == f.id || f.privatePhysicalField(column.name) {
			continue
		}
		if len(f.publicFields) > 0 {
			selected := false
			for _, name := range f.publicFields {
				selected = selected || name == projection.output
			}
			if !selected {
				continue
			}
		}
		if !featurePropertyType(column.oid) {
			return nil, featureUnsupported("source property type is not admitted for JSON output")
		}
		eligible[projection.output] = true
		public = append(public, projection.output)
	}
	for _, field := range f.publicFields {
		if !eligible[field] && field != f.id && field != f.geometry && !f.privatePhysicalField(field) {
			return nil, featureInvalid("fields", "unknown configured output property")
		}
	}
	if len(requested) == 0 {
		return public, nil
	}
	result := make([]string, 0, len(requested))
	seen := map[string]bool{}
	for _, field := range requested {
		if !eligible[field] {
			return nil, featureInvalid("fields", "unknown or private output property")
		}
		if !seen[field] {
			result = append(result, field)
			seen[field] = true
		}
	}
	return result, nil
}
func featurePropertyType(oid uint32) bool {
	switch oid {
	case 16, 17, 20, 21, 23, 25, 700, 701, 1042, 1043, 114, 3802:
		return true
	}
	return false
}

func (f *featureProfile) queryColumns(fields []string) []string {
	result := []string{f.id, f.geometry}
	seen := map[string]bool{f.id: true, f.geometry: true}
	for _, field := range append([]string{f.temporal.InstantField, f.temporal.StartField, f.temporal.EndField}, fields...) {
		if field != "" && !seen[field] {
			result = append(result, field)
			seen[field] = true
		}
	}
	return result
}

func (f *featureProfile) chunkStatement(q provider.FeatureQuery, columns []string, cursor *int64) (string, []any) {
	selected := make([]string, len(columns))
	for i, column := range columns {
		physical, _ := f.column(column)
		expression := "l." + pgQuoteIdent(physical.name)
		if column == f.geometry && f.format == "" {
			expression = pgQuoteIdent(f.postgisSchema) + ".ST_AsBinary(" + expression + ")"
		}
		selected[i] = expression + " AS " + pgQuoteIdent(column)
	}
	idColumn, _ := f.column(f.id)
	id := "l." + pgQuoteIdent(idColumn.name)
	predicate := id + " IS NOT NULL"
	args := append([]any(nil), f.whereArgs...)
	if f.where != "" {
		predicate += " AND (" + f.where + ")"
	}
	if len(q.IDs) > 0 {
		var parameters []string
		for _, value := range q.IDs {
			if value > math.MaxInt64 {
				continue
			}
			args = append(args, int64(value))
			parameters = append(parameters, fmt.Sprintf("$%d", len(args)))
		}
		if len(parameters) == 0 {
			predicate += " AND FALSE"
		} else {
			predicate += " AND " + id + " IN (" + strings.Join(parameters, ",") + ")"
		}
	}
	sameFrame := q.BoundsSRID == f.srid
	if q.BoundsCRSDefinition != "" {
		sameFrame = f.projection.Equivalent(f.queryProjection)
	}
	if f.format == "" && sameFrame && len(q.Bounds)+len(q.Bounds3D) > 0 {
		bounds := q.Bounds
		if len(q.Bounds3D) > 0 {
			bounds = make([]geom.Extent, len(q.Bounds3D))
			for i, box := range q.Bounds3D {
				bounds[i] = geom.Extent{box[0], box[1], box[3], box[4]}
			}
		}
		geometryColumn, _ := f.column(f.geometry)
		geometry := "l." + pgQuoteIdent(geometryColumn.name)
		prefix := pgQuoteIdent(f.postgisSchema) + "."
		alternatives := []string{geometry + " IS NULL", prefix + "ST_IsEmpty(" + geometry + ")"}
		for _, box := range bounds {
			start := len(args) + 1
			args = append(args, box[0], box[1], box[2], box[3], int64(f.srid))
			alternatives = append(alternatives, fmt.Sprintf("%s && %sST_MakeEnvelope($%d,$%d,$%d,$%d,$%d)", geometry, prefix, start, start+1, start+2, start+3, start+4))
		}
		predicate += " AND (" + strings.Join(alternatives, " OR ") + ")"
	}
	if cursor != nil {
		args = append(args, *cursor)
		predicate += fmt.Sprintf(" AND %s>$%d", id, len(args))
	}
	return "SELECT " + strings.Join(selected, ",") + " FROM ONLY " + f.qualifiedTable() + " l WHERE " + predicate + " ORDER BY " + id + " LIMIT 256", args
}

func executeFeatureSnapshot(ctx context.Context, tx featureSnapshot, f *featureProfile, q provider.FeatureQuery, fields []string, fn func(*provider.Feature) error) (result provider.FeatureQueryResult, err error) {
	// AccessShare protection precedes catalog inspection and any callback. ONLY
	// binds this exact physical source without implicit inherited relations.
	if _, err := tx.Exec(ctx, "LOCK TABLE ONLY "+f.qualifiedTable()+" IN ACCESS SHARE MODE"); err != nil {
		return result, featureDataError(ctx, err)
	}
	actual, err := inspectFeatureRelation(ctx, tx, f.schema, f.table)
	if err != nil {
		return result, featureDataError(ctx, err)
	}
	if actual.oid != f.oid || !reflect.DeepEqual(actual.columns, f.columns) {
		return result, featureDataError(ctx, fmt.Errorf("source catalog identity changed since registration"))
	}
	if f.projection != nil && f.format == "" {
		tuple, tupleErr := readFeatureSRSTuple(ctx, tx, f)
		if tupleErr != nil {
			return result, featureDataError(ctx, tupleErr)
		}
		if tuple != f.nativeCRSTuple {
			return result, featureDataError(ctx, fmt.Errorf("source CRS definition changed since registration"))
		}
	}
	if err := proveFeatureID(ctx, tx, f); err != nil {
		return result, featureDataError(ctx, err)
	}
	columns := f.queryColumns(fields)
	var cursor *int64
	var matches uint64
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		statement, args := f.chunkStatement(q, columns, cursor)
		rows, err := tx.Query(ctx, statement, args...)
		if err != nil {
			return result, featureDataError(ctx, err)
		}
		read, stop, scanErr := consumeFeatureChunk(ctx, rows, f, q, columns, fields, &cursor, &matches, &result, fn)
		rows.Close()
		if scanErr != nil {
			return result, scanErr
		}
		if stop || read < featureCandidateChunk {
			return result, nil
		}
	}
}

func consumeFeatureChunk(ctx context.Context, rows pgx.Rows, f *featureProfile, q provider.FeatureQuery, columns, fields []string, cursor **int64, matches *uint64, result *provider.FeatureQueryResult, fn func(*provider.Feature) error) (read int, stop bool, err error) {
	descriptions := rows.FieldDescriptions()
	if len(descriptions) != len(columns) {
		return 0, false, featureDataError(ctx, fmt.Errorf("source result column count changed"))
	}
	for i, description := range descriptions {
		expected, _ := f.column(columns[i])
		oid := expected.oid
		if columns[i] == f.geometry && f.format == "" {
			oid = 17
		}
		if description.Name != columns[i] || description.DataTypeOID != oid {
			return 0, false, featureDataError(ctx, fmt.Errorf("source result column identity changed"))
		}
		if columns[i] != f.geometry || f.format != "" {
			if description.TableOID != f.oid || description.TableAttributeNumber != uint16(expected.number) {
				return 0, false, featureDataError(ctx, fmt.Errorf("source result lineage changed"))
			}
		}
	}
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return read, false, err
		}
		values, err := rows.Values()
		if err != nil {
			return read, false, featureDataError(ctx, err)
		}
		read++
		if len(values) != len(columns) {
			return read, false, featureDataError(ctx, fmt.Errorf("source result row width changed"))
		}
		id, ok := featureInt64(values[0])
		if !ok || id < 0 {
			return read, false, featureDataError(ctx, fmt.Errorf("source identity is not a nonnegative integer"))
		}
		if *cursor != nil && id <= **cursor {
			return read, false, featureDataError(ctx, fmt.Errorf("source identity order is not strictly increasing"))
		}
		next := id
		*cursor = &next
		valueMap := make(map[string]any, len(columns))
		for i, column := range columns {
			valueMap[column] = values[i]
		}
		if f.format == "mos" {
			if data, ok := values[1].([]byte); ok && mos.IsSystemInfoBlob(data) {
				if _, err := mos.ParseSystemInfo(data); err != nil {
					return read, false, featureDataError(ctx, err)
				}
				continue
			}
		}
		geometry, err := f.decodeFeatureGeometry(values[1])
		if err != nil {
			return read, false, featureDataError(ctx, err)
		}
		timeMatch, err := f.matchesFeatureTemporal(valueMap, q.Temporal)
		if err != nil {
			return read, false, featureDataError(ctx, err)
		}
		spaceMatch, err := f.matchesFeatureBounds(geometry, q)
		if err != nil {
			if errors.Is(err, codec.ErrUnsupportedFeatureSpatialGeometry) || errors.Is(err, provider.ErrUnsupported) {
				return read, false, fmt.Errorf("postgis requested spatial profile: %w: %w", provider.ErrUnsupported, err)
			}
			return read, false, featureDataError(ctx, err)
		}
		if !timeMatch || !spaceMatch {
			continue
		}
		if *matches < q.Offset {
			*matches++
			continue
		}
		if result.NumberReturned >= uint64(q.Limit) {
			result.HasMore = true
			return read, true, nil
		}
		tags := make(map[string]interface{}, len(fields))
		for _, field := range fields {
			value, err := detachFeatureProperty(valueMap[field])
			if err != nil {
				return read, false, featureDataError(ctx, err)
			}
			tags[field] = value
		}
		if err := ctx.Err(); err != nil {
			return read, false, err
		}
		if err := fn(&provider.Feature{ID: uint64(id), SRID: f.srid, Geometry: geometry, Tags: tags}); err != nil {
			return read, false, err
		}
		result.NumberReturned++
		if err := ctx.Err(); err != nil {
			return read, false, err
		}
	}
	if err := rows.Err(); err != nil {
		return read, false, featureDataError(ctx, err)
	}
	return read, false, nil
}

func featureDataError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return contextErr
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return provider.FeatureDataError{Err: err}
}
func detachFeatureProperty(value any) (any, error) {
	switch v := value.(type) {
	case nil, bool, string, int16, int32, int64:
		return v, nil
	case float32:
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return nil, fmt.Errorf("nonfinite source property")
		}
		return v, nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("nonfinite source property")
		}
		return v, nil
	case []byte:
		return append([]byte(nil), v...), nil
	case []any:
		copyValue := make([]any, len(v))
		for i, item := range v {
			copyItem, err := detachFeatureProperty(item)
			if err != nil {
				return nil, err
			}
			copyValue[i] = copyItem
		}
		return copyValue, nil
	case map[string]any:
		copyValue := make(map[string]any, len(v))
		for key, item := range v {
			copyItem, err := detachFeatureProperty(item)
			if err != nil {
				return nil, err
			}
			copyValue[key] = copyItem
		}
		return copyValue, nil
	default:
		return nil, fmt.Errorf("unsupported source property type %T", value)
	}
}
