package postgis

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/internal/env"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/internal/sqltoken"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/tracelog"
)

// buildBuffer joins the provided values slice using a comma delimiter.
func buildBuffer(vals []string) string {
	if len(vals) == 0 {
		return "[]"
	}
	size := len(vals) - 1 // n-1 times ","
	for _, v := range vals {
		size += len(v)
	}

	buf := bytes.NewBuffer(make([]byte, 0, size))
	for i, v := range vals {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(v)
	}
	return buf.String()
}

// isMVT will return true if the provider is MVT based
func isMVT(providerType string) bool {
	return providerType == MVTProviderType
}

// genSQL will fill in the SQL field of a layer given a pool, and list of fields.
func genSQL(
	l *Layer,
	pool *connectionPoolCollector,
	tblname string,
	flds []string,
	buffer bool,
	providerType string,
) (sql string, err error) {
	// we need to hit the database to see what the fields are.
	if len(flds) == 0 {
		sql := fmt.Sprintf(fldsSQL, tblname)

		//	if a subquery is set in the 'sql' config the subquery is set to the layer's
		//	'tablename' param. because of this case normal SQL token replacement needs to be
		//	applied to tablename SQL generation
		tile := provider.NewTile(0, 0, 0, 64, tegola.WebMercator)
		sql, err = replaceTokens(sql, l, tile, buffer)
		if err != nil {
			return "", err
		}

		ictx, icancel := codec.NewInspectionContext()
		defer icancel()

		rows, err := pool.Query(ictx, sql)
		if err != nil {
			return "", err
		}
		defer rows.Close()

		fdescs := rows.FieldDescriptions()
		if len(fdescs) == 0 {
			return "", fmt.Errorf("no fields were returned for table %v", tblname)
		}

		// to avoid field names possibly colliding with Postgres keywords,
		// we wrap the field names in quotes
		for i := range fdescs {
			flds = append(flds, string(fdescs[i].Name))
		}
	}

	fgeom := -1

	for i, f := range flds {
		if f == l.geomField {
			fgeom = i
		}
		flds[i] = pgQuoteIdent(flds[i])
	}

	// rawGeometryFormat reports whether the geometry column carries a raw
	// value (plain WKB/WKT/MOS blob) that must be selected verbatim instead
	// of being wrapped in ST_AsBinary.
	rawGeometryFormat := codec.IsRawFormat(l.geometryFormat)

	// to avoid field names possibly colliding with Postgres keywords,
	// we wrap the field names in quotes

	quotedGeom := pgQuoteIdent(l.geomField)

	if fgeom == -1 {
		if isMVT(providerType) {
			flds = append(flds, fmt.Sprintf(`%[1]v AS %[1]v`, quotedGeom))
		} else if rawGeometryFormat {
			// Raw geometry columns must bypass ST_AsBinary: MOS blobs and
			// plain WKB/WKT values are not PostGIS geometry values and
			// native spatial functions cannot be applied to them.
			flds = append(flds, fmt.Sprintf(`%[1]v AS %[1]v`, quotedGeom))
		} else {
			flds = append(flds, fmt.Sprintf(`ST_AsBinary(%[1]v) AS %[1]v`, quotedGeom))
		}
	} else {
		if isMVT(providerType) || rawGeometryFormat {
			flds[fgeom] = fmt.Sprintf(`%[1]v AS %[1]v`, quotedGeom)
		} else {
			flds[fgeom] = fmt.Sprintf(`ST_AsBinary(%[1]v) AS %[1]v`, quotedGeom)
		}
	}

	// add required id field
	if l.idField != "" {
		flds = append(flds, pgQuoteIdent(l.idField))
	}

	selectClause := strings.Join(flds, ", ")

	sqlTmpl := stdSQL

	if isMVT(providerType) {
		sqlTmpl = mvtSQL
	} else if codec.IsRawFormat(l.geometryFormat) {
		// Raw geometry columns (MOS blobs, plain WKB/WKT) are not PostGIS
		// geometries: native spatial predicates (&&, ST_Intersects, ...)
		// cannot be applied to them. Filtering is done in memory after
		// decoding (see TileFeatures).
		sqlTmpl = `SELECT %[1]v FROM %[2]v WHERE %[3]v IS NOT NULL`
	} else if basic.IsSyntheticSRID(l.srid) {
		// A textual crs_defn is registered only in Tegola. Keep generated
		// database comparisons in the unknown-SRS-safe SRID 0 domain; Atlas
		// still interprets returned feature coordinates with l.srid.
		sqlTmpl = `SELECT %[1]v FROM %[2]v WHERE ST_SetSRID(%[3]v,0) && ` + config.BboxToken
	}

	return fmt.Sprintf(sqlTmpl, selectClause, tblname, quotedGeom), nil
}

