package hana

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/SAP/go-hdb/driver"
	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/internal/env"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/internal/sqltoken"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

const (
	bboxToken             = "!BBOX!"
	zoomToken             = "!ZOOM!"
	xToken                = "!X!"
	yToken                = "!Y!"
	zToken                = "!Z!"
	scaleDenominatorToken = "!SCALE_DENOMINATOR!"
	pixelWidthToken       = "!PIXEL_WIDTH!"
	pixelHeightToken      = "!PIXEL_HEIGHT!"
	idFieldToken          = "!ID_FIELD!"
	geomFieldToken        = "!GEOM_FIELD!"
	geomTypeToken         = "!GEOM_TYPE!"
)

// isSelectQuery is a regexp to check if a query starts with `SELECT`,
// case-insensitive and ignoring any preceeding whitespace and SQL comments.
var isSelectQueryRe = regexp.MustCompile(`(?i)^((\s*)(--.*\n)?)*select`)

func isSelectQuery(sql string) bool {
	return isSelectQueryRe.MatchString(sql)
}

// parseQuotedIdent reports whether the whole input is exactly one
// quoted HANA identifier ("..." with "" escapes for embedded quotes).
// A closing quote followed by trailing content is rejected, so hostile
// input like `"a"; DROP ...` never parses as a valid quoted identifier.
func parseQuotedIdent(name string) bool {
	// Audit P5-12: an empty quoted identifier `""` has no name content and
	// is not a valid identifier, so the minimum accepted form is 3 bytes
	// (`"x"`); `""""` (a single literal quote as the name) stays valid.
	if len(name) < 3 || name[0] != '"' {
		return false
	}
	for i := 1; i < len(name); {
		if name[i] != '"' {
			i++
			continue
		}
		if i == len(name)-1 {
			return true // closing quote ends the identifier
		}
		if name[i+1] == '"' {
			i += 2 // escaped "" quote inside the identifier
			continue
		}
		return false // closing quote with trailing garbage
	}
	return false // unterminated quoted identifier
}

// quoteIdentifier quotes a HANA identifier. A valid quoted identifier is
// passed through unchanged; anything else is quoted as a literal
// identifier with embedded quotes doubled, so no input can ever escape
// the quoting (audit N12).
func quoteIdentifier(name string) string {
	if parseQuotedIdent(name) {
		return name
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// escapeSQLStringLiteral escapes a value for interpolation inside a
// single-quoted SQL string literal (audit P5-8): single quotes are
// doubled so they cannot terminate the literal early.
func escapeSQLStringLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// sqlStringLiteral wraps s as a complete single-quoted SQL string
// literal with escaping applied (audit P5-8 helper contract).
func sqlStringLiteral(s string) string {
	return "'" + escapeSQLStringLiteral(s) + "'"
}

// isQuotedIdentifierValue reports whether name is already wrapped in one
// complete identifier quote pair (double quote or backtick, with doubled
// quote escapes) spanning the whole value (audit P5-10 contract). A
// single-quote pair is a SQL string literal, never an identifier, so it
// is never passed through.
func isQuotedIdentifierValue(name string) bool {
	if len(name) < 2 {
		return false
	}
	q := name[0]
	if q != '"' && q != '`' {
		return false
	}
	if name[len(name)-1] != q {
		return false
	}
	for i := 1; i < len(name); i++ {
		if name[i] == q {
			if i+1 < len(name) && name[i+1] == q {
				i++
				continue
			}
			return i == len(name)-1
		}
	}
	return false
}

// splitIdentDots splits a possibly qualified identifier at top-level
// dots, keeping dots that sit inside quoted segments ("a.b".c has one
// dot). An unclosed quote keeps the remainder in the current part, so
// hostile values are never split into injection-shaped pieces. This is
// the lenient token-value counterpart of the strict parseIdentParts
// registration parser (audit P5-3).
func splitIdentDots(v string) []string {
	var parts []string
	var cur strings.Builder
	var quote byte
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case quote != 0:
			cur.WriteByte(c)
			if c == quote {
				if i+1 < len(v) && v[i+1] == quote {
					cur.WriteByte(quote)
					i++
					continue
				}
				quote = 0
			}
		case c == '"' || c == '`' || c == '\'':
			quote = c
			cur.WriteByte(c)
		case c == '.':
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	parts = append(parts, cur.String())
	return parts
}

// quoteTokenIdentifier quotes an identifier substituted for the
// !ID_FIELD! / !GEOM_FIELD! SQL tokens (audit P5-10 contract): values
// already wrapped in a complete identifier quote pair pass through
// verbatim; the empty string (the no-id-field sentinel) passes through
// unchanged; everything else is quoted per identifier part
// (schema.table.col => "schema"."table"."col"), so dots inside quoted
// parts survive and hostile values can never escape the quoting.
func quoteTokenIdentifier(name string) string {
	if name == "" || isQuotedIdentifierValue(name) {
		return name
	}
	parts := splitIdentDots(name)
	for i := range parts {
		if isQuotedIdentifierValue(parts[i]) {
			continue
		}
		parts[i] = quoteIdentifier(parts[i])
	}
	return strings.Join(parts, ".")
}

// validateIdentName rejects empty identifier names before they can be
// quoted into SQL (audit P5-12). Quoting cannot express an empty
// identifier: `""` would become the literal two-character name `""`
// after quoting, so empty names must be refused at registration instead.
func validateIdentName(name string) error {
	if name == "" {
		return fmt.Errorf("identifier name is empty; expected a non-empty identifier name")
	}
	if name == `""` {
		return fmt.Errorf(`identifier %q is empty; expected a non-empty identifier name`, name)
	}
	return nil
}

// parseIdentParts splits a possibly qualified HANA identifier into its
// unquoted parts (audit P5-3). Quoted parts may contain dots and
// escaped double quotes (""); empty parts, unterminated quotes,
// unexpected characters after a closing quote, quotes inside bare
// parts, and more than two parts (schema.table) are rejected.
func parseIdentParts(name string) ([]string, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("invalid identifier %q: identifier is empty", name)
	}

	var parts []string
	var cur strings.Builder
	inQuote := false
	closedQuote := false // the current part just closed its quotes

	flush := func() error {
		if cur.Len() == 0 {
			return fmt.Errorf("invalid identifier %q: empty name part", name)
		}
		parts = append(parts, cur.String())
		cur.Reset()
		closedQuote = false
		if len(parts) > 2 {
			return fmt.Errorf("invalid identifier %q: expected at most schema.table (2 parts), got %d", name, len(parts))
		}
		return nil
	}

	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case inQuote:
			if c == '"' {
				if i+1 < len(name) && name[i+1] == '"' {
					// escaped quote inside a quoted part
					cur.WriteByte('"')
					i++
					continue
				}
				inQuote = false
				closedQuote = true
				continue
			}
			cur.WriteByte(c)
		case c == '.':
			if err := flush(); err != nil {
				return nil, err
			}
		case c == '"':
			if cur.Len() > 0 || closedQuote {
				return nil, fmt.Errorf("invalid identifier %q: unexpected quote", name)
			}
			inQuote = true
		case closedQuote:
			return nil, fmt.Errorf("invalid identifier %q: unexpected content after closing quote", name)
		default:
			if cur.Len() == 0 && (c == ' ' || c == '\t') {
				return nil, fmt.Errorf("invalid identifier %q: unexpected whitespace", name)
			}
			cur.WriteByte(c)
		}
	}
	if inQuote {
		return nil, fmt.Errorf("invalid identifier %q: unterminated quoted identifier", name)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return parts, nil
}

