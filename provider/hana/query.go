package hana

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/alexeydott/tegola/provider"
)

func (p *Provider) QueryFeatures(ctx context.Context, name string, query provider.FeatureQuery, fn func(*provider.Feature) error) (result provider.FeatureQueryResult, err error) {
	if err := query.Validate(); err != nil {
		return result, err
	}
	if fn == nil {
		return result, featureInvalid("callback", "must not be nil")
	}
	l, exists := p.layers[name]
	if !exists {
		return result, provider.FeatureLayerNotFoundError{Layer: name}
	}
	if err := l.FeatureQuerySupported(); err != nil {
		return result, err
	}
	return p.queryFeaturesGuarded(ctx, name, query, fn)
}

// queryFeaturesGuarded holds source protection and all reads in one transaction.
func (p *Provider) queryFeaturesGuarded(ctx context.Context, name string, query provider.FeatureQuery, fn func(*provider.Feature) error) (result provider.FeatureQueryResult, err error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	defer func() { err = featureContextError(ctx, err) }()
	if err := query.Validate(); err != nil {
		return result, err
	}
	if fn == nil {
		return result, featureInvalid("callback", "must not be nil")
	}
	l, ok := p.layers[name]
	if !ok {
		return result, provider.FeatureLayerNotFoundError{Layer: name}
	}
	if l.feature == nil {
		return result, featureUnsupported("metadata unavailable")
	}
	if l.featureError != nil {
		return result, l.featureError
	}
	if len(query.IDs) > 512 || len(query.Bounds)+len(query.Bounds3D) > 128 {
		return result, featureUnsupported("request exceeds bounded predicate profile")
	}
	s := l.feature
	filterSQL, filterArgs, err := compileFeatureFilter(s, query.Filter)
	if err != nil {
		return result, err
	}
	fields := map[string]bool{}
	for _, projection := range s.Projections {
		if !s.Private[projection.Physical] && (s.Public == nil || s.Public[projection.Output]) {
			fields[projection.Output] = len(query.Fields) == 0
		}
	}
	for _, field := range query.Fields {
		if _, exists := fields[field]; !exists {
			return result, featureInvalid("fields", "unknown or private output field")
		}
		if fields[field] {
			return result, featureInvalid("fields", "duplicate output field")
		}
		fields[field] = true
	}
	spatial, err := newSpatialQuery(&l, query)
	if err != nil {
		return result, err
	}
	if query.BoundsCRSDefinition == "" && (len(query.Bounds)+len(query.Bounds3D) != 0) && s.SRID != query.BoundsSRID && s.Height == nil {
		if err := validateQueryCRS(s.SRID); err != nil {
			return result, err
		}
		if err := validateQueryCRS(query.BoundsSRID); err != nil {
			return result, err
		}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	predicates := []string{"l." + quoteIdent(s.physical(s.ID)) + " IS NOT NULL"}
	args := append([]any(nil), s.BaseArgs...)
	if s.BasePredicate != "" {
		predicates = append(predicates, "("+s.BasePredicate+")")
	}
	if filterSQL != "" {
		predicates = append(predicates, filterSQL)
		args = append(args, filterArgs...)
	}
	if len(query.IDs) != 0 {
		var placeholders []string
		for _, id := range query.IDs {
			if id <= math.MaxInt64 {
				placeholders = append(placeholders, "?")
				args = append(args, int64(id))
			}
		}
		if len(placeholders) == 0 {
			zero := uint64(0)
			result.NumberMatched = &zero
			return result, nil
		}
		predicates = append(predicates, "l."+quoteIdent(s.physical(s.ID))+" IN ("+strings.Join(placeholders, ",")+")")
	}
	predicates = append(predicates, temporalPredicate(s, query.Temporal, &args))
	columns, selected := featureQueryColumns(s)
	conn, err := p.pool.pool.Conn(ctx)
	if err != nil {
		return result, fmt.Errorf("HANA feature reserve connection: %w", err)
	}
	defer func() {
		// go-hdb does not restore access mode/isolation when rolling back.
		// Never return this physical session to the pool, including setup errors.
		discardErr := conn.Raw(func(any) error { return driver.ErrBadConn })
		if !errors.Is(discardErr, driver.ErrBadConn) && !errors.Is(discardErr, sql.ErrConnDone) && discardErr != nil {
			err = errors.Join(err, fmt.Errorf("HANA feature physical discard: %w", discardErr))
		}
		if closeErr := conn.Close(); closeErr != nil && !errors.Is(closeErr, sql.ErrConnDone) {
			err = errors.Join(err, fmt.Errorf("HANA feature reserved connection close: %w", closeErr))
		}
	}()
	var stopWatch func() error
	if watchErr := conn.Raw(func(inner any) error {
		watcher, ok := inner.(interface {
			featureWatchContext(context.Context) (func() error, error)
		})
		if !ok {
			return featureUnsupported("reserved socket cancellation unavailable")
		}
		var err error
		stopWatch, err = watcher.featureWatchContext(ctx)
		return err
	}); watchErr != nil {
		return result, watchErr
	}
	defer func() {
		if closeErr := stopWatch(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("HANA feature cancellation socket close: %w", closeErr))
		}
	}()
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: false})
	if err != nil {
		return result, fmt.Errorf("HANA feature snapshot: %w", err)
	}
	defer func() {
		if closeErr := tx.Rollback(); closeErr != nil && !errors.Is(closeErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("HANA feature snapshot close: %w", closeErr))
		}
	}()
	if _, lockErr := tx.ExecContext(ctx, "SET TRANSACTION LOCK WAIT TIMEOUT 30000"); lockErr != nil {
		return result, fmt.Errorf("HANA feature lock timeout: %w", lockErr)
	}
	if _, lockErr := tx.ExecContext(ctx, "LOCK TABLE "+s.Catalog.qualified()+" IN EXCLUSIVE MODE"); lockErr != nil {
		return result, fmt.Errorf("HANA feature source lock: %w", lockErr)
	}
	catalog, proofErr := readFeatureCatalog(ctx, tx, s.Catalog.Schema, s.Catalog.Table)
	if proofErr != nil {
		return result, featureContextSourceError(ctx, proofErr)
	}
	if !s.Catalog.equal(catalog) {
		return result, featureContextSourceError(ctx, featureInvalid("source", "catalog changed since registration"))
	}
	if query.Filter != nil {
		var version string
		if err := tx.QueryRowContext(ctx, "SELECT VERSION FROM SYS.M_DATABASE").Scan(&version); err != nil {
			return result, featureContextSourceError(ctx, err)
		}
		if version != s.FilterVersion {
			return result, featureContextSourceError(ctx, featureInvalid("source", "filter server profile changed"))
		}
	}
	var cursor int64
	started := false
	var matched uint64
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		where := append([]string(nil), predicates...)
		chunkArgs := append([]any(nil), args...)
		if started {
			where = append(where, "l."+quoteIdent(s.physical(s.ID))+">?")
			chunkArgs = append(chunkArgs, cursor)
		}
		statement := "SELECT " + strings.Join(selected, ",") + " FROM " + s.Catalog.qualified() + " l WHERE " + strings.Join(where, " AND ") + " ORDER BY l." + quoteIdent(s.physical(s.ID)) + " LIMIT 256"
		chunk, readErr := readFeatureChunk(ctx, tx, statement, chunkArgs, columns)
		if readErr != nil {
			return result, featureContextSourceError(ctx, readErr)
		}
		// Revalidation occurs while the same transaction retains the source lock.
		catalog, proofErr := readFeatureCatalog(ctx, tx, s.Catalog.Schema, s.Catalog.Table)
		if proofErr != nil {
			return result, featureContextSourceError(ctx, proofErr)
		}
		if !s.Catalog.equal(catalog) {
			return result, featureContextSourceError(ctx, featureInvalid("source", "catalog changed since registration"))
		}
		for _, row := range chunk {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			id, valid := featureInteger(row[s.physical(s.ID)])
			if !valid || id < 0 || (started && id <= cursor) {
				return result, featureContextSourceError(ctx, featureInvalid("id", "source identity invalid or unordered"))
			}
			cursor, started = id, true
			feature, representable, decodeErr := decodeFeature(l, row, fields)
			if decodeErr != nil {
				return result, featureContextSourceError(ctx, decodeErr)
			}
			if !representable {
				continue
			}
			matches, matchErr := spatial.matches(feature.Geometry, s.SRID, query)
			if matchErr != nil {
				return result, matchErr
			}
			if !matches {
				continue
			}
			matched++
			if matched <= query.Offset {
				continue
			}
			if result.NumberReturned == uint64(query.Limit) {
				result.HasMore = true
				return result, nil
			}
			if err := ctx.Err(); err != nil {
				return result, err
			}
			var liveConnection int64
			if fenceErr := tx.QueryRowContext(ctx, "SELECT CURRENT_CONNECTION FROM DUMMY").Scan(&liveConnection); fenceErr != nil {
				return result, featureContextSourceError(ctx, fenceErr)
			}
			if callbackErr := fn(&feature); callbackErr != nil {
				return result, callbackErr
			}
			if err := ctx.Err(); err != nil {
				return result, err
			}
			result.NumberReturned++
		}
		if len(chunk) < 256 {
			result.NumberMatched = &matched
			return result, nil
		}
	}
}

