// probe.go implements the shared registration-time custom-SQL probe
// machinery used by the standard storage providers (gpkg, mysql, postgis,
// hana): the documented token-neutralization order for the permissive
// inspection query, the sample wrapping, the per-row decode closures, and
// the fail-closed structural contract validation.
//
// Probe contract: the probe always executes the SQL without a spatial
// filter (!BBOX!/!BOX! neutralized to 1=1) and with permissive
// position/zoom placeholders so tile-dependent SQL still returns its rows.
// Every provider prepares the probe query through PrepareProbeSQL in ONE
// documented order and feeds InspectSQLGeometryContract from real scanned
// row values via a next() iterator.
package geometrycodec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/internal/sqltoken"
	"github.com/alexeydott/tegola/mos"
)

// Web Mercator (EPSG:3857) half-extent in metres. The probe reference tile
// is z=0/x=0/y=0, i.e. the full extent (same formula the runtime token
// replacement uses at a tile, see e.g. provider/*/util.go replaceTokens).
const probeWebMercatorMax = 20037508.342789244

// InspectionQueryTimeout bounds registration-time probe and sample queries
// (audit P5-16): a hung or pathological database must not stall provider
// registration forever. It is the budget for one probe query, not for the
// whole registration.
const InspectionQueryTimeout = 30 * time.Second

// NewInspectionContext returns a context carrying InspectionQueryTimeout for
// one registration-time probe/sample query (audit P5-16). The caller must
// call the returned cancel function.
func NewInspectionContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), InspectionQueryTimeout)
}

// z0/0/0 Web Mercator reference values for the scale/pixel tokens. Computed
// exactly like the runtime token replacement (pixel width over 256px tiles,
// 0.28mm pixel size) so probe SQL matches tile-time semantics.
var (
	probePixelWidth       = (2 * probeWebMercatorMax) / 256
	probePixelHeight      = probePixelWidth
	probeScaleDenominator = probePixelWidth / 0.00028
)

// probePermissivePredicate is the always-true predicate tile/zoom/BBOX
// references collapse to during probe neutralization.
const probePermissivePredicate = "1=1"

// probeAllZooms is the permissive replacement for zoom-level comparisons
// ("x >= !ZOOM!" becomes "x IN (0,...,24)").
const probeAllZooms = "IN (0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24)"

// Comparison-level neutralization (audit P6-9). A comparison whose side
// operand is a tile/zoom token must collapse to the permissive predicate
// no matter which side of the operator the token is on: the token-left form
// ("!ZOOM! >= 5") used to fall through to the bare "0" substitution below
// ("0 >= 5") and silently sampled zero rows. The operand is matched as an
// atom plus any trailing operator/keyword continuation (arithmetic,
// BETWEEN, LIKE, IN, IS NULL...) so the whole predicate is consumed and
// never cut apart; shapes outside this grammar (e.g. function-call
// operands) fall back to the safe bare-token substitution below, which is
// exactly the pre-P6-9 SQL.
const (
	// probeCompareOp matches the comparison operators; two-character
	// forms are matched first so they are not cut in half.
	probeCompareOp = `(?:>=|=>|=<|<=|!=|<>|=|>|<)`
	// probeOperandAtom matches one operand: string/number literals,
	// one-level parenthesized groups, identifiers (optionally schema- or
	// table-qualified), or another token.
	probeOperandAtom = `(?:'[^']*'|"[^"]*"|` +
		`[-+]?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?|` +
		`\([^()]*\)|` +
		`[A-Za-z_][A-Za-z0-9_$]*(?:\.[A-Za-z_][A-Za-z0-9_$]*)?|` +
		`![A-Z0-9_]+!)`
	// probeExprContinue consumes operator/keyword continuations that must
	// not dangle after the neutralized comparison ("cx = !X! + 1",
	// "!X! = y BETWEEN 1 AND 2", "!X! = y IS NULL"...).
	probeExprContinue = `(?:(?:\s*(?:[-+*/]|::|\|\|)\s*` + probeOperandAtom + `)|` +
		`(?:\s+(?i:IS)\s+(?i:NOT\s+)?(?:(?i:NULL)|(?i:TRUE)|(?i:FALSE)|(?i:UNKNOWN)|(?i:DISTINCT\s+FROM)\s+` + probeOperandAtom + `))|` +
		`(?:\s+(?i:NOT\s+)?(?i:BETWEEN)\s+(?:(?i:SYMMETRIC)|(?i:ASYMMETRIC))?\s*` + probeOperandAtom + `\s+(?i:AND)\s+` + probeOperandAtom + `)|` +
		`(?:\s+(?i:NOT\s+)?(?i:LIKE|ILIKE)\s+` + probeOperandAtom + `)|` +
		`(?:\s+(?i:NOT\s+)?(?i:SIMILAR)\s+(?i:TO)\s+` + probeOperandAtom + `)|` +
		`(?:\s+(?i:NOT\s+)?(?i:IN)\s*` + probeOperandAtom + `))*`
	// probeCompareTokenLeft covers every tile/zoom token in token-first
	// comparisons; the token-last forms below exclude !ZOOM! so right-side
	// zoom comparisons keep the richer full-range substitution.
	probeCompareTokenLeft  = `!(?:ZOOM|Z|X|Y)!`
	probeCompareTokenRight = `!(?:Z|X|Y)!`
)

