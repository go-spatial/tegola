package mysql

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

const featureChunkSize = 256

// QueryFeatures reads one immutable raw source through a repeatable-read
// snapshot. The scan fallback evaluates exact predicates before logical paging.
// Inputs are neither mutated nor retained; callbacks are synchronous.
func (p *Provider) QueryFeatures(
	ctx context.Context,
	layerName string,
	query provider.FeatureQuery,
	fn func(*provider.Feature) error,
) (result provider.FeatureQueryResult, err error) {
	if err := query.Validate(); err != nil {
		return result, err
	}
	if fn == nil {
		return result, featureInvalid("callback", "must not be nil")
	}
	layer, ok := p.layers[layerName]
	if !ok {
		return result, provider.FeatureLayerNotFoundError{Layer: layerName}
	}
	if err := layer.FeatureQuerySupported(); err != nil {
		return result, err
	}
	if len(query.IDs) > 512 || len(query.Bounds)+len(query.Bounds3D) > 128 {
		return result, featureUnsupported("query predicate limit")
	}
	f := layer.feature
	f, err = f.prepareFeatureFilter(query.Filter)
	if err != nil {
		return result, err
	}
	fields, err := f.queryFields(query.Fields)
	if err != nil {
		return result, err
	}
	spatial, err := newFeatureSpatialQuery(f, query)
	if err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	where, args, impossible := f.candidatePredicate(query.IDs)
	if bounds, boundsArgs := f.boundsCandidate(query); bounds != "" {
		where += " AND " + bounds
		args = append(args, boundsArgs...)
	}
	if impossible {
		zero := uint64(0)
		result.NumberMatched = &zero
		return result, nil
	}
	// MySQL 5.5 predates START TRANSACTION READ ONLY. Its explicitly opted-in
	// legacy profile still executes only the same bound SELECT statements and
	// always rolls back. Modern profiles retain the server read-only guard.
	tx, err := p.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: !f.schema.legacyIdentity})
	if err != nil {
		return result, fmt.Errorf("mysql feature snapshot: %w", preferContextError(ctx, err))
	}
	defer func() {
		closeErr := tx.Rollback()
		if !errors.Is(closeErr, sql.ErrTxDone) {
			err = errors.Join(err, closeErr)
		}
		if ctx.Err() != nil {
			err = errors.Join(ctx.Err(), err)
		}
	}()
	// A source read acquires relation protection before comparing retained
	// catalog metadata. No callback may observe a changed schema/profile.
	if err := f.verifySnapshot(ctx, tx); err != nil {
		return result, featureDataError(err)
	}
	columns, selected := f.queryColumns(fields)
	var cursor uint64
	var matched uint64
	started := false
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		predicate := where
		chunkArgs := append([]any{}, args...)
		if started {
			predicate += " AND l." + featureQuoteIdentifier(f.physicalID()) + ">?"
			chunkArgs = append(chunkArgs, f.idArgument(cursor))
		}
		statement := "SELECT " + strings.Join(selected, ",") + " FROM " + f.relation() + " l WHERE " + predicate +
			" ORDER BY l." + featureQuoteIdentifier(f.physicalID()) + " LIMIT 256"
		read := 0
		stopped := false
		err := withFeatureRows(ctx, tx, statement, chunkArgs, func(rows *sql.Rows) error {
			actual, err := rows.Columns()
			if err != nil {
				return err
			}
			if len(actual) != len(columns) {
				return featureDataError(featureInvalid("source", "result shape changed"))
			}
			for i, column := range columns {
				if actual[i] != column {
					return featureDataError(featureInvalid("source", "result label changed"))
				}
			}
			for rows.Next() {
				if err := ctx.Err(); err != nil {
					return err
				}
				read++
				values := make([]any, len(columns))
				targets := make([]any, len(values))
				for i := range values {
					targets[i] = &values[i]
				}
				if err := rows.Scan(targets...); err != nil {
					return err
				}
				id, err := f.decodeID(values[0])
				if err != nil {
					return featureDataError(err)
				}
				if started && id <= cursor {
					return featureDataError(featureInvalid("id", "identity order or uniqueness changed"))
				}
				cursor, started = id, true
				feature, temporalMatch, representable, err := f.decodeFeature(columns, values, query.Temporal)
				if err != nil {
					return featureDataError(err)
				}
				if !representable {
					continue
				}
				spatialMatch, err := spatial.matches(feature.Geometry, f.srid, query)
				if err != nil {
					return err
				}
				if !temporalMatch || !spatialMatch {
					continue
				}
				matched++
				if matched <= query.Offset {
					continue
				}
				if result.NumberReturned == uint64(query.Limit) {
					result.HasMore, stopped = true, true
					return nil
				}
				for name := range feature.Tags {
					if !slices.Contains(fields, name) {
						delete(feature.Tags, name)
					}
				}
				if err := fn(&feature); err != nil {
					return fmt.Errorf("mysql feature callback: %w", err)
				}
				result.NumberReturned++
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return result, err
		}
		if stopped {
			return result, nil
		}
		if read < featureChunkSize {
			total := matched
			result.NumberMatched = &total
			return result, nil
		}
	}
}