// replaceTokens replaces tokens in the provided SQL string
//
// !BBOX! - the bounding box of the tile
// !ZOOM! - the tile Z value
// !X! - the tile X value
// !Y! - the tile Y value
// !Z! - the tile Z value
// !SCALE_DENOMINATOR! - scale denominator, assuming 90.7 DPI (i.e. 0.28mm pixel size)
// !PIXEL_WIDTH! - unbuffered pixel width in the resolved layer CRS units
// !PIXEL_HEIGHT! - unbuffered pixel height in the resolved layer CRS units
// !GEOM_FIELD! - the geom field name
// !GEOM_TYPE! - the geom field type if defined otherwise ""
//
// Only tokens in SQL code context are substituted; token-looking text
// inside string literals, quoted identifiers, comments, or dollar-quoted
// strings is left verbatim.
func replaceTokens(sql string, lyr *Layer, tile provider.Tile, withBuffer bool) (string, error) {
	var (
		extent  *geom.Extent
		geoType string
	)

	if lyr == nil {
		return "", ErrNilLayer
	}
	srid := lyr.SRID()

	if withBuffer {
		extent, _ = tile.BufferedExtent()
	} else {
		extent, _ = tile.Extent()
	}

	// Convert the complete tile perimeter into the source CRS. Sampling the
	// perimeter is conservative for nonlinear or rotated projections, unlike
	// converting only the two opposite corners.
	sourceExtent, err := basic.FromWebMercatorExtent(srid, extent)
	if err != nil {
		return "", fmt.Errorf("error converting tile extent: %w", err)
	}

	bboxSRID := srid
	if basic.IsSyntheticSRID(srid) {
		// Synthetic SRIDs are process-local Tegola identifiers and are not
		// present in PostGIS spatial_ref_sys. Use SRID 0 for the coordinate
		// comparison so PostgreSQL never receives an unknown SRS identifier.
		bboxSRID = 0
	}
	bbox := fmt.Sprintf(
		"ST_MakeEnvelope(%.8f,%.8f,%.8f,%.8f,%d)",
		sourceExtent.MinX(),
		sourceExtent.MinY(),
		sourceExtent.MaxX(),
		sourceExtent.MaxY(),
		bboxSRID,
	)

	// Raw (MOS) custom-SQL layers cannot use the native envelope: their
	// !BBOX! token expands to the bounds predicate over the configured
	// bounds fields with MOS raw scaling instead. Custom SQL for MOS is
	// required to carry the token (RequireBBoxCustomSQL), and tablename
	// MOS layers never reach the !BBOX! path (raw sqlTmpl has no token).
	if lyr.geometryFormat == codec.FormatMOS {
		predicate, perr := codec.BuildBoundsPredicate(
			lyr.bboxFields, sourceExtent, codec.BoundsMOSRaw, lyr.mosConfig,
			codec.BoundsQuote(lyr.bboxTable, pgQuoteIdent),
		)
		if perr != nil {
			return "", fmt.Errorf("layer (%v): %w", lyr.name, perr)
		}
		bbox = predicate
	}

	var pixelWidth, pixelHeight, scaleDenominator float64
	if sqltoken.PostgreSQL.ContainsTokenFold(sql, config.PixelWidthToken, config.PixelHeightToken, config.ScaleDenominatorToken) {
		var err error
		pixelWidth, pixelHeight, scaleDenominator, err = provider.TileScale(tile, lyr.SRID())
		if err != nil {
			return "", fmt.Errorf("layer (%v) scale tokens: %w", lyr.name, err)
		}
	}

	if lyr.GeomType() != nil {
		geoType = fmt.Sprintf("%v", lyr.GeomType())
	}

	// replace query string tokens
	z, x, y := tile.ZXY()
	sql = uppercaseTokens(sql)
	return sqltoken.PostgreSQL.MapTokens(sql, func(tok string) string {
		switch tok {
		case config.BboxToken:
			return bbox
		case config.ZoomToken, config.ZToken:
			return strconv.FormatUint(uint64(z), 10)
		case config.XToken:
			return strconv.FormatUint(uint64(x), 10)
		case config.YToken:
			return strconv.FormatUint(uint64(y), 10)
		case config.ScaleDenominatorToken:
			return strconv.FormatFloat(scaleDenominator, 'f', 8, 64)
		case config.PixelWidthToken:
			return strconv.FormatFloat(pixelWidth, 'f', 8, 64)
		case config.PixelHeightToken:
			return strconv.FormatFloat(pixelHeight, 'f', 8, 64)
		case config.IdFieldToken:
			return quoteTokenIdentifier(lyr.IDFieldName())
		case config.GeomFieldToken:
			return quoteTokenIdentifier(lyr.GeomFieldName())
		case config.GeomTypeToken:
			return geoType
		}
		return tok
	}), nil
}

