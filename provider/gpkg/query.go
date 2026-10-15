//go:build cgo

package gpkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/proj"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

const featureCandidateChunk = 256

// QueryFeatures streams a bounded page from a snapshot of a supported table.
// Different-CRS bounds use ordered source scanning, not heuristic envelopes.
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
		return result, invalidQuery("callback", "must not be nil")
	}
	layer, exists := p.layers[layerName]
	if !exists {
		return result, provider.FeatureLayerNotFoundError{Layer: layerName}
	}
	if err := layer.FeatureQuerySupported(); err != nil {
		return result, err
	}
	if len(query.IDs) > 512 || len(query.Bounds)+len(query.Bounds3D) > 128 {
		return result, fmt.Errorf("gpkg feature query exceeds bounded predicate profile: %w", provider.ErrUnsupported)
	}
	fields, err := resolveQueryFields(layer, query.Fields)
	if err != nil {
		return result, err
	}
	filterSQL, filterArgs, err := layer.prepareFeatureFilter(query.Filter)
	if err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	spatial, err := newSpatialQuery(layer, query)
	if err != nil {
		return result, err
	}
	if query.BoundsCRSDefinition == "" && (len(query.Bounds) != 0 || len(query.Bounds3D) != 0) && query.BoundsSRID != resolvedLayerSRID(layer) && layer.heightProjection == nil {
		if err := validateQueryCRS(resolvedLayerSRID(layer)); err != nil {
			return result, err
		}
		if err := validateQueryCRS(query.BoundsSRID); err != nil {
			return result, err
		}
	}
	where, args, impossible, err := featureCandidatePredicate(layer, query)
	if err != nil {
		return result, err
	}
	if filterSQL != "" {
		where = "(" + where + ") AND (" + filterSQL + ")"
		args = append(args, filterArgs...)
	}
	if impossible {
		zero := uint64(0)
		result.NumberMatched = &zero
		return result, nil
	}
	columns := queryColumns(layer, fields)
	selected := make([]string, len(columns))
	for i, column := range columns {
		selected[i] = "l." + quoteIdent(column)
		if query.Filter != nil && layer.filterProfile.columns[column].kind == provider.QueryableBoolean {
			// Arithmetic projection removes SQLite's declared BOOLEAN coercion.
			selected[i] = "(" + selected[i] + "+0) AS " + quoteIdent(column)
		}
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("gpkg feature snapshot: %w", err)
	}
	defer func() {
		rollbackErr := tx.Rollback()
		// Read-only transactions always roll back; propagate cleanup failure
		// only when it can invalidate an otherwise successful query.
		if err == nil && rollbackErr != nil {
			err = fmt.Errorf("gpkg feature snapshot close: %w", rollbackErr)
		}
	}()
	if query.Filter != nil {
		if err := layer.filterProfile.verify(ctx, tx, layer.tablename); err != nil {
			return result, featureSourceError(err)
		}
	}
	var cursor int64
	started := false
	var matched uint64
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		predicate := where
		chunkArgs := append([]any(nil), args...)
		if started {
			predicate += " AND l." + quoteIdent(layer.idFieldname) + ">?"
			chunkArgs = append(chunkArgs, cursor)
		}
		statement := "SELECT " + strings.Join(selected, ",") + " FROM " + quoteIdent(layer.tablename) + " l WHERE " + predicate +
			" ORDER BY l." + quoteIdent(layer.idFieldname) + " LIMIT 256"
		read, stopped, scanErr := scanFeatureCandidates(ctx, tx, statement, chunkArgs, len(columns), func(values []any) (bool, error) {
			if query.Filter != nil {
				for i, column := range columns {
					if layer.filterProfile.columns[column].kind != provider.QueryableBoolean || values[i] == nil {
						continue
					}
					value, ok := values[i].(int64)
					if !ok || (value != 0 && value != 1) {
						return false, featureSourceError(invalidQuery("source", "invalid boolean scalar"))
					}
					values[i] = value == 1
				}
			}
			// Identity is the first column. NULL retains the legacy skip
			// policy; other invalid SQLite identities must fail closed.
			if values[0] != nil {
				id, ok := values[0].(int64)
				if !ok || id < 0 {
					return false, featureSourceError(invalidQuery("id", "source identity is not a nonnegative INTEGER"))
				}
				cursor, started = id, true
			}
			decoded, decodeErr := p.decodeRow(ctx, layer, columns, values, rowDecodePolicy{dimensionalRaw: true})
			if decodeErr != nil {
				return false, fmt.Errorf("gpkg feature decode: %w", featureSourceError(wrapSpatialError(decodeErr)))
			}
			if !decoded.Representable {
				return false, nil
			}
			if temporalErr := validateTemporalRow(layer, columns, values); temporalErr != nil {
				return false, featureSourceError(temporalErr)
			}
			feature := decoded.Feature
			if decoded.HadAbsentGeometry {
				feature.Geometry = nil
			}
			exact, exactErr := spatial.matches(feature.Geometry, feature.SRID, query)
			if exactErr != nil {
				return false, exactErr
			}
			if !exact {
				return false, nil
			}
			matched++
			if matched <= query.Offset {
				return false, nil
			}
			if result.NumberReturned == uint64(query.Limit) {
				result.HasMore = true
				return true, nil
			}
			for key := range feature.Tags {
				if !containsFold(fields, key) {
					delete(feature.Tags, key)
				}
			}
			// Restore only actual selected SQL NULLs after strict decoding.
			// The shared tile decoder deliberately omits nullable tags.
			for i, column := range columns {
				if values[i] == nil && containsFold(fields, column) {
					feature.Tags[column] = nil
				}
			}
			if callbackErr := fn(&feature); callbackErr != nil {
				return false, fmt.Errorf("gpkg feature callback: %w", callbackErr)
			}
			result.NumberReturned++
			if err := ctx.Err(); err != nil {
				return false, err
			}
			return false, nil
		})
		if scanErr != nil {
			return result, scanErr
		}
		if stopped {
			return result, nil
		}
		if read < featureCandidateChunk || !started {
			total := matched
			result.NumberMatched = &total
			return result, nil
		}
	}
}