// probeTokenLeftCompareRegexp matches token-first comparisons
// ("!ZOOM! >= 5", "!X! = !Y!"); probeTokenRightCompareRegexp matches
// token-last ones ("cx = !X! + 1", "5 <= !X!"). The leading boundary
// character is captured in group 1 and re-emitted with the replacement so
// the surrounding SQL is never cut apart; the right-side form deliberately
// excludes "(" from its boundary class so an expression like "f(x) >= !X!"
// is never truncated (it falls back to the safe bare-token substitution).
var (
	probeTokenLeftCompareRegexp  = regexp.MustCompile(`(^|[\s(,;])` + probeCompareTokenLeft + `\s*` + probeCompareOp + `\s*` + probeOperandAtom + probeExprContinue)
	probeTokenRightCompareRegexp = regexp.MustCompile(`(^|[\s,;])` + probeOperandAtom + probeExprContinue + `\s*` + probeCompareOp + `\s*` + probeCompareTokenRight + probeExprContinue)
)

// probeZoomCompareRegexp matches zoom-level comparisons with the token on
// the right ("z >= !ZOOM!" / "z>=!zoom!" / "z <= !zoom!"). The operator is
// swallowed together with the token so the comparison collapses onto the
// permissive IN list; any operator/keyword continuation is swallowed along
// with it so nothing dangles ("z = !ZOOM! + 1" must not become
// "z IN (0,...) + 1"). Token-first forms are handled by
// probeTokenLeftCompareRegexp above.
var probeZoomCompareRegexp = regexp.MustCompile(probeCompareOp + `\s*` + `(?i)!ZOOM!` + probeExprContinue)

// probeKnownTokens are the token spellings PrepareProbeSQL normalizes to
// their canonical uppercase form before substitution (the same token
// surface the providers' runtime replaceTokens understands).
var probeKnownTokens = []string{
	config.BboxToken, "!BOX!",
	config.ZoomToken, config.ZToken, config.XToken, config.YToken,
	config.ScaleDenominatorToken, config.PixelWidthToken, config.PixelHeightToken,
	config.IdFieldToken, config.GeomFieldToken, config.GeomTypeToken,
}