// pgQuoteIdent wraps a PostgreSQL identifier in double quotes, doubling any
// embedded quotes (SQL standard identifier escaping).
func pgQuoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// escapeSQLStringLiteral escapes a value embedded in a single-quoted SQL
// string literal by doubling single quotes (audit P5-8).
func escapeSQLStringLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// sqlStringLiteral wraps a value in a single-quoted SQL string literal,
// escaping embedded quotes (audit P5-8).
func sqlStringLiteral(s string) string {
	return "'" + escapeSQLStringLiteral(s) + "'"
}

// rawGeometryBoundsWarning returns the registration-time warning for a
// raw-geometry-format layer (wkb/wkt/mos) whose per-tile query has no
// bounds-columns-backed server-side predicate: tegola scans the whole layer
// source on every tile request and filters geometries in memory. Returns ""
// when no warning is needed (audit P6-19).
func rawGeometryBoundsWarning(layerName, geometryFormat string, boundsPredicateUsed bool) string {
	if !codec.IsRawFormat(geometryFormat) || boundsPredicateUsed {
		return ""
	}
	return fmt.Sprintf(
		"layer (%v): geometry_format=%q stores raw geometry without bounds columns backing a server-side filter; every tile request scans the full table and filters geometries in memory (O(rows) per tile). Configure bounds columns (bbox_minx_fieldname/bbox_maxx_fieldname/bbox_miny_fieldname/bbox_maxy_fieldname) with a bounds-backed MOS query carrying !BBOX! so tile requests filter server-side, or use a native geometry column (audit P6-19)",
		layerName, geometryFormat,
	)
}

// rawGeometryBoundsWarnings returns the sorted set of registration-time
// warnings for the configured layers (audit P6-19). For PostGIS only
// bounds-backed custom SQL counts as a server-side filter for a raw-format
// layer: its !BBOX! token expands to the bounds-columns predicate, while
// the generated table SQL for raw formats carries no spatial filter at all.
func rawGeometryBoundsWarnings(layers map[string]Layer) []string {
	var msgs []string
	for _, l := range layers {
		boundsPredicateUsed := codec.PostgreSQL.SQLHasBBoxToken(l.sql, config.BboxToken, "!BOX!")
		if msg := rawGeometryBoundsWarning(l.name, l.geometryFormat, boundsPredicateUsed); msg != "" {
			msgs = append(msgs, msg)
		}
	}
	sort.Strings(msgs)
	return msgs
}

// buildMVTLayerSQL builds the per-layer ST_AsMVT subquery. The name,
// geometry-field and feature-id arguments are single-quoted SQL literals;
// embedded single quotes are escaped by doubling (audit P5-8). An empty
// idFieldName produces NULL (no feature id) as before.
func buildMVTLayerSQL(mvtName, geomFieldName, idFieldName, innerSQL string) string {
	featureIDName := "NULL"
	if idFieldName != "" {
		featureIDName = sqlStringLiteral(idFieldName)
	}
	// ref: https://postgis.net/docs/ST_AsMVT.html
	// bytea ST_AsMVT(any_element row, text name, integer extent, text geom_name, text feature_id_name)
	return fmt.Sprintf(
		`(SELECT ST_AsMVT(q,%s,%d,%s,%s) AS data FROM (%s) AS q)`,
		sqlStringLiteral(mvtName),
		tegola.DefaultExtent,
		sqlStringLiteral(geomFieldName),
		featureIDName,
		innerSQL,
	)
}