func featureDataError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return provider.FeatureDataError{Err: err}
}

func (f *featureProfile) verifySnapshot(ctx context.Context, tx *sql.Tx) error {
	if err := withFeatureRows(ctx, tx, "SELECT l."+featureQuoteIdentifier(f.physicalID())+" FROM "+f.relation()+" l LIMIT 0",
		[]any{}, func(rows *sql.Rows) error { return nil }); err != nil {
		return err
	}
	current, err := inspectFeatureSchema(ctx, tx, f.schema.database, f.schema.table, f.schema.legacyIdentity)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current, f.schema) {
		return featureInvalid("source", "catalog changed after registration")
	}
	return nil
}

func (f *featureProfile) physicalID() string { source, _ := f.sourceColumn(f.id); return source }

func (f *featureProfile) idMaximum() uint64 {
	bits := f.idBits
	if !f.idUnsigned {
		bits--
	}
	if bits == 64 {
		return math.MaxUint64
	}
	return (uint64(1) << bits) - 1
}

func (f *featureProfile) idArgument(id uint64) any {
	if f.idUnsigned {
		return id
	}
	return int64(id)
}

func (f *featureProfile) decodeID(value any) (uint64, error) {
	switch value.(type) {
	case int64, uint64, []byte:
	default:
		return 0, featureInvalid("id", "source identity is not integral")
	}
	id, err := provider.ConvertFeatureID(value)
	if err != nil || id > f.idMaximum() {
		return 0, featureInvalid("id", "source identity outside declared range")
	}
	return id, nil
}

func (f *featureProfile) candidatePredicate(ids []uint64) (string, []any, bool) {
	where := "(" + f.filter + ") AND l." + featureQuoteIdentifier(f.physicalID()) + " IS NOT NULL"
	args := append([]any{}, f.filterArgs...)
	if len(ids) == 0 {
		return where, args, false
	}
	seen := make(map[uint64]bool, len(ids))
	placeholders := []string{}
	for _, id := range ids {
		if id > f.idMaximum() || seen[id] {
			continue
		}
		seen[id] = true
		placeholders = append(placeholders, "?")
		args = append(args, f.idArgument(id))
	}
	if len(placeholders) == 0 {
		return where, args, true
	}
	return where + " AND l." + featureQuoteIdentifier(f.physicalID()) + " IN (" + strings.Join(placeholders, ",") + ")", args, false
}

func (f *featureProfile) queryFields(requested []string) ([]string, error) {
	if len(requested) == 0 {
		return slices.Clone(f.properties), nil
	}
	fields := []string{}
	for _, requestedField := range requested {
		found := false
		for _, available := range f.properties {
			if strings.EqualFold(requestedField, available) {
				if !slices.Contains(fields, available) {
					fields = append(fields, available)
				}
				found = true
				break
			}
		}
		if !found {
			return nil, featureInvalid("fields", "property unavailable")
		}
	}
	return fields, nil
}

func (f *featureProfile) queryColumns(fields []string) ([]string, []string) {
	columns := []string{f.id, f.geometry}
	for _, name := range append(slices.Clone(fields), f.temporal.InstantField, f.temporal.StartField, f.temporal.EndField) {
		if name != "" && !slices.Contains(columns, name) {
			columns = append(columns, name)
		}
	}
	selected := make([]string, len(columns))
	for i, column := range columns {
		source, _ := f.sourceColumn(column)
		expression := "l." + featureQuoteIdentifier(source)
		metadata, _ := f.schema.column(source)
		if strings.EqualFold(metadata.dataType, "bit") && strings.EqualFold(metadata.columnType, "bit(1)") {
			expression = "CAST(" + expression + " AS UNSIGNED)"
		}
		if column == f.geometry && f.native {
			option := ""
			if f.nativeAxisOption {
				option = ",'axis-order=long-lat'"
			}
			expression = "ST_AsWKB(" + expression + option + ")"
		}
		selected[i] = expression + " AS " + featureQuoteIdentifier(column)
	}
	if f.native {
		// The extra positional field is not a public property; reject a label
		// collision at registration before this result shape is used.
		source, _ := f.sourceColumn(f.geometry)
		columns = append(columns, f.nativeSRIDLabel)
		selected = append(selected, "ST_SRID(l."+featureQuoteIdentifier(source)+") AS "+featureQuoteIdentifier(f.nativeSRIDLabel))
	}
	return columns, selected
}

