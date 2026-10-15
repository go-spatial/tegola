package gpkg

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/internal/sqltoken"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// sqliteQuoteIdent quotes a SQLite identifier, escaping embedded backticks
// so a crafted config value cannot break out of the quoted name. Defined
// separately from quoteIdent in gpkg.go because util.go is built without
// the cgo tag while gpkg.go is not.
func sqliteQuoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// sqliteReadOnlyDSN builds the sqlite3 DSN used to open GeoPackage files.
// The provider never writes to a GeoPackage, so the database is opened in
// read-only mode (audit P6-16); _busy_timeout keeps reads patient against a
// file locked by another process.
func sqliteReadOnlyDSN(path string) string {
	return "file:" + path + "?mode=ro&_busy_timeout=5000"
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

// isQuotedIdentifierValue reports whether v is one complete quoted
// identifier spanning the whole value (audit P5-10 pass-through rule):
// first and last byte are the same identifier-quote character and the
// closing quote at the end is the actual terminator (doubled quote
// characters inside count as escapes). Only the identifier quote
// characters (double quote, backtick) qualify; a single-quote pair is a
// SQL string literal, never an identifier, so it is never passed through.
// A value like "`x';DROP`" is NOT a complete pair and is quoted normally
// instead of being passed through as SQL text.
func isQuotedIdentifierValue(v string) bool {
	if len(v) < 2 {
		return false
	}
	q := v[0]
	if q != '"' && q != '`' {
		return false
	}
	if v[len(v)-1] != q {
		return false
	}
	for i := 1; i < len(v); i++ {
		if v[i] == q {
			if i+1 < len(v) && v[i+1] == q {
				i++ // doubled quote escape
				continue
			}
			return i == len(v)-1
		}
	}
	return false
}

// splitIdentDots splits a possibly qualified identifier at top-level dots,
// keeping dots that sit inside quoted segments ("a.b".c has one dot). An
// unclosed quote keeps the remainder in the current part, so hostile values
// are never split into injection-shaped pieces.
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

// quoteTokenIdentifier prepares a configured !ID_FIELD!/!GEOM_FIELD!
// token value for interpolation into SQLite SQL text (audit P5-10,
// shorthand "!ID!/!GEOM!"). A value already wrapped in one complete
// identifier quote pair passes through verbatim; anything else is quoted
// per identifier part (schema.table.col => `schema`.`table`.`col`) with
// embedded quote characters made inert.
func quoteTokenIdentifier(v string) string {
	if isQuotedIdentifierValue(v) {
		return v
	}
	parts := splitIdentDots(v)
	for i := range parts {
		if isQuotedIdentifierValue(parts[i]) {
			continue
		}
		parts[i] = sqliteQuoteIdent(parts[i])
	}
	return strings.Join(parts, ".")
}

// replaceTokens replaces tile and layer metadata tokens in a SQL query.
// Only tokens in SQL code context are substituted; token-looking text
// inside string literals, quoted identifiers, comments, or dollar-quoted
// strings is left verbatim.
//
// bboxExtent must be the tile's buffered extent transformed to the layer's
// source SRID. Pixel dimensions use the unbuffered extent in the source CRS;
// scale uses its linear units or a latitude-adjusted geographic conversion.
//
// Bounds-backed custom SQL (!BBOX! expanding into the configured bounds
// fields predicate) is fail-closed (A12): a predicate build error is
// returned to the caller instead of silently substituting "1=1". The
// predicate is built lazily — only when the query actually carries the
// !BBOX!/!BOX! token (in code context).
func replaceTokens(qtext string, layer *Layer, tile provider.Tile, bboxExtent *geom.Extent) (string, error) {
	qtext = uppercaseTokens(qtext)

	// only build the bounds predicate when the query actually uses it;
	// build errors are fail-closed (A12).
	bboxSQL := ""
	if sqltoken.SQLite.ContainsTokenFold(qtext, config.BboxToken, "!BOX!") {
		var err error
		bboxSQL, err = buildBBoxPredicate(layer, bboxExtent)
		if err != nil {
			return "", err
		}
	}

	var pixelWidth, pixelHeight, scaleDenominator float64
	if sqltoken.SQLite.ContainsTokenFold(qtext, config.PixelWidthToken, config.PixelHeightToken, config.ScaleDenominatorToken) {
		var err error
		pixelWidth, pixelHeight, scaleDenominator, err = provider.TileScale(tile, layer.SRID())
		if err != nil {
			return "", fmt.Errorf("layer (%v) scale tokens: %w", layer.name, err)
		}
	}

	var geomType string
	if layer.geomType != nil {
		geomType = codec.GeomTypeName(layer.geomType)
	}

	z, x, y := tile.ZXY()
	return sqltoken.SQLite.MapTokens(qtext, func(tok string) string {
		switch tok {
		case config.BboxToken, "!BOX!":
			return bboxSQL
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
			return quoteTokenIdentifier(layer.idFieldname)
		case config.GeomFieldToken:
			return quoteTokenIdentifier(layer.geomFieldname)
		case config.GeomTypeToken:
			return geomType
		}
		return tok
	}), nil
}

// buildBBoxPredicate builds the !BBOX! expansion for the layer. Custom MOS
// SQL filters over bounds columns quantized to raw MOS integers
// (BoundsMOSRaw); every other bounds-backed use (custom gpkg SQL, the
// native GeoPackage RTree path) filters in the source CRS over the resolved
// bounds fields (layer > provider > defaults MINX/MAXX/MINY/MAXY; SQLite
// identifiers are case-insensitive).
func buildBBoxPredicate(layer *Layer, bboxExtent *geom.Extent) (string, error) {
	fields := layer.bboxFields
	if fields == (codec.BBoxFields{}) {
		// registration resolves the names (layer > provider > defaults);
		// fall back to the defaults for bare layers.
		fields = codec.DefaultBBoxFields()
	}
	if layer.tablename == "" && layer.geometryFormat == codec.FormatMOS {
		return codec.BuildBoundsPredicate(
			fields, bboxExtent, codec.BoundsMOSRaw, layer.mosConfig,
			codec.BoundsQuote(layer.bboxTable, sqliteQuoteIdent),
		)
	}
	return codec.BuildBoundsPredicate(fields, bboxExtent, codec.BoundsSourceCRS, layer.mosConfig, sqliteQuoteIdent)
}

// uppercaseTokens makes SQL tokens case-insensitive, matching PostGIS.
// Only code-context tokens are normalized; token-looking text in string
// literals, identifiers, or comments keeps its exact bytes.
func uppercaseTokens(str string) string {
	return sqltoken.SQLite.MapTokens(str, strings.ToUpper)
}

func trimTrailingSemicolon(sqlText string) string {
	sqlText = strings.TrimSpace(sqlText)
	return strings.TrimSpace(strings.TrimSuffix(sqlText, ";"))
}