// quoteTokenIdentifier quotes an identifier substituted for an
// !ID_FIELD!/!GEOM_FIELD! SQL token (audit P5-10). Values the user already
// wrapped in a complete quote pair (double quotes or backticks) pass through
// verbatim for backward compatibility; qualified names are quoted per part
// so table-qualified fields keep working; anything else is escaped with
// pgQuoteIdent so hostile names cannot break out of the identifier.
func quoteTokenIdentifier(name string) string {
	if isQuotedIdentifierValue(name) {
		return name
	}
	parts := strings.Split(name, ".")
	for i := range parts {
		parts[i] = pgQuoteIdent(parts[i])
	}
	return strings.Join(parts, ".")
}

// isQuotedIdentifierValue reports whether name is already wrapped in a
// complete quote pair (double quotes or backticks). Semantics are kept
// identical to the mysql provider's token quoting (audit P5-10).
func isQuotedIdentifierValue(name string) bool {
	if len(name) < 2 {
		return false
	}
	if name[0] != '`' && name[0] != '"' {
		return false
	}
	return name[len(name)-1] == name[0]
}

// extractQueryParamValues finds default values for SQL tokens and constructs query parameter values out of them
func extractQueryParamValues(pname string, maps []provider.Map, layer *Layer) provider.Params {
	result := make(provider.Params, 0)

	expectedMapName := fmt.Sprintf("%s.%s", pname, layer.name)
	for _, m := range maps {
		for _, l := range m.Layers {
			if l.ProviderLayer == env.String(expectedMapName) {
				for _, p := range m.Parameters {
					pv, err := p.ToDefaultValue()
					if err == nil {
						result[p.Token] = pv
					}
				}
			}
		}
	}

	return result
}

// uppercaseTokens converts all !tokens! in SQL code context to uppercase
// !TOKENS!. Tokens can contain alphanumerics, dash and underline chars.
// Token-looking text in string literals, identifiers, or comments keeps
// its exact bytes.
func uppercaseTokens(str string) string {
	return sqltoken.PostgreSQL.MapTokens(str, strings.ToUpper)
}

func transformVal(valType uint32, val any) (any, error) {
	switch valType {
	default:
		switch vt := val.(type) {
		default:
			log.Errorf("%v type is not supported. (Expected it to be a stringer type)", valType)
			return nil, fmt.Errorf("%v type is not supported. (Expected it to be a stringer type)", valType)
		case fmt.Stringer:
			return vt.String(), nil
		case string:
			return vt, nil
		}
	case pgtype.BoolOID,
		pgtype.ByteaOID,
		pgtype.TextOID,
		pgtype.OIDOID,
		pgtype.VarcharOID,
		pgtype.JSONBOID:
		return val, nil
	case pgtype.Int8OID,
		pgtype.Int2OID,
		pgtype.NumericOID,
		pgtype.Int4OID,
		pgtype.Float4OID,
		pgtype.Float8OID:
		switch vt := val.(type) {
		case int8:
			return int64(vt), nil
		case int16:
			return int64(vt), nil
		case int32:
			return int64(vt), nil
		case int64, uint64:
			return vt, nil
		case uint8:
			return int64(vt), nil
		case uint16:
			return int64(vt), nil
		case uint32:
			return int64(vt), nil
		case float32:
			return float64(vt), nil
		case float64:
			return vt, nil
		default: // should never happen.
			return nil, fmt.Errorf("%v type is not supported. (should never happen)", valType)
		}
	case pgtype.DateOID, pgtype.TimestampOID, pgtype.TimestamptzOID:
		return fmt.Sprintf("%v", val), nil
	}
}

