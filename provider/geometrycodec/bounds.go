// bounds.go implements the common bounds-backed SQL contract shared by all
// standard storage providers (gpkg, postgis, mysql, hana): configurable
// bounds-field names, the DB-independent !BBOX! predicate builder for raw
// bounds columns, and the registration-time SQL MapplGIS inspection probe
// that detects bounds-backed MOS custom SQL without a live MapplGIS table
// contract.
package geometrycodec

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/cmp"
	"github.com/alexeydott/tegola/dict"
)

// BBox field-name configuration keys. These name the result columns/aliases
// that carry the raw bounds of each feature for bounds-backed MOS SQL.
const (
	ConfigKeyBBoxMinXField = "bbox_minx_fieldname"
	ConfigKeyBBoxMaxXField = "bbox_maxx_fieldname"
	ConfigKeyBBoxMinYField = "bbox_miny_fieldname"
	ConfigKeyBBoxMaxYField = "bbox_maxy_fieldname"
)

// Default bounds field names, matching the MapplGIS EGKO convention.
const (
	BBoxMinXDefault = "MINX"
	BBoxMaxXDefault = "MAXX"
	BBoxMinYDefault = "MINY"
	BBoxMaxYDefault = "MAXY"
)

// MapplGISSource distinguishes how a layer was detected as MapplGIS. Table
// canonical detection guarantees SystemInfo; SQL sample detection does not.
type MapplGISSource int

const (
	MapplGISNone MapplGISSource = iota
	// MapplGISTableCanonical: DDL + PK + indexes + OKEY=1 SystemInfo probe.
	MapplGISTableCanonical
	// MapplGISSQLSample: bounds contract plus automatic MOS evidence (DetectsMOS).
	MapplGISSQLSample
)

// String returns a human-readable source name for logs and errors.
func (s MapplGISSource) String() string {
	switch s {
	case MapplGISTableCanonical:
		return "table-canonical"
	case MapplGISSQLSample:
		return "sql-sample"
	}
	return "none"
}

// DefaultBBoxFields returns the default bounds field names as a BBoxFields
// value (MINX, MAXX, MINY, MAXY), handy for constructing test layers.
func DefaultBBoxFields() BBoxFields {
	return BBoxFields{BBoxMinXDefault, BBoxMaxXDefault, BBoxMinYDefault, BBoxMaxYDefault}
}

// BBoxFields holds the resolved bounds field names for a layer, following
// the layer > provider > defaults precedence of every other common key.
type BBoxFields [4]string

// Field index constants for BBoxFields.
const (
	BBoxMinXIdx = iota
	BBoxMaxXIdx
	BBoxMinYIdx
	BBoxMaxYIdx
)

// ResolveBBoxFields resolves the four bounds field names from the provider
// and layer configs. Resolution is per-field: a layer-level value overrides
// the provider-level value for that field only, so partial overrides are
// supported. Unset at both levels falls back to the documented defaults.
// Configured values are TrimSpace'd; whitespace-only or empty values,
// duplicates (case-insensitively), and anything that is not a simple SQL
// identifier (e.g. qualified names like t.MINX) are registration errors.
// layer may be nil; layerName is used for error context.
func ResolveBBoxFields(provider, layer dict.Dicter, layerName string) (BBoxFields, error) {
	keys := []struct {
		key string
		def string
		idx int
	}{
		{ConfigKeyBBoxMinXField, BBoxMinXDefault, BBoxMinXIdx},
		{ConfigKeyBBoxMaxXField, BBoxMaxXDefault, BBoxMaxXIdx},
		{ConfigKeyBBoxMinYField, BBoxMinYDefault, BBoxMinYIdx},
		{ConfigKeyBBoxMaxYField, BBoxMaxYDefault, BBoxMaxYIdx},
	}

	var fields BBoxFields
	for _, k := range keys {
		fields[k.idx] = k.def

		if provider != nil {
			if _, explicit := provider.Interface(k.key); explicit {
				v, err := provider.String(k.key, nil)
				if err != nil {
					return fields, fmt.Errorf("for layer (%v) invalid %v: %v", layerName, k.key, err)
				}
				if strings.TrimSpace(v) == "" {
					return fields, fmt.Errorf("for layer (%v) invalid %v: empty value; provide a non-empty column name or omit the key to use the default %v", layerName, k.key, k.def)
				}
				fields[k.idx] = strings.TrimSpace(v)
			}
		}
		if layer != nil {
			if _, explicit := layer.Interface(k.key); explicit {
				v, err := layer.String(k.key, nil)
				if err != nil {
					return fields, fmt.Errorf("for layer (%v) invalid %v: %v", layerName, k.key, err)
				}
				if strings.TrimSpace(v) == "" {
					return fields, fmt.Errorf("for layer (%v) invalid %v: empty value; provide a non-empty column name or omit the key to use the default %v", layerName, k.key, k.def)
				}
				fields[k.idx] = strings.TrimSpace(v)
			}
		}
	}
	if err := fields.validate(layerName); err != nil {
		return fields, err
	}
	return fields, nil
}