// splitQualifiedTableName splits a qualified HANA table reference into
// its unquoted schema and bare table name parts (audit P5-3). An
// unqualified name yields an empty schema (resolved against the
// connection's CURRENT SCHEMA by the callers).
func splitQualifiedTableName(name string) (schema string, table string, err error) {
	parts, err := parseIdentParts(name)
	if err != nil {
		return "", "", err
	}
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}
	return "", parts[0], nil
}

// validateTableName validates a configured table name at registration
// (audit P5-3/P5-12): empty names are rejected and non-subquery names
// must parse as (quoted) schema.table parts.
func validateTableName(name string) error {
	if err := validateIdentName(name); err != nil {
		return err
	}
	if strings.Contains(name, " ") {
		// subquery form (see quoteTableName), passed through verbatim
		return nil
	}
	if _, err := parseIdentParts(name); err != nil {
		return err
	}
	return nil
}

// quoteTableName quotes a possibly qualified HANA table name (audit
// P5-3). Names containing whitespace are treated as subqueries (e.g.
// "(SELECT * FROM tbl) x") and passed through verbatim. Other names
// are split with the quote-aware parseIdentParts (so dots inside
// quoted identifiers are not treated as separators) and each part is
// quoted; on a parse error the whole name is quoted as a single
// identifier, which is safe but -- unlike the old strings.Split -- can
// never split a hostile name into injection-shaped parts.
func quoteTableName(name string) string {
	if strings.Contains(name, " ") {
		return name
	}

	parts, err := parseIdentParts(name)
	if err != nil {
		return quoteIdentifier(name)
	}

	nstrs := len(parts)
	if nstrs == 1 {
		return quoteIdentifier(parts[0])
	}

	ret := ""
	for i, s := range parts {
		ret = ret + quoteIdentifier(s)
		if i != nstrs-1 {
			ret = ret + "."
		}
	}

	return ret
}

func hasSrsPlanarEquivalent(pool *connectionPoolCollector, srid uint64) (bool, error) {
	var numSRIDs int = 0
	sql := "SELECT COUNT(*) FROM SYS.ST_SPATIAL_REFERENCE_SYSTEMS WHERE SRS_ID = ?"
	ctx, cancel := NewInspectionContext(context.Background())
	defer cancel()
	if err := pool.QueryRowContext(ctx, sql, toPlanarEquivalenSrid(srid)).Scan(&numSRIDs); err != nil {
		return false, fmt.Errorf("planar equivalent lookup for srid %v failed: %w", srid, err)
	}
	return numSRIDs > 0, nil
}

// isSrsRoundEarth reports whether the SRS is round-earth. The "?" form
// is the placeholder go-hdb's scanner recognizes (audit N11); lookup
// errors are returned so registration cannot silently treat an unknown
// SRS as planar.
func isSrsRoundEarth(pool *connectionPoolCollector, srid uint64) (bool, error) {
	if srid == tegola.WGS84 {
		return true, nil
	}

	sql := "SELECT TO_BOOLEAN(ROUND_EARTH) FROM SYS.ST_SPATIAL_REFERENCE_SYSTEMS WHERE SRS_ID = ?"
	var ret bool = false
	ctx, cancel := NewInspectionContext(context.Background())
	defer cancel()
	if err := pool.QueryRowContext(ctx, sql, srid).Scan(&ret); err != nil {
		return false, fmt.Errorf("round-earth lookup for srid %v failed: %w", srid, err)
	}
	return ret, nil
}

func genGeomField(name string, providerType string) string {
	return fmt.Sprintf(`%v.ST_AsBinary()  AS %[1]v`, quoteIdentifier(name))
}