func featureQueryColumns(s *featureSource) ([]featureColumn, []string) {
	seen := map[string]bool{}
	required := map[string]bool{s.ID: true, s.Geometry: true, s.Temporal.InstantField: true, s.Temporal.StartField: true, s.Temporal.EndField: true}
	var columns []featureColumn
	var selected []string
	for _, projection := range s.Projections {
		if !required[projection.Output] && (s.Private[projection.Physical] || (s.Public != nil && !s.Public[projection.Output])) {
			continue
		}
		if seen[projection.Physical] {
			continue
		}
		seen[projection.Physical] = true
		column, _ := s.Catalog.column(projection.Physical)
		expression := "l." + quoteIdent(column.Name)
		if column.Type == "ST_POINT" || column.Type == "ST_GEOMETRY" {
			expression += ".ST_AsBinary() AS " + quoteIdent(column.Name)
			column.Type = "BLOB"
		}
		columns = append(columns, column)
		selected = append(selected, expression)
	}
	return columns, selected
}

func readFeatureChunk(ctx context.Context, tx *sql.Tx, statement string, args []any, columns []featureColumn) (chunk []map[string]any, err error) {
	rows, err := tx.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	labels, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	if len(labels) != len(columns) || len(types) != len(columns) {
		return nil, featureInvalid("source", "result metadata changed")
	}
	for i, column := range columns {
		precision, scale, known := types[i].DecimalSize()
		if labels[i] != column.Name || !featureResultWireCompatible(column, types[i].DatabaseTypeName(), precision, scale, known) {
			return nil, featureInvalid("source", "result lineage or type changed")
		}
	}
	var bytesRead int
	for rows.Next() {
		if len(chunk) == 256 {
			return nil, featureInvalid("source", "bounded query returned excess rows")
		}
		row, scanErr := featureScanRow(rows, columns)
		if scanErr != nil {
			return nil, scanErr
		}
		for _, v := range row {
			switch value := v.(type) {
			case []byte:
				bytesRead += len(value)
			case string:
				bytesRead += len(value)
			}
		}
		if bytesRead > 64<<20 {
			return nil, featureUnsupported("chunk payload exceeds bounded profile")
		}
		chunk = append(chunk, row)
	}
	return chunk, rows.Err()
}

// go-hdb exposes fixed-decimal protocol names rather than the catalog DECIMAL
// name. The locked physical catalog and result precision/scale must still agree.
func featureResultWireCompatible(column featureColumn, wire string, precision, scale int64, known bool) bool {
	if column.Type != "DECIMAL" {
		return wire == column.Type
	}
	if column.Length < 1 || column.Length > 38 || column.Scale < 0 || column.Scale > column.Length || !known || precision != column.Length || scale != column.Scale {
		return false
	}
	switch wire {
	case "FIXED8":
		return precision <= 18
	case "FIXED12":
		return precision <= 28
	case "FIXED16":
		return true
	case "DECIMAL":
		// The generic protocol decimal has a 34-digit coefficient.
		return precision <= 34
	default:
		return false
	}
}