// PrepareProbeSQL neutralizes a custom-SQL query for the registration-time
// probe. The probe always executes the SQL without a spatial filter and
// with permissive tile placeholders; the documented substitution order is:
//
//  1. known !WORD! tokens are normalized to their canonical uppercase
//     spelling (case-insensitive token surface);
//  2. comparisons referencing a tile/zoom token collapse to the permissive
//     "1=1" predicate regardless of which side of the operator the token
//     is on ("!ZOOM! >= 5", "cx = !X! + 1", "!X! = !Y!"...). Right-side
//     zoom comparisons ("min_zoom <= !ZOOM!") instead collapse to the
//     permissive IN (0,...,24) range;
//  3. !BBOX!/!BOX! expand to "1=1" — the probe never applies a spatial
//     filter;
//  4. remaining bare !ZOOM!/!Z!/!X!/!Y! expand to "0" (projection
//     positions such as "!X! AS tile_x", or operand shapes outside the
//     comparison grammar — e.g. function-call operands — where the numeric
//     fallback keeps the SQL valid exactly as before);
//  5. !SCALE_DENOMINATOR!/!PIXEL_WIDTH!/!PIXEL_HEIGHT! expand to the z=0
//     Web Mercator reference values (identical to the runtime expansion at
//     tile 0/0/0); !ID_FIELD!/!GEOM_FIELD!/!GEOM_TYPE! expand to the
//     provided names.
//
// Unknown !token! values (custom query parameters) are left untouched:
// they are resolved by the parameter layer at tile time.
//
// SQL-context awareness: every step above touches only tokens and
// comparisons in SQL code context. Token-looking text inside string
// literals, quoted identifiers, comments, or PostgreSQL dollar-quoted
// strings stays verbatim (and is never uppercased by the normalization),
// so probing cannot rewrite documented or quoted text.
func (d SQLDialect) PrepareProbeSQL(customSQL, geomField, idField, geomType string) string {
	sql := d.normalizeProbeTokens(customSQL)
	// collapse tile/zoom comparisons to the permissive predicate no matter
	// which side of the operator the token is on (audit P6-9). The
	// token-left pass runs first so token-to-token comparisons collapse in
	// one step instead of being cut into bare substitutions. A match whose
	// span carries token-looking text in a protected context is left
	// verbatim: collapsing it would rewrite quoted or commented text.
	sql = d.replaceAllGuarded(sql, probeTokenLeftCompareRegexp, probePermissivePredicate, true)
	sql = d.replaceAllGuarded(sql, probeTokenRightCompareRegexp, probePermissivePredicate, true)
	sql = d.replaceAllGuarded(sql, probeZoomCompareRegexp, probeAllZooms, false)
	// the result is embedded into wrapping inspection queries
	// ("SELECT ... FROM (%s) ..."), so a trailing semicolon must go
	sql = d.scanner.MapTokens(sql, func(tok string) string {
		switch tok {
		case config.BboxToken, "!BOX!":
			return probePermissivePredicate
		case config.ZoomToken, config.ZToken, config.XToken, config.YToken:
			return "0"
		case config.ScaleDenominatorToken:
			return strconv.FormatFloat(probeScaleDenominator, 'f', 8, 64)
		case config.PixelWidthToken:
			return strconv.FormatFloat(probePixelWidth, 'f', 8, 64)
		case config.PixelHeightToken:
			return strconv.FormatFloat(probePixelHeight, 'f', 8, 64)
		case config.IdFieldToken:
			return idField
		case config.GeomFieldToken:
			return geomField
		case config.GeomTypeToken:
			return geomType
		}
		return tok
	})
	return strings.TrimSuffix(strings.TrimSpace(sql), ";")
}

// normalizeProbeTokens rewrites the known tokens to their canonical
// uppercase spelling so the probe substitution is case-insensitive. Only
// code-context tokens are normalized; token-looking text in protected
// contexts keeps its exact bytes.
func (d SQLDialect) normalizeProbeTokens(sql string) string {
	return d.scanner.MapTokens(sql, func(tok string) string {
		for _, known := range probeKnownTokens {
			if strings.EqualFold(tok, known) {
				return known
			}
		}
		return tok
	})
}

// replaceAllGuarded mirrors re.ReplaceAllString(sql, "${1}"+repl) (or
// re.ReplaceAllString(sql, repl) when emitGroup1 is false) except that
// matches whose span contains token-looking text in a protected context
// are left verbatim instead of collapsed. The probe replacements only
// re-emit submatch 1, so the expansion is modeled by the flag rather than
// a full ${N} expander.
func (d SQLDialect) replaceAllGuarded(sql string, re *regexp.Regexp, repl string, emitGroup1 bool) string {
	matches := re.FindAllStringSubmatchIndex(sql, -1)
	if len(matches) == 0 {
		return sql
	}
	var b strings.Builder
	prev := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		b.WriteString(sql[prev:start])
		if d.scanner.SpanHasProtectedToken(sql, start, end) {
			b.WriteString(sql[start:end])
		} else {
			if emitGroup1 && len(m) >= 4 && m[2] >= 0 {
				b.WriteString(sql[m[2]:m[3]])
			}
			// A compact comparison such as z=!ZOOM! needs a separator
			// before the replacement keyword; preserve existing whitespace.
			if strings.HasPrefix(repl, "IN ") && start > 0 && sql[start-1] > ' ' {
				b.WriteByte(' ')
			}
			b.WriteString(repl)
		}
		prev = end
	}
	b.WriteString(sql[prev:])
	return b.String()
}