// genRawGeomField selects a raw geometry column verbatim: MOS blobs and
// plain WKB/WKT values are not HANA ST_Geometry values and native spatial
// functions cannot be applied to them.
func genRawGeomField(name string) string {
	return fmt.Sprintf(`%v AS %[1]v`, quoteIdentifier(name))
}

func getLayerSQL(tblname string) string {
	quotedTblName := quoteTableName(tblname)
	return fmt.Sprintf(`SELECT * FROM %[1]v LIMIT 0;`, quotedTblName)
}

// getLayerRows runs a registration-time probe query under the supplied
// context. Callers create the context via NewInspectionContext (audit
// P5-16) and own its cancelation: the returned rows stay live past this
// function, so no cancel may run before the caller closes them.
func getLayerRows(ctx context.Context, pool *connectionPoolCollector, sql string, extent *geom.Extent, srid uint64, withBBox bool) (*sql.Rows, error) {
	if withBBox {
		rows, err := pool.QueryContextWithBBox(ctx, sql, extent, srid, false)
		if err := ctxErr(ctx, err); err != nil {
			return nil, err
		}
		return rows, nil
	} else {
		rows, err := pool.QueryContext(ctx, sql)
		if err := ctxErr(ctx, err); err != nil {
			return nil, err
		}
		return rows, nil
	}
}

func getLayerFields(pool *connectionPoolCollector, l *Layer, sql string) ([]FieldDescription, error) {
	withBBox := strings.Contains(sql, bboxToken) && !codec.IsRawFormat(l.geometryFormat)

	//	if a subquery is set in the 'sql' config the subquery is set to the layer's
	//	'tablename' param. because of this case normal SQL token replacement needs to be
	//	applied to tablename SQL generation
	tile := provider.NewTile(18, 0, 0, 64, tegola.WebMercator)
	sql, err := replaceTokens(2, sql, l, l.GeomType(), l.SRID(), tile, false)
	if err != nil {
		return nil, err
	}

	extent, _, err := getTileExtent(tile, false)
	if err != nil {
		return nil, err
	}
	// audit P5-16: the metadata probe runs under the inspection timeout;
	// the deferred cancel stays live until this function's rows are closed.
	ctx, cancel := NewInspectionContext(context.Background())
	defer cancel()
	rows, err := getLayerRows(ctx, pool, sql, extent, l.SRID(), withBBox)
	if err != nil {
		return nil, err
	}
	// audit N10: the metadata probe never iterates the rows but must
	// still release them (sqlclosecheck/rowserrcheck clean).
	defer func() { _ = rows.Close() }()

	columns, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return getFieldDescriptions(l.Name(), l.GeomFieldName(), l.IDFieldName(), columns, l.geometryFormat == "")
}

func getFieldNames(fields []FieldDescription) []string {
	var fieldNames []string
	for i := range fields {
		fieldNames = append(fieldNames, fields[i].name)
	}
	return fieldNames
}

func getTableFieldNames(pool *connectionPoolCollector, l *Layer, tblName string) ([]string, error) {
	fields, err := getLayerFields(pool, l, getLayerSQL(tblName))
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("no fields were returned for table %v", tblName)
	}

	return getFieldNames(fields), nil
}

func genSQL(l *Layer, tblName string, fieldNames []string, buffer bool, providerType string) (sql string, err error) {
	fgeom := -1

	for i, f := range fieldNames {
		if f == l.geomField {
			fgeom = i
		}
		fieldNames[i] = quoteIdentifier(fieldNames[i])
	}

	if fgeom == -1 {
		if codec.IsRawFormat(l.geometryFormat) {
			fieldNames = append(fieldNames, genRawGeomField(l.geomField))
		} else {
			fieldNames = append(fieldNames, genGeomField(l.geomField, providerType))
		}
	} else {
		if codec.IsRawFormat(l.geometryFormat) {
			fieldNames[fgeom] = genRawGeomField(l.geomField)
		} else {
			fieldNames[fgeom] = genGeomField(l.geomField, providerType)
		}
	}

	// The id field is always appended to the SELECT list, even when the
	// configured fields already contain it: the first occurrence feeds the
	// feature id and later occurrences fall through into the tags map, so
	// a configured id field is both the feature id and a tag. This mirrors
	// the postgis provider contract ("the id has to be parsed once but it
	// can also be a tag").
	if l.idField != "" {
		fieldNames = append(fieldNames, quoteIdentifier(l.idField))
	}

	stdSQL := `SELECT %[1]v FROM %[2]v WHERE ` + bboxToken

	if codec.IsRawFormat(l.geometryFormat) {
		// Raw geometry columns (MOS blobs, plain WKB/WKT) are not HANA
		// ST_Geometry values: native spatial predicates
		// (ST_IntersectsRect, ...) cannot be applied to them. Filtering is
		// done in memory after decoding (see TileFeatures).
		stdSQL = `SELECT %[1]v FROM %[2]v WHERE ` + quoteIdentifier(l.geomField) + ` IS NOT NULL`
	}

	return fmt.Sprintf(stdSQL, strings.Join(fieldNames, ", "), quoteTableName(tblName)), nil
}

