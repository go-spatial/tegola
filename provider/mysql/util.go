package mysql

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/internal/sqltoken"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// replaceTokens replaces tile and layer metadata tokens in a SQL query.
// Only tokens in SQL code context are substituted; token-looking text
// inside string literals, quoted identifiers, comments, or dollar-quoted
// strings is left verbatim.
//
// bboxExtent must be the tile's buffered extent transformed to the layer's
// source SRID. Pixel dimensions use the unbuffered extent in the source CRS;
// scale uses its linear units or a latitude-adjusted geographic conversion.
//
// The bounds predicate is built lazily: only queries that actually carry a
// bbox token (in code context) pay for it. Predicate build errors are
// fail-closed in bounds-backed modes (MOS custom SQL) and for the native
// spatial filter: they are returned to the caller instead of silently
// degrading to 1=1.
func replaceTokens(qtext string, layer *Layer, tile provider.Tile, bboxExtent *geom.Extent) (string, error) {
	qtext = uppercaseTokens(qtext)

	// only build a bounds predicate when the query carries a bbox token
	bboxSQL := "1=1"
	if sqltoken.MySQL.ContainsTokenFold(qtext, config.BboxToken, "!BOX!") {
		var err error
		bboxSQL, err = boundsSQLForLayer(layer, bboxExtent)
		if err != nil {
			return "", err
		}
	}

	var pixelWidth, pixelHeight, scaleDenominator float64
	if sqltoken.MySQL.ContainsTokenFold(qtext, config.PixelWidthToken, config.PixelHeightToken, config.ScaleDenominatorToken) {
		// Deferred inspection learns the source CRS from rows only after this
		// SQL executes. Never substitute units from its provisional SRID.
		// Runtime callers pass a request-local Layer copy, as does header
		// resolution; these fields are not shared mutable state.
		if layer.deferredInspection && !layer.crsExplicit && !layer.deferredSRIDLocked {
			return "", fmt.Errorf("layer (%v) scale tokens require a resolved source CRS; configure srid or crs_defn for deferred custom SQL", layer.name)
		}

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
	return sqltoken.MySQL.MapTokens(qtext, func(tok string) string {
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

// rawGeometryBoundsWarning returns the registration-time warning for a
// raw-geometry-format layer (wkb/wkt/mos) whose per-tile query has no
// bounds-columns-backed server-side predicate: tegola scans the whole layer
// source on every tile request and filters geometries in memory (or via a
// non-indexable per-row predicate over raw geometry storage). Returns ""
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
// warnings for the configured layers (audit P6-19). MOS layers always
// filter server-side over the configured bounds columns (the generated
// table query and the required !BBOX! in custom SQL both expand to the
// bounds-columns predicate); raw wkb/wkt storage cannot use such a
// predicate and is fully scanned per tile.
func rawGeometryBoundsWarnings(layers map[string]Layer) []string {
	var msgs []string
	for _, l := range layers {
		boundsPredicateUsed := l.geometryFormat == codec.FormatMOS ||
			codec.MySQL.SQLHasBBoxToken(l.sql, config.BboxToken, "!BOX!")
		if msg := rawGeometryBoundsWarning(l.name, l.geometryFormat, boundsPredicateUsed); msg != "" {
			msgs = append(msgs, msg)
		}
	}
	sort.Strings(msgs)
	return msgs
}

// boundsSQLForLayer builds the !BBOX! replacement for a layer. MOS blobs are
// opaque proprietary binaries: the server has no geometry functions over
// them, so the filter is the raw-bounds predicate over the configured
// bounds columns. Native spatial formats filter via ST_Intersects; raw
// deferred/synthetic layers fall back to the in-memory check (1=1). Build
// errors are returned (fail-closed), never logged-and-continued.
func boundsSQLForLayer(layer *Layer, bboxExtent *geom.Extent) (string, error) {
	if layer.geometryFormat == GeometryFormatMOS {
		return mosBoundsSQL(layer, bboxExtent)
	}
	if !layer.deferredInspection && !basic.IsSyntheticSRID(layer.srid) {
		if bboxExtent == nil {
			return "", fmt.Errorf("layer (%v): nil tile extent for bounds predicate", layer.name)
		}
		// WKT geometry columns hold text, so ST_Intersects(col, ...) fails;
		// wrap the column in ST_GeomFromText so MariaDB/MySQL can intersect
		// it with the tile bbox polygon. WKB columns hold BLOBs:
		// ST_GeomFromWKB is the documented constructor, avoiding implicit
		// BLOB->geometry coercion.
		geomRef := quoteIdentifier(layer.geomFieldname)
		switch layer.geometryFormat {
		case GeometryFormatWKT:
			geomRef = geomFromTextSQL(geomRef, layer.srid, layer.serverFlavor)
		case GeometryFormatWKB:
			geomRef = geomFromWKBSQL(geomRef, layer.srid, layer.serverFlavor)
		}
		return fmt.Sprintf(
			"ST_Intersects(%v, %v)",
			geomRef,
			geomFromTextSQL(sqlStringLiteral(wktPolygon(bboxExtent)), layer.srid, layer.serverFlavor),
		), nil
	}
	return "1=1", nil
}

// axisOrderSQL returns the ST_GeomFromText/ST_GeomFromWKB axis-order option
// for MySQL servers with a geographic (degrees) SRID. MySQL 8 interprets
// such values latitude-first per its SRS metadata, while tegola writes WKT
// and bbox polygons longitude-first, so without 'axis-order=long-lat' bbox
// filters and stored geometries disagree (audit P6-3). MariaDB does not
// support the options argument, and projected SRIDs have no axis order, so
// both keep the plain constructor form.
func axisOrderSQL(srid uint64, serverFlavor string) string {
	if serverFlavor == GeometryFormatMySQL && codec.IsGeographicSRID(srid) {
		return ", 'axis-order=long-lat'"
	}
	return ""
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

// geomFromTextSQL creates a geometry expression with the layer SRID when one
// is configured. MySQL and MariaDB otherwise assign SRID 0 to WKT values;
// comparing that value with a geometry column that has a non-zero SRID can
// fail with a different-SRID error instead of applying the spatial filter.
// On MySQL servers with a geographic SRID the value is read longitude-first
// via the 'axis-order=long-lat' option (audit P6-3).
func geomFromTextSQL(value string, srid uint64, serverFlavor string) string {
	if srid == 0 {
		return fmt.Sprintf("ST_GeomFromText(%v)", value)
	}
	return fmt.Sprintf("ST_GeomFromText(%v, %d%v)", value, srid, axisOrderSQL(srid, serverFlavor))
}

// geomFromWKBSQL creates a geometry expression from a raw WKB BLOB column
// with the layer SRID when one is configured, symmetric to geomFromTextSQL.
// ST_GeomFromWKB is the documented MySQL/MariaDB constructor for WKB values;
// without it, spatial predicates would rely on implicit BLOB->geometry
// coercion whose behavior differs between server versions. On MySQL servers
// with a geographic SRID the value is read longitude-first via the
// 'axis-order=long-lat' option (audit P6-3).
func geomFromWKBSQL(value string, srid uint64, serverFlavor string) string {
	if srid == 0 {
		return fmt.Sprintf("ST_GeomFromWKB(%v)", value)
	}
	return fmt.Sprintf("ST_GeomFromWKB(%v, %d%v)", value, srid, axisOrderSQL(srid, serverFlavor))
}

// mosBoundsSQL builds a coarse indexed filter for the raw bounds stored
// alongside MOS blobs. The bounds use the quantized MOS coordinate units,
// while bboxExtent is expressed in the layer CRS metres after applying
// mosUnitsFactor. Field names come from the resolved layer BBoxFields
// (layer > provider > defaults). An invalid scale is a configuration error,
// not a silent filter bypass.
func mosBoundsSQL(layer *Layer, bboxExtent *geom.Extent) (string, error) {
	return codec.BuildBoundsPredicate(
		layer.bboxFields,
		bboxExtent,
		codec.BoundsMOSRaw,
		layer.mosConfig,
		codec.BoundsQuote(layer.bboxTable, quoteIdentifier),
	)
}

// uppercaseTokens makes SQL tokens case-insensitive, matching PostGIS and
// GPKG. Only code-context tokens are normalized; token-looking text in
// string literals, identifiers, or comments keeps its exact bytes.
func uppercaseTokens(str string) string {
	return sqltoken.MySQL.MapTokens(str, strings.ToUpper)
}

func trimTrailingSemicolon(sqlText string) string {
	sqlText = strings.TrimSpace(sqlText)
	return strings.TrimSpace(strings.TrimSuffix(sqlText, ";"))
}