// WrapProbeSQL wraps a prepared custom-SQL query in the standard probe
// sample window: at most InspectionSampleLimit rows.
func WrapProbeSQL(sql string) string {
	return fmt.Sprintf("SELECT * FROM (%s) AS __tegola_bounds_probe LIMIT %d",
		probeBody(sql), InspectionSampleLimit)
}

// WrapProbeSQLTopStyle wraps a prepared custom-SQL query in the TOP-style
// probe sample window used by HANA: at most InspectionSampleLimit rows.
func WrapProbeSQLTopStyle(sql string) string {
	return fmt.Sprintf("SELECT TOP %d * FROM (%s) AS __tegola_bounds_probe",
		InspectionSampleLimit, probeBody(sql))
}

// RowDecode decodes one raw result-column value into a geometry. isMOS
// reports whether the value carried a MOS blob (as opposed to a
// native/WKB/WKT value); an error means the row cannot be counted (the
// probe skips it, e.g. SystemInfo rows or malformed blobs).
type RowDecode func(value interface{}) (geom.Geometry, bool, error)

// probeValueBytes normalizes the raw driver representations of a blob
// column into bytes. Besides []byte/string it handles driver LOB values
// carrying a *bytes.Buffer (e.g. the HANA driver's driver.Lob, matched
// structurally through its Writer() accessors).
func probeValueBytes(v interface{}) ([]byte, bool) {
	switch val := v.(type) {
	case []byte:
		return val, true
	case string:
		return []byte(val), true
	case *bytes.Buffer:
		if val == nil {
			return nil, false
		}
		return val.Bytes(), true
	case interface{ Writer() io.Writer }:
		if w, ok := val.Writer().(*bytes.Buffer); ok && w != nil {
			return w.Bytes(), true
		}
	case interface{ Reader() io.Reader }:
		if r, ok := val.Reader().(*bytes.Buffer); ok && r != nil {
			return r.Bytes(), true
		}
	}
	return nil, false
}

// decodeMOSValue decodes v as a MOS blob. The blob must first carry a
// positive MOS signature (the MOS header must validate as MOS) and then
// decode as a complete MOS geometry; SystemInfo/layerinfo rows are never
// applied, they are reported as errors so the probe skips them.
func decodeMOSValue(v interface{}, cfg MOSConfig) (geom.Geometry, error) {
	if IsSystemInfoValue(v) {
		return nil, fmt.Errorf("system info row skipped")
	}
	b, ok := probeValueBytes(v)
	if !ok {
		return nil, fmt.Errorf("unexpected geometry column type %T, expected MOS blob", v)
	}
	// Positive MOS signature required: a native/WKB/WKT value must never
	// count as a MOS row.
	if _, err := mos.DecodeHeader(b); err != nil {
		return nil, fmt.Errorf("no MOS signature: %v", err)
	}
	return DecodeMOS(v, cfg)
}

// MOSRowDecode returns the probe row decoder for an explicit MOS layer:
// rows count as MOS only when they carry a valid MOS signature and decode
// as MOS geometries.
func MOSRowDecode(cfg MOSConfig) RowDecode {
	return func(value interface{}) (geom.Geometry, bool, error) {
		g, err := decodeMOSValue(value, cfg)
		if err != nil {
			return nil, false, err
		}
		return g, true, nil
	}
}