func genMVTSQL(l *Layer, fields []string, buffer uint, clipGeometry bool) (sql string, err error) {
	var flds []string
	for i := range fields {
		if l.GeomFieldName() != fields[i] {
			flds = append(flds, quoteIdentifier(fields[i]))
		}
	}

	geomFieldName := quoteIdentifier(l.GeomFieldName())

	// ref: https://help.sap.com/docs/HANA_CLOUD_DATABASE/bc9e455fe75541b8a248b4c09b086cf5/8cd683c4bb664fd8a71fc3f19ffa7e42.html
	// BLOB  ST_AsMVT(expression_list Expression List, layer_name NCLOB, extent INT, geom_name NCLOB, feature_id_name NCLOB)

	var clip string = "TRUE"
	if !clipGeometry {
		clip = "FALSE"
	}

	if len(flds) == 0 {
		sql = fmt.Sprintf(`SELECT ST_AsMVT(%v.ST_AsMVTGeom(bounds => NEW ST_LINESTRING($4, $3), buffer => %v, clipgeom => %v) AS %v, layer_name => %v, geom_name => %v) FROM (%v)`, geomFieldName, buffer, clip, geomFieldName, sqlStringLiteral(l.Name()), sqlStringLiteral(l.GeomFieldName()), l.sql)
	} else {
		if l.IDFieldName() != "" {
			sql = fmt.Sprintf(`SELECT ST_AsMVT(%v, %v.ST_AsMVTGeom(bounds => NEW ST_LINESTRING($4, $3), buffer => %v, clipgeom => %v) AS %v, layer_name => %v, geom_name => %v, feature_id_name => %v) FROM (%v)`, strings.Join(flds, ","), geomFieldName, buffer, clip, geomFieldName, sqlStringLiteral(l.Name()), sqlStringLiteral(l.GeomFieldName()), sqlStringLiteral(l.IDFieldName()), l.sql)
		} else {
			sql = fmt.Sprintf(`SELECT ST_AsMVT(%v, %v.ST_AsMVTGeom(bounds => NEW ST_LINESTRING($4, $3), buffer => %v, clipgeom => %v) AS %v, layer_name => %v, geom_name => %v) FROM (%v)`, strings.Join(flds, ","), geomFieldName, buffer, clip, geomFieldName, sqlStringLiteral(l.Name()), sqlStringLiteral(l.GeomFieldName()), l.sql)
		}
	}
	return sql, nil
}

const (
	PLANAR_SRID_OFFSET = 1000000000
)

func isPlanarEquivalentSrid(srid uint64) bool {
	return srid >= PLANAR_SRID_OFFSET
}

// isSyntheticCRS reports whether srid is a Tegola-only synthetic SRID
// (crs_defn) rather than a planar-equivalent round-earth SRS or a real
// database SRS. The planar-equivalent offset (1e9) sits inside the numeric
// synthetic range, so it must be excluded explicitly.
func isSyntheticCRS(srid uint64) bool {
	return basic.IsSyntheticSRID(srid) && !isPlanarEquivalentSrid(srid)
}

// validateCRSFormatCompatibility checks that a layer on a synthetic SRID
// (registered from crs_defn, unknown to the database) uses a raw geometry
// format: spatial predicates degrade to 1=1 (see getBBoxFilter), so filtering
// happens client-side on a decodable geometry. MVT passthrough cannot provide
// that. Native HANA ST_Geometry columns require a database-side SRS and are
// rejected as well. Unit-testable without a database connection.
func validateCRSFormatCompatibility(srid int, providerType string, geometryFormat string) error {
	if !isSyntheticCRS(uint64(srid)) {
		return nil
	}
	if providerType == MVTProviderType {
		return fmt.Errorf(
			"%v (synthetic SRID) is not supported for MVT providers",
			crsconfig.KeyCRSDefn,
		)
	}
	if geometryFormat == "" {
		return fmt.Errorf(
			"%v (synthetic SRID) requires %v = %q, %q, or %q; native HANA ST_Geometry columns use a database-side SRS",
			crsconfig.KeyCRSDefn, codec.ConfigKeyGeometryFormat,
			codec.FormatWKB, codec.FormatWKT, codec.FormatMOS,
		)
	}
	return nil
}

func toPlanarEquivalenSrid(srid uint64) uint64 {
	return PLANAR_SRID_OFFSET + srid
}

// transformSRID maps a layer SRID to the SRID that client-side coordinate
// transforms must use. The planar-equivalent offset only encodes the
// round-earth SRID of a HANA geometry column so the server-side predicates can
// run planar; the stored coordinates are unchanged and registered CRS
// transforms only know the effective (base) SRID. Synthetic CRS SRIDs pass
// through untouched.
func transformSRID(srid uint64) uint64 {
	if isPlanarEquivalentSrid(srid) {
		return srid - PLANAR_SRID_OFFSET
	}
	return srid
}

// tileBBoxInLayerCRS returns the buffered tile extent (WebMercator)
// transformed into the layer's source CRS for the exact in-memory geometry
// filter used with raw geometry formats. A planar-equivalent layer SRID is
// normalized to its effective SRID first (transformSRID).
func tileBBoxInLayerCRS(tile provider.Tile, layerSRID uint64) (*geom.Extent, error) {
	bbox, _ := tile.BufferedExtent()
	srid := transformSRID(layerSRID)
	if srid == tegola.WebMercator {
		return bbox, nil
	}
	return basic.FromWebMercatorExtent(srid, bbox)
}

// getBBoxCoordinates returns the lower-left and upper-right corners of the
// axis-aligned bounding box enclosing the tile extent after transformation
// into the target CRS (audit P5-15). The tile's full perimeter is sampled
// and transformed and min/max is taken: converting only the two opposite
// corners under-covers the tile footprint in rotated or polar projections
// and the bbox predicate then drops valid features.
func getBBoxCoordinates(extent *geom.Extent, srid uint64) (geom.Point, geom.Point, error) {
	// The planar-equivalent offset only encodes the round-earth SRID;
	// transform with the effective SRID.
	effective := transformSRID(srid)

	sourceExtent, err := basic.FromWebMercatorExtent(effective, extent)
	if err != nil {
		return geom.Point{}, geom.Point{}, fmt.Errorf("Error trying to convert tile point: %w ", err)
	}

	return geom.Point{sourceExtent.MinX(), sourceExtent.MinY()}, geom.Point{sourceExtent.MaxX(), sourceExtent.MaxY()}, nil
}