// Source integrity is distinct from invalid request parameters. Decoder profile
// failures describe source data here; admission and requested-transform failures
// are classified before or after this seam. Cancellation retains its identity.
func featureSourceError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return provider.FeatureDataError{Err: err}
}

// Each chunk owns its rows in a separate scope; every exit closes the cursor
// before the next query, preserving callback/context and cleanup error chains.
func scanFeatureCandidates(
	ctx context.Context,
	tx *sql.Tx,
	statement string,
	args []any,
	columnCount int,
	fn func([]any) (bool, error),
) (read int, stopped bool, err error) {
	rows, err := tx.QueryContext(ctx, statement, args...)
	if err != nil {
		return 0, false, fmt.Errorf("gpkg feature candidate query: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("gpkg feature candidate close: %w", closeErr))
		}
	}()
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return read, false, err
		}
		values, pointers := make([]any, columnCount), make([]any, columnCount)
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return read, false, fmt.Errorf("gpkg feature row scan: %w", err)
		}
		read++
		stop, err := fn(values)
		if err != nil || stop {
			return read, stop, err
		}
	}
	if err := rows.Err(); err != nil {
		return read, false, fmt.Errorf("gpkg feature candidate iteration: %w", err)
	}
	return read, false, nil
}

func resolvedLayerSRID(layer *Layer) uint64 {
	if layer.srid == 0 {
		return DefaultSRID
	}
	return layer.srid
}

func validateQueryCRS(srid uint64) error {
	if srid == 4326 {
		return nil
	}
	code := proj.EPSGCode(srid)
	if srid == 0 || code < 0 || uint64(code) != srid || !proj.IsKnownConversionSRID(code) {
		return fmt.Errorf("gpkg query CRS profile: %w", provider.ErrUnsupported)
	}
	// Empty coordinate arrays construct the existing engine without testing
	// an arbitrary location outside a restricted geographic domain.
	converted, err := proj.Convert(code, []float64{})
	if err != nil {
		return fmt.Errorf("gpkg query CRS conversion: %w: %v", provider.ErrUnsupported, err)
	}
	if _, err := proj.Inverse(code, converted); err != nil {
		return fmt.Errorf("gpkg query CRS inverse conversion: %w: %v", provider.ErrUnsupported, err)
	}
	return nil
}

func containsFold(fields []string, field string) bool {
	for _, candidate := range fields {
		if strings.EqualFold(candidate, field) {
			return true
		}
	}
	return false
}

func resolveQueryFields(layer *Layer, requested []string) ([]string, error) {
	available := layer.tagFieldnames
	if len(available) == 0 {
		available = layer.featureColumns
	}
	var public []string
	for _, field := range available {
		if strings.EqualFold(field, layer.idFieldname) || strings.EqualFold(field, layer.geomFieldname) ||
			layer.bboxFields.IsBBoxField(field) || strings.EqualFold(field, "min_zoom") || strings.EqualFold(field, "max_zoom") {
			continue
		}
		if !containsFold(public, field) {
			public = append(public, field)
		}
	}
	if len(requested) == 0 {
		return append([]string(nil), public...), nil
	}
	var resolved []string
	for _, field := range requested {
		found := ""
		for _, candidate := range public {
			if strings.EqualFold(field, candidate) {
				found = candidate
				break
			}
		}
		if found == "" || containsFold(resolved, found) {
			return nil, invalidQuery("fields", "unknown or duplicate published property")
		}
		resolved = append(resolved, found)
	}
	return resolved, nil
}