// AutoRowDecode returns the probe row decoder used by the storage-format
// inference: native values are tried first and count as non-MOS rows;
// otherwise a positive MOS signature plus a decodable MOS geometry counts
// as a MOS row. A native/WKB/WKT-decodable row never counts as MOS.
func AutoRowDecode(native func(interface{}) (geom.Geometry, error), cfg MOSConfig) RowDecode {
	return func(value interface{}) (geom.Geometry, bool, error) {
		if IsSystemInfoValue(value) {
			return nil, false, fmt.Errorf("system info row skipped")
		}
		if native != nil {
			if g, err := native(value); err == nil && g != nil {
				return g, false, nil
			}
		}
		g, err := decodeMOSValue(value, cfg)
		if err != nil {
			return nil, false, err
		}
		return g, true, nil
	}
}

// ErrSQLGeometryContract is the fail-closed registration error raised when
// a bounds-backed custom SQL query violates the structural contract.
type ErrSQLGeometryContract struct {
	Layer  string
	Reason string
}

func (e ErrSQLGeometryContract) Error() string {
	return fmt.Sprintf("layer (%v) custom SQL geometry contract: %v", e.Layer, e.Reason)
}

// ResolveBoundsSQLContract validates the structural contract of a
// bounds-backed custom SQL query and resolves the bounds field names the
// layer must persist for the runtime !BBOX! predicate.
//
// Fail-closed registration errors — the runtime tile query cannot execute
// without them:
//
//   - the SQL must contain a bbox token (!BBOX! or !BOX!, case-insensitive,
//     in code context — token text in strings/comments/identifiers does
//     not count);
//   - when a geometry column is configured, the probe must have matched it
//     in the result columns.
//
// Bounds columns missing from the result set are NOT an error
// (boundsInResult=false): the generated !BBOX! predicate expands INSIDE the
// custom SQL and resolves in the query's own scope (e.g. the source table's
// bounds columns — filtered on but not selected in the SELECT list), so the
// resolved chain (layer > provider > MINX/MAXX/MINY/MAXY) stays in effect.
// Callers warn and register the layer instead of failing the whole
// provider: the fail-closed variant of this gate (b5ad0979) regressed
// working MapplGIS SQL for every provider.
//
// When all four bounds columns are present in the result set
// (boundsInResult=true), resolved carries their ACTUAL result-column names
// — runtime predicates quote identifiers case-sensitively, so callers must
// persist those instead of the configured spellings. Otherwise resolved is
// the configured chain passed in.
func (d SQLDialect) ResolveBoundsSQLContract(layerName, sql, geometryField string, contract SQLGeometryContract, configured BBoxFields) (resolved BBoxFields, boundsInResult bool, err error) {
	if !d.scanner.ContainsTokenFold(sql, config.BboxToken, "!BOX!") {
		return configured, false, ErrSQLGeometryContract{Layer: layerName,
			Reason: fmt.Sprintf("missing %v token; bounds-backed custom SQL must carry a bounds predicate", config.BboxToken)}
	}
	if geometryField != "" && contract.GeometryField == "" {
		return configured, false, ErrSQLGeometryContract{Layer: layerName,
			Reason: fmt.Sprintf("geometry column %q not present in the result columns", geometryField)}
	}
	if contract.HasBounds {
		return contract.BoundsFields, true, nil
	}
	return configured, false, nil
}

// ValidateMOSSQLExplicitConfig enforces the CRS policy for MOS custom SQL:
// MOS carries no CRS metadata, so the layer configuration must state the
// source CRS explicitly (srid or crs_defn). mos_precision/mos_units stay
// optional with their paired defaults (DefaultMOSPrecisionForUnits /
// DefaultMOSConfig).
func ValidateMOSSQLExplicitConfig(layerName string, crsExplicit bool) error {
	if !crsExplicit {
		return fmt.Errorf("layer (%v): source CRS unresolved; MOS columns carry no CRS metadata, specify srid or crs_defn in the layer configuration", layerName)
	}
	return nil
}

// degreesCRS lists the common geographic (degrees) CRS codes whose units
// are not metric. This is the warn-only 1.3 policy for the Web-Mercator-
// metres scale tokens.
var degreesCRS = map[int]struct{}{
	4326: {}, 4269: {}, 4258: {}, 4490: {}, 4214: {}, 4674: {}, 4230: {}, 4267: {},
}

// IsGeographicSRID reports whether the SRID is one of the common geographic
// (degrees) CRS codes in degreesCRS. Providers use it to apply
// axis-order-sensitive SQL for MySQL's latitude-first geographic SRS
// metadata (audit P6-3) and to detect non-metric scale tokens.
func IsGeographicSRID(srid uint64) bool {
	_, ok := degreesCRS[int(srid)]
	return ok
}