func getBBoxFilter(dbVersion uint, geomField string, srid uint64) string {
	// Synthetic SRIDs (crs_defn) are not known to the database: degrade the
	// spatial predicate to a no-op; filtering happens client-side.
	if isSyntheticCRS(srid) {
		return "1=1"
	}
	if dbVersion == 1 {
		if isPlanarEquivalentSrid(srid) {
			return fmt.Sprintf("%v.ST_SRID($3).ST_IntersectsRect(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1", quoteIdentifier(geomField))
		} else {
			return fmt.Sprintf("%v.ST_IntersectsRect(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1", quoteIdentifier(geomField))
		}
	} else {
		if isPlanarEquivalentSrid(srid) {
			return fmt.Sprintf("%v.ST_SRID($3).ST_IntersectsRectPlanar(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1", quoteIdentifier(geomField))
		} else {
			return fmt.Sprintf("%v.ST_IntersectsRectPlanar(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1", quoteIdentifier(geomField))
		}
	}
}

func getGeometryColumnSRID(pool *connectionPoolCollector, dbVersion uint, sql string, geomFieldName string) (srid int, err error) {
	// Shared bounds-contract probe preparation (audit R1): neutralizes
	// !BBOX!/!BOX! to "1=1" and expands zoom/position placeholders
	// permissively so the sample cannot be filtered out by tile tokens.
	sqlQuery := codec.HANA.PrepareProbeSQL(sql, geomFieldName, "", "")

	sqlQuery = fmt.Sprintf("SELECT %[1]v.ST_SRID() FROM %[2]v WHERE %[1]v IS NOT NULL LIMIT 1", quoteIdentifier(geomFieldName), sqlQuery)
	ctx, cancel := NewInspectionContext(context.Background())
	defer cancel()
	err = pool.QueryRowContext(ctx, sqlQuery).Scan(&srid)
	return srid, err
}

// getTileExtent returns the tile extent (buffered and clamped to the
// WebMercator range when withBuffer is set) together with its SRID.
// provider.Tile's Extent()/BufferedExtent() accessors cannot report
// errors, so a nil or non-finite extent is rejected here with a clear
// error instead of flowing into the query builder and panicking or
// silently producing an invalid bbox (audit P6-20).
func getTileExtent(tile provider.Tile, withBuffer bool) (*geom.Extent, uint64, error) {
	if withBuffer {
		extent, srid := tile.BufferedExtent()
		if err := validateTileExtent(extent); err != nil {
			return nil, 0, err
		}

		minx := math.Max(-20037508.3427892, extent[0])
		miny := math.Max(-20037508.3427892, extent[1])
		maxx := math.Min(20037508.3427892, extent[2])
		maxy := math.Min(20037508.3427892, extent[3])

		return geom.NewExtent([2]float64{minx, miny}, [2]float64{maxx, maxy}), srid, nil
	}

	extent, srid := tile.Extent()
	if err := validateTileExtent(extent); err != nil {
		return nil, 0, err
	}
	return extent, srid, nil
}

// validateTileExtent rejects nil and non-finite tile extents with a clear
// error (audit P6-20).
func validateTileExtent(extent *geom.Extent) error {
	if extent == nil {
		return fmt.Errorf("tile extent is nil")
	}
	for i, v := range [4]float64{extent[0], extent[1], extent[2], extent[3]} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("tile extent coordinate %v is not finite (%v)", i, v)
		}
	}
	return nil
}

func sanitizeSQL(sql string) string {
	// convert !BOX! (MapServer) and !bbox! (Mapnik) to !BBOX! for
	// compatibility; only in SQL code context — token-looking text in
	// string literals, identifiers, or comments stays verbatim.
	return sqltoken.HANA.MapTokens(sql, func(tok string) string {
		if tok == "!BOX!" || tok == "!bbox!" {
			return bboxToken
		}
		return tok
	})
}