// decipherFields is responsible for processing the SQL result set, decoding geometries, ids and feature tags.
// When boundsFields is non-nil, the four configured bounds columns
// (bbox_minx_fieldname etc.) are excluded from the tags: they are
// implementation details of the bounds-backed MOS SQL contract, not feature
// attributes.
func decipherFields(
	ctx context.Context,
	geomFieldname, idFieldname string,
	boundsFields *codec.BBoxFields,
	descriptions []pgconn.FieldDescription,
	values []any,
) (gid uint64, geom []byte, tags map[string]any, err error) {
	tags = make(map[string]any)

	var idParsed bool
	for i := range values {

		// do a quick check
		if err := ctx.Err(); err != nil {
			return 0, nil, nil, err
		}

		// skip nil values.
		if values[i] == nil {
			continue
		}

		desc := descriptions[i]
		descName := string(desc.Name)

		if boundsFields != nil && boundsFields.IsBBoxField(descName) {
			continue
		}

		switch descName {
		case geomFieldname:
			// pgx returns Go strings for text/varchar geometry columns
			// (geometry_format="wkt" reads WKT from such columns), while bytea
			// columns come back as []byte; accept both (audit P6-2).
			switch gval := values[i].(type) {
			case []byte:
				geom = gval
			case string:
				geom = []byte(gval)
			default:
				return 0, nil, nil, fmt.Errorf(
					"unable to convert geometry field (%v) into bytes",
					geomFieldname,
				)
			}
		case idFieldname:
			// the id has to be parsed once but it can also be a tag
			if !idParsed {
				gid, err = gId(values[i])
				if err != nil {
					return 0, nil, nil, err
				}
				idParsed = true
				// NOTE: if it can also be a tag, then breaking here
				// will never add it to tags
				break
			}

			// adds id as a tag
			fallthrough
		default:
			switch vex := values[i].(type) {
			case map[string]pgtype.Text:
				for key, value := range vex {
					// we need to check if the key already exists. if it does, then don't overwrite it
					if _, ok := tags[key]; !ok {
						tags[key] = value
					} else {
						log.Warnf("tag %s already exists", key)
					}
				}
			case pgtype.Hstore:
				for key, ptr := range vex {
					value := ""
					if ptr != nil {
						value = *ptr
					}

					// we need to check if the key already exists. if it does, then don't overwrite it
					if _, ok := tags[key]; !ok {
						tags[key] = value
					} else {
						log.Warnf("tag %s already exists", key)
					}
				}
			case pgtype.Numeric:
				num, err := vex.Float64Value()
				if err != nil {
					return 0, nil, nil, fmt.Errorf("unable to scan numeric field (%v) into float64", vex)
				}

				tags[descName] = num.Float64
			default:
				value, err := transformVal(desc.DataTypeOID, values[i])
				if err != nil {
					return gid,
						geom,
						tags,
						fmt.Errorf("unable to convert field [%v] (%v) of type (%v) to a suitable value: %+v (%T)",
							i,
							descName,
							desc.DataTypeOID,
							values[i],
							values[i],
						)
				}
				tags[descName] = value
			}
		}
	}

	return gid, geom, tags, nil
}

// gId converts a feature ID value to uint64. The conversion rules (audit
// P6-10: reject negative, fractional and out-of-range IDs with a clear
// error) live in provider.ConvertFeatureID so every provider call site
// behaves identically.
func gId(v any) (gid uint64, err error) {
	return provider.ConvertFeatureID(v)
}

// ctxErr will check if the supplied context has an error (i.e. context canceled)
// and if so, return that error, else return the supplied error. This is useful
// as not all of Go's stdlib has adopted error wrapping so context.Canceled
// errors are not always easy to capture.
func ctxErr(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}

	return err
}

// NOTE: @iwpnd to remove this adapter once we move to slog
// LoggerAdapter adapts the internal logger to the pgx tracelog.Logger interface.
type LoggerAdapter struct{}

func NewLoggerAdapter() *LoggerAdapter {
	return &LoggerAdapter{}
}

// Log is the implementation of the Log method required by pgx's tracelog.Logger interface.
// It logs messages with the warn level only.
func (l *LoggerAdapter) Log(
	ctx context.Context, level tracelog.LogLevel,
	msg string, data map[string]any,
) {
	// drop >3, where 2=Error 3=Warn
	if level > tracelog.LogLevelWarn {
		return
	}

	if level == tracelog.LogLevelError {
		log.Errorf("PostGIS(pgx): %s, %#v", msg, data)
	} else {
		log.Warnf("PostGIS(pgx): %s, %#v", msg, data)
	}
}