// crsDefnConfigKey mirrors crsconfig.KeyCRSDefn ("crs_defn"), referenced by
// literal to keep this package independent of crsconfig.
const crsDefnConfigKey = "crs_defn"

// WarnNonMetricScaleTokens warns (never errors) when custom SQL uses the
// scale-denominator / pixel-size tokens and the layer CRS is non-metric
// (degrees): those tokens are always computed as Web Mercator metres
// regardless of the layer CRS. Token values are left unchanged. The
// crs_defn text (layer over provider) is used as a hint for synthetic CRSs.
// Either cfg may be nil. Returns true when the warning was logged (for
// testability); callers may ignore the result. Token detection is
// case-insensitive but code-context only: token text in string literals,
// identifiers, or comments does not count as usage.
func WarnNonMetricScaleTokens(layerName, sql string, srid uint32, providerCfg, layerCfg dict.Dicter) bool {
	uses := sqltoken.ContainsTokenFold(sql, config.ScaleDenominatorToken, config.PixelWidthToken, config.PixelHeightToken)
	if !uses {
		return false
	}
	var crsDefn string
	for _, d := range []dict.Dicter{layerCfg, providerCfg} {
		if d == nil {
			continue
		}
		if v, err := d.String(crsDefnConfigKey, nil); err == nil {
			crsDefn = v
			break
		}
	}
	_, degrees := degreesCRS[int(srid)]
	if !degrees && basic.IsSyntheticSRID(uint64(srid)) {
		// synthetic CRS (crs_defn): warn only when the definition mentions
		// degrees, since its units are otherwise unknown.
		degrees = strings.Contains(strings.ToUpper(crsDefn), "DEGREE")
	}
	if !degrees {
		return false
	}
	log.Warnf("layer (%v): custom SQL uses %v/%v/%v tokens with a non-metric (degrees) CRS; "+
		"those tokens are always computed as Web Mercator metres regardless of the layer CRS",
		layerName, config.ScaleDenominatorToken, config.PixelWidthToken, config.PixelHeightToken)
	return true
}

func PrepareProbeSQL(customSQL, geomField, idField, geomType string) string {
	return Legacy.PrepareProbeSQL(customSQL, geomField, idField, geomType)
}

func normalizeProbeTokens(sql string) string { return Legacy.normalizeProbeTokens(sql) }

func replaceAllGuarded(sql string, re *regexp.Regexp, repl string, emitGroup1 bool) string {
	return Legacy.replaceAllGuarded(sql, re, repl, emitGroup1)
}

func ResolveBoundsSQLContract(layerName, sql, geometryField string, contract SQLGeometryContract, configured BBoxFields) (resolved BBoxFields, boundsInResult bool, err error) {
	return Legacy.ResolveBoundsSQLContract(layerName, sql, geometryField, contract, configured)
}

// A line comment at EOF must end before the enclosing probe's closing paren.
func probeBody(sql string) string {
	sql = strings.TrimSuffix(strings.TrimSpace(sql), ";")
	// This public wrapper has no provider context. A newline is harmless in
	// every dialect, so terminate a possible trailing line comment recognized
	// by any supported dialect (legacy scanning can miss ordinary backslashes).
	for _, dialect := range []sqltoken.Dialect{sqltoken.PostgreSQL, sqltoken.MySQL, sqltoken.SQLite, sqltoken.HANA} {
		segments := dialect.Scan(sql)
		if len(segments) == 0 {
			continue
		}
		last := segments[len(segments)-1]
		text := sql[last.Start:last.End]
		if last.Kind == sqltoken.Comment && (strings.HasPrefix(text, "--") || strings.HasPrefix(text, "#")) {
			return sql + "\n"
		}
	}
	return sql
}

// MetadataProbeSQL preserves result columns without requesting sample rows.
// The input is already token-expanded and may include a sample limit.
func MetadataProbeSQL(sql string) string {
	return fmt.Sprintf("SELECT * FROM (%s) AS __tegola_metadata_probe WHERE 1=0", probeBody(sql))
}