// replaceTokens replaces tokens in the provided SQL string
//
// !ZOOM! - the tile Z value
// !X! - the tile X value
// !Y! - the tile Y value
// !Z! - the tile Z value
// !SCALE_DENOMINATOR! - horizontal scale using the OGC 0.28mm rendering pixel
// !PIXEL_WIDTH! - unbuffered pixel width in the resolved layer CRS units
// !PIXEL_HEIGHT! - unbuffered pixel height in the resolved layer CRS units
// !GEOM_FIELD! - the geom field name
// !GEOM_TYPE! - the geom field type if defined otherwise ""
//
// Only tokens in SQL code context are substituted; token-looking text
// inside string literals, quoted identifiers, comments, or dollar-quoted
// strings is left verbatim.
func replaceTokens(dbVersion uint, sql string, l *Layer, geomFieldType geom.Geometry, srid uint64, tile provider.Tile, withBuffer bool) (string, error) {
	var (
		geoType string
	)

	_, _, terr := getTileExtent(tile, false)
	if terr != nil {
		return "", terr
	}
	var pixelWidth, pixelHeight, scaleDenominator float64
	if sqltoken.HANA.ContainsTokenFold(sql, pixelWidthToken, pixelHeightToken, scaleDenominatorToken) {
		var err error
		pixelWidth, pixelHeight, scaleDenominator, err = tileScale(tile, srid)
		if err != nil {
			return "", fmt.Errorf("layer (%v) scale tokens: %w", l.name, err)
		}
	}

	if geomFieldType != nil {
		geoType = fmt.Sprintf("%v", geomFieldType)
	}

	bboxFilter := getBBoxFilter(dbVersion, l.geomField, srid)
	// Raw (MOS) layers cannot use the native spatial predicate: their
	// !BBOX! token expands to the bounds predicate over the configured
	// bounds fields with MOS raw scaling instead. Custom SQL for MOS is
	// required to carry the token (RequireBBoxCustomSQL).
	if l.geometryFormat == codec.FormatMOS {
		bboxExtent, _, berr := getTileExtent(tile, withBuffer)
		if berr != nil {
			return "", berr
		}
		// A planar-equivalent SRID only labels the round-earth SRID of a
		// HANA geometry column and must be normalized before the
		// conversion into the layer's source CRS.
		sourceExtent, cerr := basic.FromWebMercatorExtent(transformSRID(srid), bboxExtent)
		if cerr != nil {
			return "", fmt.Errorf("error converting tile extent: %w", cerr)
		}
		predicate, perr := codec.BuildBoundsPredicate(
			l.bboxFields, sourceExtent, codec.BoundsMOSRaw, l.mosConfig,
			codec.BoundsQuote(l.bboxTable, quoteIdentifier),
		)
		if perr != nil {
			return "", fmt.Errorf("layer (%v): %w", l.name, perr)
		}
		bboxFilter = predicate
	}

	// replace query string tokens
	z, x, y := tile.ZXY()
	sql = uppercaseTokens(sql)
	return sqltoken.HANA.MapTokens(sql, func(tok string) string {
		switch tok {
		case bboxToken:
			return bboxFilter
		case zoomToken, zToken:
			return strconv.FormatUint(uint64(z), 10)
		case xToken:
			return strconv.FormatUint(uint64(x), 10)
		case yToken:
			return strconv.FormatUint(uint64(y), 10)
		case idFieldToken:
			return quoteTokenIdentifier(l.IDFieldName())
		case geomFieldToken:
			return quoteTokenIdentifier(l.geomField)
		case geomTypeToken:
			return geoType
		case scaleDenominatorToken:
			return strconv.FormatFloat(scaleDenominator, 'f', 8, 64)
		case pixelWidthToken:
			return strconv.FormatFloat(pixelWidth, 'f', 8, 64)
		case pixelHeightToken:
			return strconv.FormatFloat(pixelHeight, 'f', 8, 64)
		}
		return tok
	}), nil
}

func getFieldDescriptions(layerName, geomFieldname, idFieldname string, columns []*sql.ColumnType, checkFieldType bool) ([]FieldDescription, error) {
	list := make([]FieldDescription, 0, len(columns))

	var geomFieldFound bool
	var idFieldFound bool
	for i := range columns {
		column := columns[i]
		fieldName := column.Name()

		isIdField := false
		isGeometryField := false

		var dataType DataType
		switch column.DatabaseTypeName() {
		case "BOOLEAN":
			dataType = DtBoolean
		case "TINYINT":
			dataType = DtTinyint
		case "SMALLINT":
			dataType = DtSmallint
		case "INTEGER":
			dataType = DtInteger
		case "BIGINT":
			dataType = DtBigint
		case "DECIMAL", "FIXED8", "FIXED12", "FIXED16":
			precision, _, _ := column.DecimalSize()
			if precision <= 16 {
				dataType = DtSmalldecimal
			} else {
				dataType = DtDecimal
			}
		case "SMALLDECIMAL":
			dataType = DtSmalldecimal
		case "REAL":
			dataType = DtReal
		case "DOUBLE":
			dataType = DtDouble
		case "CHAR":
			dataType = DtChar
		case "VARCHAR":
			dataType = DtVarchar
		case "NCHAR":
			dataType = DtNChar
		case "NVARCHAR":
			dataType = DtNVarchar
		case "SHORTTEXT":
			dataType = DtShorttext
		case "ALPHANUM":
			dataType = DtAlphanum
		case "BINARY":
			dataType = DtBinary
		case "VARBINARY":
			dataType = DtVarbinary
		case "DATE", "DAYDATE":
			dataType = DtDate
		case "TIME", "SECONDTIME":
			dataType = DtTime
		case "TIMESTAMP", "LONGDATE":
			dataType = DtTimestamp
		case "SECONDDATE":
			dataType = DtSeconddate
		case "BLOB":
			dataType = DtBlob
		case "CLOB":
			dataType = DtClob
		case "NCLOB":
			dataType = DtNClob
		case "TEXT":
			dataType = DtText
		case "STGEOMETRY":
			dataType = DtSTGeometry
		case "STPOINT":
			dataType = DtSTPoint
		default:
			dataType = DtUnknown
		}

		if !geomFieldFound && fieldName == geomFieldname {
			if !checkFieldType || (dataType == DtSTGeometry || dataType == DtSTPoint || dataType == DtBlob) {
				geomFieldFound = true
				isGeometryField = true
			}
		} else if !idFieldFound && fieldName == idFieldname {
			if !checkFieldType || (dataType == DtTinyint || dataType == DtSmallint || dataType == DtInteger || dataType == DtBigint) {
				idFieldFound = true
				isIdField = true
			}
		}

		list = append(list, FieldDescription{
			dataType:    dataType,
			name:        fieldName,
			isFeatureId: isIdField,
			isGeometry:  isGeometryField})
	}

	if !geomFieldFound && geomFieldname != "" {
		return nil, ErrGeomFieldNotFound{
			GeomFieldName: geomFieldname,
			LayerName:     layerName,
		}
	}

	return list, nil
}