func (f *featureProfile) decodeFeature(columns []string, values []any, temporal *provider.TemporalConstraint) (provider.Feature, bool, bool, error) {
	feature := provider.Feature{SRID: f.srid, Tags: map[string]any{}}
	id, err := f.decodeID(values[0])
	if err != nil {
		return feature, false, false, err
	}
	feature.ID = id
	valueMap := make(map[string]any, len(columns))
	for i, column := range columns {
		valueMap[column] = values[i]
	}
	geometryValue := values[1]
	if f.format == GeometryFormatMOS && codec.IsSystemInfoValue(geometryValue) {
		return feature, false, false, nil
	}
	var geometry geom.Geometry
	if geometryValue != nil {
		switch {
		case f.native || f.format == GeometryFormatWKB:
			geometry, err = codec.DecodeRawWKB(geometryValue)
		case f.format == GeometryFormatWKT:
			geometry, err = codec.DecodeRawWKT(geometryValue)
		case f.format == GeometryFormatMOS:
			geometry, err = codec.DecodeMOS(geometryValue, f.mos)
		default:
			err = featureUnsupported("geometry storage")
		}
		if err != nil {
			return feature, false, false, err
		}
		if f.native {
			srid, err := provider.ConvertFeatureID(valueMap[f.nativeSRIDLabel])
			if err != nil || srid != f.srid {
				return feature, false, false, featureInvalid("srid", "native row differs from frozen CRS")
			}
		}
	}
	if err := validateFeatureDimension(geometry, f.spatial.Dimension); err != nil {
		return feature, false, false, err
	}
	if !featureGeometryEmpty(geometry) {
		feature.Geometry = geometry
	}
	temporalMatch, err := f.temporalMatches(valueMap, temporal)
	if err != nil {
		return feature, false, false, err
	}
	for i, column := range columns {
		if column == f.id || column == f.geometry || !slices.Contains(f.properties, column) {
			continue
		}
		source, _ := f.sourceColumn(column)
		metadata, _ := f.schema.column(source)
		value, err := featureProperty(metadata, values[i])
		if err != nil {
			return feature, false, false, err
		}
		feature.Tags[column] = value
	}
	return feature, temporalMatch, true, nil
}

func featureProperty(column featureColumn, value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	if strings.EqualFold(column.dataType, "bit") && strings.EqualFold(column.columnType, "bit(1)") {
		integer, err := provider.ConvertFeatureID(value)
		if err != nil || integer > 1 {
			return nil, featureInvalid("property", "invalid boolean source")
		}
		return integer == 1, nil
	}
	if bytes, ok := value.([]byte); ok {
		switch strings.ToLower(column.dataType) {
		case "binary", "varbinary", "tinyblob", "blob", "mediumblob", "longblob":
			return base64.StdEncoding.EncodeToString(bytes), nil
		case "decimal", "numeric":
			number := json.Number(string(bytes))
			if !json.Valid(bytes) || len(bytes) == 0 || (bytes[0] != '-' && (bytes[0] < '0' || bytes[0] > '9')) {
				return nil, featureInvalid("property", "invalid decimal source")
			}
			return number, nil
		}
		converted, err := convertTagValue(bytes, categoryFromDatabaseTypeName(column.dataType))
		if err != nil {
			return nil, err
		}
		return featureProperty(column, converted)
	}
	switch v := value.(type) {
	case time.Time:
		return v.Format(time.RFC3339Nano), nil
	case float32:
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return nil, featureInvalid("property", "nonfinite number")
		}
		return strconv.ParseFloat(strconv.FormatFloat(float64(v), 'g', -1, 32), 64)
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, featureInvalid("property", "nonfinite number")
		}
		return v, nil
	case string, bool, int64, uint64:
		return value, nil
	default:
		return nil, featureInvalid("property", "unsupported source property type")
	}
}