// validate rejects duplicate bounds field names (case-insensitively) and
// names that are not simple SQL identifiers (e.g. qualified names like
// t.MINX), which must not reach the generated predicates unquoted.
func (f BBoxFields) validate(layerName string) error {
	seen := make(map[string]string, len(f))
	for i, name := range f {
		key := strings.ToLower(name)
		if first, dup := seen[key]; dup {
			return fmt.Errorf("for layer (%v) bounds field name %q (bbox_%v_fieldname) duplicates %q; bounds field names must be unique (case-insensitive)",
				layerName, name, [...]string{"minx", "maxx", "miny", "maxy"}[i], first)
		}
		seen[key] = name
		if !simpleIdentifierRegexp.MatchString(name) {
			return fmt.Errorf("for layer (%v) bounds field name %q is not a simple SQL identifier; use a bare column name (e.g. MINX), not a qualified name like t.MINX",
				layerName, name)
		}
	}
	return nil
}

// simpleIdentifierRegexp matches bare SQL identifiers: a letter or
// underscore followed by letters, digits or underscores.
var simpleIdentifierRegexp = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// IsBBoxField reports whether name matches any resolved bounds field name
// (case-insensitive, mirroring SQL column-name semantics). Providers use it
// to keep operational bounds columns out of feature tags.
func (f BBoxFields) IsBBoxField(name string) bool {
	for _, field := range f {
		if strings.EqualFold(strings.TrimSpace(name), field) {
			return true
		}
	}
	return false
}

// BoundsPredicateMode selects how the source extent is translated into the
// bounds-column predicate.
type BoundsPredicateMode int

const (
	// BoundsSourceCRS compares the bounds columns against the source extent
	// expressed in the layer CRS coordinates (GPKG custom `gpkg` binary).
	BoundsSourceCRS BoundsPredicateMode = iota
	// BoundsMOSRaw first scales the source extent from layer CRS units into
	// quantized MOS units (10^precision / unit factor) before comparison.
	BoundsMOSRaw
)

// BuildBoundsPredicate builds the DB-independent bounds overlap predicate
// used to replace !BBOX! for bounds-backed custom SQL:
//
//	<maxx_field> >= tile_minx AND <minx_field> <= tile_maxx
//	AND <maxy_field> >= tile_miny AND <miny_field> <= tile_maxy
//
// mode=BoundsMOSRaw scales the tile extent into quantized MOS units first
// (floor/ceil rounding, mirroring the MySQL/GPKG raw bounds filters). An
// invalid scale (non-finite, non-positive) is an error, never a silent 1=1.
// quote wraps each field name; pass a no-op wrapper for unquoted names.
func BuildBoundsPredicate(fields BBoxFields, extent *geom.Extent, mode BoundsPredicateMode, mosConfig MOSConfig, quote func(string) string) (string, error) {
	if extent == nil {
		return "", fmt.Errorf("bounds predicate: nil extent")
	}
	if quote == nil {
		quote = func(s string) string { return s }
	}

	minX, maxX, minY, maxY := extent.MinX(), extent.MaxX(), extent.MinY(), extent.MaxY()
	if mode == BoundsMOSRaw {
		rawScale, err := MOSRawScale(mosConfig)
		if err != nil {
			return "", fmt.Errorf("bounds predicate: %w", err)
		}
		minX, maxX = math.Floor(minX*rawScale), math.Ceil(maxX*rawScale)
		minY, maxY = math.Floor(minY*rawScale), math.Ceil(maxY*rawScale)
	}

	values := []float64{minX, maxX, minY, maxY}
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "", fmt.Errorf("bounds predicate: non-finite tile bound")
		}
	}

	format := func(v float64) string {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return fmt.Sprintf(
		"%v >= %v AND %v <= %v AND %v >= %v AND %v <= %v",
		quote(fields[BBoxMaxXIdx]), format(minX),
		quote(fields[BBoxMinXIdx]), format(maxX),
		quote(fields[BBoxMaxYIdx]), format(minY),
		quote(fields[BBoxMinYIdx]), format(maxY),
	), nil
}

// SQLGeometryContract is the outcome of the registration-time
// InspectSQLGeometryContract probe for a bounds-backed custom-SQL layer.
type SQLGeometryContract struct {
	// BoundsFields are the actual (database-reported) bounds column names
	// discovered in the sample, resolved case-insensitively against the
	// configured names. Empty when no bounds columns were found. Providers
	// must persist these actual names (layer.bboxFields), never the
	// configured spellings: PostGIS runtime predicates quote identifiers
	// case-sensitively.
	BoundsFields BBoxFields
	// GeometryField is the actual (database-reported) geometry column name
	// matched in the sample, resolved case-insensitively against the
	// configured geometry field (or the last result column when the
	// geometry field is not configured). Empty when unmatched.
	GeometryField string
	// ValidRows counts sample rows carrying a decodable geometry with at
	// least one coordinate point, in any supported storage format.
	ValidRows int
	// ValidMOSRows counts the ValidRows subset carrying a decodable MOS
	// geometry with a positive MOS signature.
	ValidMOSRows int
	// HasBounds reports whether all four bounds columns were found.
	HasBounds bool
	// SampleComplete means EOF was reached before the sampling budget.
	SampleComplete bool
}