func setupRowValues(descriptions []FieldDescription, rowValues []interface{}) {
	for i := range rowValues {
		switch descriptions[i].dataType {
		case DtBoolean:
			rowValues[i] = new(sql.NullBool)
		case DtTinyint:
			rowValues[i] = new(sql.NullByte)
		case DtSmallint:
			rowValues[i] = new(sql.NullInt16)
		case DtInteger:
			rowValues[i] = new(sql.NullInt32)
		case DtBigint:
			rowValues[i] = new(sql.NullInt64)
		case DtDecimal, DtSmalldecimal:
			rowValues[i] = &driver.NullDecimal{Decimal: new(driver.Decimal)}
		case DtReal, DtDouble:
			rowValues[i] = new(sql.NullFloat64)
		case DtDate, DtTime, DtTimestamp, DtSeconddate:
			rowValues[i] = new(sql.NullTime)
		case DtBinary, DtVarbinary:
			rowValues[i] = new(driver.NullBytes)
		case DtBlob, DtClob, DtNClob, DtText:
			rowValues[i] = &driver.NullLob{Lob: new(driver.Lob).SetWriter(new(bytes.Buffer))}
		case DtChar, DtNChar, DtNVarchar, DtVarchar, DtShorttext, DtAlphanum:
			rowValues[i] = new(sql.NullString)
		case DtSTGeometry, DtSTPoint:
			rowValues[i] = new(sql.NullString)
		default:
			rowValues[i] = new(interface{})
		}
	}
}

// probeRawValues unwraps typed scan targets created by setupRowValues into
// raw row values so the shared SQL-contract probe inspects REAL values.
func probeRawValues(rowValues []interface{}) []interface{} {
	vals := make([]interface{}, len(rowValues))
	for i, v := range rowValues {
		vals[i] = probeRawValue(v)
	}
	return vals
}

func probeRawValue(v interface{}) interface{} {
	switch val := v.(type) {
	case *sql.NullBool:
		if val.Valid {
			return val.Bool
		}
	case *sql.NullByte:
		if val.Valid {
			return val.Byte
		}
	case *sql.NullInt16:
		if val.Valid {
			return val.Int16
		}
	case *sql.NullInt32:
		if val.Valid {
			return val.Int32
		}
	case *sql.NullInt64:
		if val.Valid {
			return val.Int64
		}
	case *sql.NullFloat64:
		if val.Valid {
			return val.Float64
		}
	case *sql.NullTime:
		if val.Valid {
			return val.Time
		}
	case *sql.NullString:
		if val.Valid {
			return val.String
		}
	case *driver.NullBytes:
		if val.Valid {
			return append([]byte(nil), val.Bytes...)
		}
	case *driver.NullDecimal:
		if val.Valid {
			r := (*big.Rat)(val.Decimal)
			f, _ := r.Float64()
			return f
		}
	case *driver.NullLob:
		if val.Valid {
			if w, ok := val.Lob.Writer().(*bytes.Buffer); ok {
				return append([]byte(nil), w.Bytes()...)
			}
		}
	case *interface{}:
		return probeRawValue(*val)
	}
	// plain driver values (int64, float64, bool, string, []byte, ...)
	// pass through unchanged for the geometry contract probe
	return v
}