func queryColumns(layer *Layer, fields []string) []string {
	columns := []string{layer.idFieldname, layer.geomFieldname}
	for _, field := range append(append([]string(nil), fields...),
		layer.temporalMapping.InstantField, layer.temporalMapping.StartField, layer.temporalMapping.EndField) {
		if field != "" && !containsFold(columns, field) {
			columns = append(columns, field)
		}
	}
	return columns
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

func featureMatchesBounds(geometry geom.Geometry, source uint64, query provider.FeatureQuery) (bool, error) {
	if len(query.Bounds) == 0 {
		return codec.FeatureGeometryIntersectsExtent(geometry, nil)
	}
	transformed, err := transformQueryGeometry(geometry, source, query.BoundsSRID)
	if err != nil {
		return false, fmt.Errorf("gpkg exact query CRS transform: %w", err)
	}
	matched := false
	for _, bounds := range query.Bounds {
		extent := bounds
		intersects, err := codec.FeatureGeometryIntersectsExtent(transformed, &extent)
		if err != nil {
			return false, err
		}
		matched = matched || intersects
	}
	return matched, nil
}

func featureCandidatePredicate(layer *Layer, query provider.FeatureQuery) (string, []any, bool, error) {
	predicates := []string{"l." + quoteIdent(layer.idFieldname) + " IS NOT NULL"}
	var args []any
	if len(query.IDs) != 0 {
		var placeholders []string
		seen := make(map[uint64]bool)
		for _, id := range query.IDs {
			if id <= math.MaxInt64 && !seen[id] {
				placeholders = append(placeholders, "?")
				args = append(args, int64(id))
				seen[id] = true
			}
		}
		if len(placeholders) == 0 {
			return "0", nil, true, nil
		}
		predicates = append(predicates, "l."+quoteIdent(layer.idFieldname)+" IN ("+strings.Join(placeholders, ",")+")")
	}
	predicates = append(predicates, temporalPredicate(layer, query.Temporal, &args))
	bounds := query.Bounds
	if len(query.Bounds3D) != 0 {
		bounds = make([]geom.Extent, len(query.Bounds3D))
		for i, b := range query.Bounds3D {
			bounds[i] = geom.Extent{b[0], b[1], b[3], b[4]}
		}
	}
	identity := query.BoundsSRID == resolvedLayerSRID(layer)
	if query.BoundsCRSDefinition != "" {
		target, err := crsconfig.NewFeatureProjection(query.BoundsCRSDefinition)
		if err != nil {
			return "", nil, false, fmt.Errorf("query CRS definition unsupported: %w", provider.ErrUnsupported)
		}
		identity = layer.featureCRSProjection != nil && layer.featureCRSProjection.Equivalent(target)
	}
	if len(bounds) != 0 && identity {
		spatial, err := spatialCandidatePredicate(layer, bounds, &args)
		if err != nil {
			return "", nil, false, err
		}
		if spatial != "" {
			predicates = append(predicates, "("+spatial+")")
		}
	}
	return "(" + strings.Join(predicates, ") AND (") + ")", args, false, nil
}

// Absence masks deliberately include unclassifiable short/wrong-type values
// for strict decoding, rather than silently losing NULL/empty geometry.
func absenceCandidateSQL(layer *Layer) string {
	field := "l." + quoteIdent(layer.geomFieldname)
	base := field + " IS NULL"
	switch layer.geometryFormat {
	case "", GeometryFormatGPKG:
		envelope := "CASE substr(hex(substr(" + field + ",4,1)),2,1) WHEN '0' THEN 10 WHEN '1' THEN 10 WHEN '2' THEN 42 WHEN '3' THEN 42 WHEN '4' THEN 58 WHEN '5' THEN 58 WHEN '6' THEN 58 WHEN '7' THEN 58 WHEN '8' THEN 74 WHEN '9' THEN 74 ELSE 0 END"
		// Native empty flags are authoritative. Containers may nevertheless
		// decode wholly empty; include unclassified containers conservatively.
		return base + " OR typeof(" + field + ")<>'blob' OR length(" + field + ")<8 OR substr(hex(substr(" + field + ",4,1)),1,1) IN ('1','3','5','7','9','B','D','F') OR hex(substr(" + field + "," + envelope + ",4)) NOT IN ('01000000','00000001','02000000','00000002') OR (hex(substr(" + field + "," + envelope + ",4)) IN ('02000000','00000002') AND hex(substr(" + field + ",(" + envelope + ")+4,4))='00000000')"
	case GeometryFormatWKT:
		return base + " OR typeof(" + field + ")<>'text' OR upper(" + field + ") LIKE '%EMPTY%'"
	case GeometryFormatWKB:
		return base + " OR typeof(" + field + ")<>'blob' OR length(" + field + ")<=9 OR hex(substr(" + field + ",2,4)) NOT IN ('01000000','00000001','02000000','00000002') OR (" + rawEmptyPointSQL(field) + ")"
	case GeometryFormatMOS:
		// MOS supports polygon, polyline, point, text and image headers.
		// Treating every non-point header as absent defeats bounds pruning
		// for whole network/polygon tables and decodes every row per request.
		return base + " OR typeof(" + field + ")<>'blob' OR length(" + field + ")<10 OR hex(substr(" + field + ",1,1)) NOT IN ('00','01','02','03','04') OR hex(substr(" + field + ",7,4))='00000000'"
	}
	return "1"
}

func spatialCandidatePredicate(layer *Layer, bounds []geom.Extent, args *[]any) (string, error) {
	var union []string
	absent := absenceCandidateSQL(layer)
	if (layer.geometryFormat == "" || layer.geometryFormat == GeometryFormatGPKG) && layer.tileQueryPlan == planRTree {
		index := quoteIdent("rtree_" + layer.tablename + "_" + layer.geomFieldname)
		table := quoteIdent(layer.tablename)
		identity := "l." + quoteIdent(layer.idFieldname)
		rowID := "l." + quoteIdent(layer.featureRowIDAlias)
		for _, extent := range bounds {
			union = append(union, "SELECT "+identity+" FROM "+index+" r JOIN "+table+" l ON "+rowID+"=r.id WHERE r.minx<=? AND r.maxx>=? AND r.miny<=? AND r.maxy>=?")
			*args = append(*args, extent[2], extent[0], extent[3], extent[1])
		}
		// Indexless rows cannot safely be rejected on spatial metadata.
		union = append(union, "SELECT "+identity+" FROM "+table+" l WHERE "+absent+" OR NOT EXISTS (SELECT 1 FROM "+index+" r WHERE r.id="+rowID+")")
		return identity + " IN (" + strings.Join(union, " UNION ") + ")", nil
	}
	if layer.boundFieldnames != nil {
		table := quoteIdent(layer.tablename)
		identity := "l." + quoteIdent(layer.idFieldname)
		for _, extent := range bounds {
			coordinates := [4]float64{extent[0], extent[2], extent[1], extent[3]}
			if layer.geometryFormat == GeometryFormatMOS {
				scale, err := codec.MOSRawScale(layer.mosConfig)
				if err != nil {
					return "", fmt.Errorf("gpkg MOS bounds scale: %w", err)
				}
				if math.IsNaN(scale) || math.IsInf(scale, 0) || scale <= 0 {
					return "", fmt.Errorf("gpkg MOS bounds scale: %w", provider.ErrUnsupported)
				}
				coordinates = [4]float64{math.Floor(extent[0] * scale), math.Ceil(extent[2] * scale), math.Floor(extent[1] * scale), math.Ceil(extent[3] * scale)}
			}
			for _, value := range coordinates {
				if math.IsNaN(value) || math.IsInf(value, 0) {
					return "", fmt.Errorf("gpkg quantized bounds range: %w", provider.ErrUnsupported)
				}
			}
			fields := layer.boundFieldnames
			union = append(union, "SELECT "+identity+" FROM "+table+" l WHERE l."+quoteIdent(fields[1])+">=? AND l."+quoteIdent(fields[0])+"<=? AND l."+quoteIdent(fields[3])+">=? AND l."+quoteIdent(fields[2])+"<=?")
			*args = append(*args, coordinates[0], coordinates[1], coordinates[2], coordinates[3])
		}
		var absence []string
		for _, field := range *layer.boundFieldnames {
			absence = append(absence, "l."+quoteIdent(field)+" IS NULL")
		}
		absence = append(absence, absent)
		union = append(union, "SELECT "+identity+" FROM "+table+" l WHERE "+strings.Join(absence, " OR "))
		return identity + " IN (" + strings.Join(union, " UNION ") + ")", nil
	}
	return "", nil
}