// DetectsMOS accepts three valid samples, or a shorter completed result
// containing at least one valid MOS geometry. Budget exhaustion is not EOF.
func (c SQLGeometryContract) DetectsMOS() bool {
	return c.ValidMOSRows >= MinValidMOSRows || (c.SampleComplete && c.ValidMOSRows > 0)
}

// MinValidMOSRows is the early-stop threshold for automatic MOS detection.
// A completed shorter result needs only one valid MOS row (see DetectsMOS). It is an inference
// evidence bar only: an explicit MOS format skips sampling altogether.
// geometry_type controls class inference independently; row-count evidence
// never substitutes for the structural contract.
const MinValidMOSRows = 3

// InspectSQLGeometryContract runs a registration-time probe over a sample of
// the layer's custom SQL result set and reports the storage-format
// contract: presence of the four bounds columns (as ACTUAL result-column
// names), the matched geometry column name, and the number of decodable
// rows (any format and the MOS subset).
//
// Row source: next returns the next scanned result-row values, ok=false at
// end of rows, or an error. Providers MUST feed real scanned values (e.g.
// database/sql rows.Scan into []interface{}), never placeholders — the
// contract inspects the actual geometry column value of each row. The
// sample window is at most InspectionSampleLimit rows; the caller prepares
// the query through PrepareProbeSQL/WrapProbeSQL so the probe always
// executes the SQL without a spatial filter.
//
// A nil decode requests column metadata only and never calls next.
// Sampling stops after MinValidMOSRows successfully decoded MOS geometries.
// decode is the provider's row decoder (MOSRowDecode / AutoRowDecode):
// rows whose decode errors, or whose geometry is empty or carries no
// coordinates, are skipped and never counted; SystemInfo/layerinfo rows are
// always skipped, never applied. A next() error is returned to the caller
// (fail-closed). Row-level decode errors are not probe failures.
func InspectSQLGeometryContract(
	next func() ([]interface{}, bool, error),
	columnNames []string,
	geometryField string,
	bboxFields BBoxFields,
	decode RowDecode,
) (SQLGeometryContract, error) {
	var contract SQLGeometryContract

	boundsIdx, boundsCols := findBoundsColumnsIn(columnNames, bboxFields)
	if boundsIdx != nil {
		contract.BoundsFields = *boundsCols
		contract.HasBounds = true
	}

	// geometry column index; -1 when the geometry field is absent from the
	// result set (a bounds-backed contract cannot be verified without it).
	geomIdx := -1
	if geometryField == "" {
		// geometry assumed to be the last selected column
		if len(columnNames) > 0 {
			geomIdx = len(columnNames) - 1
			contract.GeometryField = columnNames[geomIdx]
		}
	} else {
		for i, name := range columnNames {
			if strings.EqualFold(name, geometryField) {
				geomIdx = i
				contract.GeometryField = columnNames[i]
				break
			}
		}
	}

	// Explicit storage formats need column metadata only. Do not consume rows.
	if decode == nil {
		return contract, nil
	}

	for read := 0; read < InspectionSampleLimit; read++ {
		row, ok, err := next()
		if err != nil {
			return contract, err
		}
		if !ok {
			contract.SampleComplete = true
			break
		}
		if geomIdx < 0 || geomIdx >= len(row) {
			continue
		}
		value := row[geomIdx]
		if value == nil {
			continue
		}
		geo, isMOS, derr := decode(value)
		if derr != nil || geo == nil {
			// row-level decode errors are skipped, never probe failures
			continue
		}
		if !hasGeometryCoordinates(geo) {
			continue
		}
		contract.ValidRows++
		if isMOS {
			contract.ValidMOSRows++
			if contract.ValidMOSRows >= MinValidMOSRows {
				break
			}
		}
	}
	return contract, nil
}

// hasGeometryCoordinates reports whether the geometry carries at least one
// coordinate point. Empty geometries (e.g. an empty line string) do not
// satisfy the probe contract. Unknown geometry types are treated as
// carrying coordinates so the probe stays permissive for future types.
func hasGeometryCoordinates(geo geom.Geometry) bool {
	if geo == nil || geom.IsNil(geo) {
		return false
	}
	if cmp.IsEmptyGeo(geo) {
		return false
	}
	return true
}

// findBoundsColumnsIn maps the sample's column names case-insensitively onto
// the configured bounds field names and returns the resolved [4]string in
// minx/maxx/miny/maxy order plus their sample column indices. Returns nil
// indices when any of the four bounds fields is missing from the columns.
func findBoundsColumnsIn(columns []string, bboxFields BBoxFields) (*[4]int, *BBoxFields) {
	lookup := make(map[string]int, len(columns))
	for i, name := range columns {
		lookup[strings.ToLower(strings.TrimSpace(name))] = i
	}
	var idx [4]int
	var resolved BBoxFields
	for i, field := range bboxFields {
		j, ok := lookup[strings.ToLower(strings.TrimSpace(field))]
		if !ok {
			return nil, nil
		}
		idx[i] = j
		resolved[i] = columns[j]
	}
	return &idx, &resolved
}