func readRowValues(ctx context.Context, l *Layer, descriptions []FieldDescription, rowValues []interface{}) (gid uint64, geom []byte, tags map[string]interface{}, err error) {
	var idFieldParsed bool
	tags = make(map[string]interface{})

	for i := range rowValues {
		// do a quick check
		if err := ctx.Err(); err != nil {
			return 0, nil, nil, err
		}

		// skip nil values.
		if rowValues[i] == nil {
			continue
		}

		desc := descriptions[i]
		fieldName := desc.name

		// Bounds fields backing the bounds-backed !BBOX! predicate are
		// operational columns, not user attributes: never leak them into
		// feature tags.
		if l != nil && l.bboxFields.IsBBoxField(fieldName) {
			continue
		}

		// Feature id column (audit P6-10/P6-11): the id is extracted for
		// every storage type. Numeric id columns previously fell through
		// into the tags map and every feature got duplicate ID 0. A NULL
		// id skips the row with a warning (the same strategy the gpkg
		// provider uses): NULL ids must never silently become duplicate
		// ID 0 features.
		if desc.isFeatureId && !idFieldParsed {
			idVal, ok := featureIDValue(rowValues[i])
			if !ok {
				layerName := ""
				if l != nil {
					layerName = l.name
				}
				log.Warnf("skipping feature with NULL id field '%v' in layer '%v' (audit P6-11)", fieldName, layerName)
				return 0, nil, nil, nil
			}
			gid, err = convertToUInt64(idVal)
			if err != nil {
				return 0, nil, nil, fmt.Errorf("feature id field '%v': %w", fieldName, err)
			}
			idFieldParsed = true
			continue
		}

		switch desc.dataType {
		case DtBoolean:
			boolValue := *(rowValues[i].(*sql.NullBool))
			if boolValue.Valid {
				tags[fieldName] = boolValue.Bool
			}
		case DtTinyint:
			byteValue := *(rowValues[i].(*sql.NullByte))
			if byteValue.Valid {
				tags[fieldName] = byteValue.Byte
			}
		case DtSmallint:
			int16Value := *(rowValues[i].(*sql.NullInt16))
			if int16Value.Valid {
				tags[fieldName] = int16Value.Int16
			}
		case DtInteger:
			int32Value := *(rowValues[i].(*sql.NullInt32))
			if int32Value.Valid {
				tags[fieldName] = int32Value.Int32
			}
		case DtBigint:
			int64Value := *(rowValues[i].(*sql.NullInt64))
			if int64Value.Valid {
				tags[fieldName] = int64Value.Int64
			}
		case DtDecimal:
			decimalValue := *(rowValues[i].(*driver.NullDecimal))
			if decimalValue.Valid {
				r := (*big.Rat)(decimalValue.Decimal)
				f, _ := r.Float64()
				tags[fieldName] = f
			}
		case DtSmalldecimal:
			decimalValue := *(rowValues[i].(*driver.NullDecimal))
			if decimalValue.Valid {
				r := (*big.Rat)(decimalValue.Decimal)
				f, _ := r.Float32()
				tags[fieldName] = f
			}
		case DtReal, DtDouble:
			float64Value := *(rowValues[i].(*sql.NullFloat64))
			if float64Value.Valid {
				if desc.dataType == DtReal {
					tags[fieldName] = float32(float64Value.Float64)
				} else {
					tags[fieldName] = float64Value.Float64
				}
			}
		case DtDate, DtTime, DtTimestamp, DtSeconddate:
			timeValue := *(rowValues[i].(*sql.NullTime))
			if timeValue.Valid {
				switch desc.dataType {
				case DtDate:
					tags[fieldName] = timeValue.Time.Format("2006-01-02")
				case DtTime:
					tags[fieldName] = timeValue.Time.Format("15:04:05")
				case DtTimestamp:
					tags[fieldName] = timeValue.Time.Format("2006-01-02T15:04:05.000")
				case DtSeconddate:
					tags[fieldName] = timeValue.Time.Format("2006-01-02T15:04:05")
				}
			}
		case DtNVarchar, DtVarchar, DtShorttext, DtAlphanum, DtChar, DtNChar:
			strValue := *(rowValues[i].(*sql.NullString))
			if strValue.Valid {
				tags[fieldName] = strValue.String
			}
		case DtBinary, DtVarbinary:
			binValue := *(rowValues[i].(*driver.NullBytes))
			if binValue.Valid {
				tags[fieldName] = hex.EncodeToString(binValue.Bytes[:])
			}
		case DtBlob, DtClob, DtNClob, DtText:
			lobValue := *(rowValues[i].(*driver.NullLob))
			if lobValue.Valid {
				writer := lobValue.Lob.Writer()
				data := writer.(*bytes.Buffer).Bytes()
				dataLen := writer.(*bytes.Buffer).Len()
				if dataLen > 0 {
					if desc.isGeometry {
						geom = make([]byte, dataLen)
						copy(geom, data)
					} else {
						if desc.dataType == DtBlob {
							tags[fieldName] = hex.EncodeToString(data[0:dataLen])
						} else {
							tags[fieldName] = string(data[0:dataLen])
						}
					}
				}
			}
		case DtSTGeometry, DtSTPoint:
			strValue := *(rowValues[i].(*sql.NullString))
			if strValue.Valid {
				if desc.isGeometry {
					geom, err = hex.DecodeString(strValue.String)
					if err != nil {
						return 0, nil, nil, fmt.Errorf("unable to decode geometry binary string in field '%v'", fieldName)
					}
				} else {
					tags[fieldName] = strValue.String
				}
			}
		default:
			return 0, nil, nil, fmt.Errorf("data type is unsupported in field '%v'", fieldName)
		}
	}

	return gid, geom, tags, nil
}

// featureIDValue unwraps a typed row scan target (see setupRowValues)
// into its underlying feature id value (audit P6-10/P6-11). The second
// result is false for NULL values, which must never silently become
// duplicate ID 0 features. Unlike probeRawValue it reports NULL-ness
// explicitly instead of passing the scan-target pointer through.
func featureIDValue(v interface{}) (interface{}, bool) {
	switch val := v.(type) {
	case *sql.NullString:
		return val.String, val.Valid
	case *sql.NullInt64:
		return val.Int64, val.Valid
	case *sql.NullInt32:
		return val.Int32, val.Valid
	case *sql.NullInt16:
		return val.Int16, val.Valid
	case *sql.NullByte:
		return val.Byte, val.Valid
	case *sql.NullFloat64:
		return val.Float64, val.Valid
	case *driver.NullDecimal:
		if !val.Valid {
			return nil, false
		}
		r := (*big.Rat)(val.Decimal)
		f, _ := r.Float64()
		return f, true
	case *driver.NullBytes:
		if !val.Valid {
			return nil, false
		}
		return append([]byte(nil), val.Bytes...), true
	case *driver.NullLob:
		if !val.Valid {
			return nil, false
		}
		if w, ok := val.Lob.Writer().(*bytes.Buffer); ok {
			return append([]byte(nil), w.Bytes()...), true
		}
		return nil, false
	case *interface{}:
		return featureIDValue(*val)
	}
	return v, v != nil
}

// convertToUInt64 converts a feature id value to uint64 by delegating to
// the shared provider.ConvertFeatureID (audit P6-10 cross-provider ID
// contract): every provider converts id types identically and the shared
// validation policy lives in one place (provider/feature.go). The
// sql.NullString and int16 shapes are unwrapped/widened first because
// ConvertFeatureID does not handle those two scan-target shapes.
func convertToUInt64(v interface{}) (intv uint64, err error) {
	switch aval := v.(type) {
	case sql.NullString:
		if !aval.Valid {
			return 0, fmt.Errorf("unable to convert field into a uint64: feature id is NULL")
		}
		v = aval.String
	case int16:
		v = int64(aval)
	}
	gid, cerr := provider.ConvertFeatureID(v)
	if cerr != nil {
		return 0, fmt.Errorf("unable to convert field into a uint64: %w", cerr)
	}
	return gid, nil
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
	return sqltoken.HANA.MapTokens(str, strings.ToUpper)
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
