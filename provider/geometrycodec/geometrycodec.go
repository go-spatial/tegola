// Package geometrycodec implements the provider-independent geometry format
// contract shared by all standard storage providers (gpkg, postgis, mysql,
// hana). It hosts the common configuration parsing for `geometry_format`,
// `mos_precision` and `mos_units`, the MOS decode adapter, MapplGIS LayerInfo
// (layer self-description) inspection/application and the exact in-memory
// geometry-vs-extent filter used when a spatial predicate cannot be pushed
// into SQL.
//
// MVT passthrough providers (mvt_postgis, mvt_hana) do not decode raw
// geometry and are intentionally outside this contract.
package geometrycodec

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/geom/encoding/wkt"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/mos"
)

// Common configuration keys. Providers may expose additional
// provider-specific format values (e.g. "mysql", "mariadb", "auto") but MUST
// support the common ones below.
const (
	ConfigKeyGeometryFormat = "geometry_format"
	ConfigKeyMOSPrecision   = "mos_precision"
	ConfigKeyMOSUnits       = "mos_units"
	ConfigKeyGeometryType   = "geometry_type"
	ConfigKeyGeomField      = "geometry_fieldname"
	ConfigKeyIDField        = "id_fieldname"
)

// Common geometry format values understood by every standard provider.
const (
	// FormatMOS selects the opaque MapplGIS MOS binary blob format.
	FormatMOS = "mos"
	// FormatWKB selects plain OGC WKB (e.g. selected via ST_AsBinary).
	FormatWKB = "wkb"
	// FormatWKT selects WKT text (e.g. selected via ST_AsText).
	FormatWKT = "wkt"
)

// IsRawFormat reports whether the given geometry format value selects a raw
// (non-native) geometry representation that the provider reads directly from
// the database without database-side conversion helpers (e.g. ST_AsBinary).
// Both WKB and WKT columns are raw formats, as is MOS. Raw formats share a
// common behavior across providers: the geometry column cannot be used in
// server-side geometry predicates (hence "IS NOT NULL" SQL guards and
// in-memory bbox filtering) and is excluded from MVT providers.
func IsRawFormat(format string) bool {
	switch format {
	case FormatMOS, FormatWKB, FormatWKT:
		return true
	}
	return false
}

// InspectionSampleLimit bounds startup geometry-class and automatic MOS format
// inference. Format inference stops after MinValidMOSRows successful geometries;
// explicit MOS uses column metadata only for the storage contract.
const InspectionSampleLimit = 16

// ValidateMVTGeometryFormat rejects raw geometry formats on MVT passthrough
// providers (mvt_postgis, mvt_hana): their geometry is MVT bytes produced by
// the database, not a raw feature geometry that can be decoded client-side.
// An empty format means the provider-native default and is always valid.
func ValidateMVTGeometryFormat(format string) error {
	if IsRawFormat(format) {
		return fmt.Errorf(
			"%v = %q is not supported for MVT providers; use a standard provider (postgis/hana) instead",
			ConfigKeyGeometryFormat, format,
		)
	}
	return nil
}

// ValidateRawCustomSQL enforces the raw custom-SQL contract shared by all
// providers. Capability-aware: the MOS format supports bounds-backed custom
// SQL via the configured bounds fields (bbox_minx_fieldname etc.), so
// !BBOX! is permitted for `mos`; wkb/wkt store geometries as BLOB/TEXT
// columns without raw bounds columns, so !BBOX! remains rejected for them —
// tegola applies an exact in-memory bbox filter instead. Token matching is
// case-insensitive and SQL-context aware: only tokens in code context count,
// mirroring the providers' substitution, so token-looking text in string
// literals, identifiers, or comments is not flagged.
// Non-raw formats and empty SQL are always accepted. For MOS callers that
// require bounds-backed SQL, use RequireBBoxCustomSQL afterwards.
func (d SQLDialect) ValidateRawCustomSQL(layerName, geometryFormat, customSQL string, bboxTokens ...string) error {
	if !IsRawFormat(geometryFormat) || customSQL == "" {
		return nil
	}
	if geometryFormat == FormatMOS {
		// bounds-backed MOS SQL: !BBOX! expands into the configured
		// bounds-column predicate, so the token is valid.
		return nil
	}
	for _, tok := range bboxTokens {
		if d.scanner.ContainsTokenFold(customSQL, tok) {
			return fmt.Errorf(
				"layer (%v): custom SQL cannot use %v with geometry_format=%q: raw formats (wkb/wkt) store geometries as BLOB/TEXT and have no native spatial column for the token's predicate; remove %v from the custom SQL (tegola applies an exact in-memory bbox filter instead)",
				layerName, tok, geometryFormat, tok,
			)
		}
	}
	return nil
}

// SQLHasBBoxToken reports whether customSQL carries any of the given
// bounds-predicate tokens (!BBOX! / !BOX!), case-insensitively. Only tokens
// in SQL code context count: token-looking text inside string literals,
// quoted identifiers, or comments does not.
func (d SQLDialect) SQLHasBBoxToken(customSQL string, bboxTokens ...string) bool {
	return d.scanner.ContainsTokenFold(customSQL, bboxTokens...)
}

// RequireBBoxCustomSQL reports whether customSQL for a bounds-backed layer
// is missing the required !BBOX! token (or its !BOX! alias). boundsBacked
// is the caller's bounds-backed mode flag: bounds-backed custom SQL (MOS
// custom SQL in all providers, custom gpkg SQL) relies on the bounds
// predicate over the configured bounds fields as its server-side
// selectivity, so the token is mandatory and its absence is a registration
// error.
func (d SQLDialect) RequireBBoxCustomSQL(layerName string, boundsBacked bool, customSQL string, bboxTokens ...string) error {
	if !boundsBacked || customSQL == "" {
		return nil
	}
	if d.SQLHasBBoxToken(customSQL, bboxTokens...) {
		return nil
	}
	return fmt.Errorf(
		"layer (%v): bounds-backed custom SQL must use %v: the bounds predicate over the configured bounds fields (bbox_minx_fieldname etc.) is the only server-side filter such SQL supports",
		layerName, bboxTokens[0],
	)
}

// Default MOS quantization settings: integer units with no offset.
const (
	MOSPrecisionDefault   = 0.0
	MOSUnitsFactorDefault = 1.0
)

// DefaultMOSPrecisionForUnits returns the default mos_precision paired with
// the given map-unit factor (metres per unit):
//
//	mm (0.001) → 0, cm (0.01) → 1, dm (0.1) → 1, m (1) → 2, km (1000) → 5.
//
// An unknown or undefined factor falls back to MOSPrecisionDefault.
func DefaultMOSPrecisionForUnits(unitFactor float64) float64 {
	switch unitFactor {
	case 0.001: // mm
		return 0
	case 0.01, 0.1: // cm, dm
		return 1
	case 1: // m
		return 2
	case 1000: // km
		return 5
	}
	return MOSPrecisionDefault
}

// MaxMOSPrecision bounds the decimal digit count a float64 can represent
// exactly in MOS dequantization.
const MaxMOSPrecision = 308

// ValidateMOSPrecision reports whether p is a usable decimal digit count for
// MOS quantized coordinates.
func ValidateMOSPrecision(p float64) error {
	if math.IsNaN(p) || math.IsInf(p, 0) ||
		p < 0 || math.Trunc(p) != p ||
		p > MaxMOSPrecision {
		return fmt.Errorf("must be a finite non-negative integer no greater than %d, got %v", MaxMOSPrecision, p)
	}
	return nil
}

// MOSConfig holds the resolved MOS quantization settings for a layer together
// with flags marking which values were set explicitly. Explicit settings
// always win over MapplGIS LayerInfo self-description.
type MOSConfig struct {
	// Precision is the number of decimal digits the quantized integer
	// coordinates carry (mos_precision config or MapplGIS LayerInfo precision).
	Precision float64
	// UnitFactor scales the dequantized coordinates from the configured map
	// units to metres of the projected CRS (mos_units config or
	// MapplGIS LayerInfo map units); defaults to 1.
	UnitFactor float64
	// PrecisionSet marks an explicit provider- or layer-level mos_precision.
	PrecisionSet bool
	// UnitsSet marks an explicit provider- or layer-level mos_units.
	UnitsSet bool
}

// DefaultMOSConfig returns the default MOS quantization configuration. The
// default precision is paired with the default units factor (metres).
func DefaultMOSConfig() MOSConfig {
	return MOSConfig{
		Precision:  DefaultMOSPrecisionForUnits(MOSUnitsFactorDefault),
		UnitFactor: MOSUnitsFactorDefault,
	}
}

// ResolveMOSConfig parses mos_precision and mos_units from the provider and
// layer configuration dictionaries. Layer-level values override provider
// level values. A nil layer dict is allowed. Errors are prefixed with the
// layer name when one is provided.
//
// The default mos_precision is paired with the effective units
// (see DefaultMOSPrecisionForUnits): an explicit mos_units without an
// explicit mos_precision selects the units-paired default precision.
func ResolveMOSConfig(provider, layer dict.Dicter, layerName string) (MOSConfig, error) {
	cfg := DefaultMOSConfig()

	prefix := ""
	if layerName != "" {
		prefix = fmt.Sprintf("for layer (%v) ", layerName)
	}

	precision, precisionSet, err := resolveMOSPrecision(provider, prefix)
	if err != nil {
		return cfg, err
	}
	if precisionSet {
		cfg.Precision, cfg.PrecisionSet = precision, true
	}

	unitsFactor, unitsSet, err := resolveMOSUnits(provider, prefix)
	if err != nil {
		return cfg, err
	}
	if unitsSet {
		cfg.UnitFactor, cfg.UnitsSet = unitsFactor, true
	}

	if layer == nil {
		return cfg, nil
	}

	precision, precisionSet, err = resolveMOSPrecision(layer, prefix)
	if err != nil {
		return cfg, err
	}
	if precisionSet {
		cfg.Precision, cfg.PrecisionSet = precision, true
	}

	unitsFactor, unitsSet, err = resolveMOSUnits(layer, prefix)
	if err != nil {
		return cfg, err
	}
	if unitsSet {
		cfg.UnitFactor, cfg.UnitsSet = unitsFactor, true
	}

	// mos_precision is paired with the effective units: when the caller did
	// not set it explicitly, derive the default from the units in effect.
	if !cfg.PrecisionSet {
		cfg.Precision = DefaultMOSPrecisionForUnits(cfg.UnitFactor)
	}

	return cfg, nil
}

func resolveMOSPrecision(cfg dict.Dicter, errPrefix string) (float64, bool, error) {
	if cfg == nil {
		return MOSPrecisionDefault, false, nil
	}
	if _, explicit := cfg.Interface(ConfigKeyMOSPrecision); !explicit {
		return MOSPrecisionDefault, false, nil
	}
	value := MOSPrecisionDefault
	v, err := cfg.Float(ConfigKeyMOSPrecision, &value)
	if err != nil {
		return 0, false, fmt.Errorf("%sinvalid %v: %v", errPrefix, ConfigKeyMOSPrecision, err)
	}
	if err := ValidateMOSPrecision(v); err != nil {
		return 0, false, fmt.Errorf("%sinvalid %v: %v", errPrefix, ConfigKeyMOSPrecision, err)
	}
	return v, true, nil
}

func resolveMOSUnits(cfg dict.Dicter, errPrefix string) (float64, bool, error) {
	if cfg == nil {
		return MOSUnitsFactorDefault, false, nil
	}
	def := ""
	name, err := cfg.String(ConfigKeyMOSUnits, &def)
	if err != nil {
		return 0, false, fmt.Errorf("%sinvalid %v: %v", errPrefix, ConfigKeyMOSUnits, err)
	}
	if strings.TrimSpace(name) == "" {
		return MOSUnitsFactorDefault, false, nil
	}
	units, uerr := mos.ParseMapUnits(name)
	if uerr != nil {
		return 0, false, fmt.Errorf("%sinvalid %v: %v", errPrefix, ConfigKeyMOSUnits, uerr)
	}
	factor, uerr := units.ToMetres()
	if uerr != nil {
		return 0, false, fmt.Errorf("%sinvalid %v: %v", errPrefix, ConfigKeyMOSUnits, uerr)
	}
	return factor, true, nil
}

// MergeMOSConfig merges a layer-level MOS config override onto a base
// (provider-level) config. Explicit layer values override the base
// atomically: the value and its explicit flag travel together, so a layer
// value never bleeds into the provider base and vice versa. Values left
// unset at both levels keep the base's (default) values.
//
// mos_precision is paired with the effective units: when the layer
// overrides mos_units without an explicit mos_precision (and the base
// precision was not explicitly set either), the default precision for the
// resulting units is re-derived (see DefaultMOSPrecisionForUnits). This
// makes MergeMOSConfig equivalent to resolving the provider+layer configs
// through ResolveMOSConfig in one pass.
func MergeMOSConfig(base, override MOSConfig) MOSConfig {
	cfg := base
	if override.PrecisionSet {
		cfg.Precision, cfg.PrecisionSet = override.Precision, true
	}
	if override.UnitsSet {
		cfg.UnitFactor, cfg.UnitsSet = override.UnitFactor, true
	}
	if override.UnitsSet && !cfg.PrecisionSet {
		// units changed without an explicit precision anywhere: re-pair
		// the precision with the effective units.
		cfg.Precision = DefaultMOSPrecisionForUnits(cfg.UnitFactor)
	}
	return cfg
}

// HasExplicitMOSParams reports whether any MOS quantization setting was
// explicitly configured (layer or provider level).
func (c MOSConfig) HasExplicitMOSParams() bool {
	return c.PrecisionSet || c.UnitsSet
}

// WarnAndResetMOSParams implements the MOS relevance policy shared by the
// SQL providers: explicit mos_precision / mos_units settings only apply when
// the effective layer geometry format selects MOS decoding. When the format
// is not FormatMOS (and not one of the alsoValid exceptions, such as the
// MySQL "auto" format) and explicit parameters were configured, a warning is
// logged and the config is reset to defaults. It reports whether the config
// was reset.
func WarnAndResetMOSParams(geometryFormat string, cfg *MOSConfig, layerName string, alsoValid ...string) bool {
	if geometryFormat == FormatMOS || cfg == nil || !cfg.HasExplicitMOSParams() {
		return false
	}
	for _, format := range alsoValid {
		if geometryFormat == format {
			return false
		}
	}
	log.Warnf("layer (%v): %v / %v only apply when %v = %q; ignoring values",
		layerName, ConfigKeyMOSPrecision, ConfigKeyMOSUnits, ConfigKeyGeometryFormat, geometryFormat)
	*cfg = DefaultMOSConfig()
	return true
}

// ResolveLayerGeometryFormat resolves the effective layer geometry format:
// an optional layer-level geometry_format key, validated against allowed,
// overriding the provider-level value. An empty (or unset) layer value
// falls back to providerValue. layer may be nil; layerName is used for
// error context. allowed must contain every non-empty format value the
// provider accepts at either level.
func ResolveLayerGeometryFormat(providerValue string, layer dict.Dicter, layerName string, allowed map[string]struct{}) (string, error) {
	if layer == nil {
		return providerValue, nil
	}
	if _, explicit := layer.Interface(ConfigKeyGeometryFormat); !explicit {
		return providerValue, nil
	}
	v := ""
	v, err := layer.String(ConfigKeyGeometryFormat, &v)
	if err != nil {
		return "", fmt.Errorf("for layer (%v) invalid %v: %v", layerName, ConfigKeyGeometryFormat, err)
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return providerValue, nil
	}
	if _, ok := allowed[v]; !ok {
		keys := make([]string, 0, len(allowed))
		for k := range allowed {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return "", fmt.Errorf("for layer (%v) invalid %v: %q (expected one of: %v)",
			layerName, ConfigKeyGeometryFormat, v, strings.Join(keys, ", "))
	}
	return v, nil
}

// ApplySystemInfo fills in the unset values from a parsed
// MapplGIS LayerInfo layer self-description. Explicit configuration always
// wins. The receiver is updated in place so both startup registration and
// per-tile runtime application share one implementation. nil is a no-op.
func (c *MOSConfig) ApplySystemInfo(si *mos.SystemInfo) error {
	if si == nil {
		return nil
	}
	if err := ValidateMOSPrecision(float64(si.Precision)); err != nil {
		return fmt.Errorf("invalid system-info MOS precision: %v", err)
	}
	if !c.PrecisionSet {
		c.Precision = float64(si.Precision)
	}
	if !c.UnitsSet && si.MapUnitsDefined {
		factor, err := si.ScaleToMetres()
		if err != nil {
			return fmt.Errorf("unable to convert MOS map units %v to metres: %v", si.MapUnits, err)
		}
		c.UnitFactor = factor
	}
	return nil
}

// Options builds the low-level mos decode options for this configuration.
func (c MOSConfig) Options() mos.Options {
	return mos.Options{
		Precision:  c.Precision,
		UnitFactor: c.UnitFactor,
	}
}

// blobValue normalizes driver values that may arrive as []byte or string.
func blobValue(v interface{}) ([]byte, bool) {
	switch val := v.(type) {
	case []byte:
		return val, true
	case string:
		return []byte(val), true
	}
	return nil, false
}

// IsSystemInfoValue reports whether v looks like a MapplGIS LayerInfo
// version wrapper blob rather than a regular MOS geometry.
func IsSystemInfoValue(v interface{}) bool {
	b, ok := blobValue(v)
	if !ok {
		return false
	}
	return mos.IsSystemInfoBlob(b)
}

// ParseSystemInfoValue parses v as a MapplGIS LayerInfo blob.
func ParseSystemInfoValue(v interface{}) (mos.SystemInfo, error) {
	b, ok := blobValue(v)
	if !ok {
		return mos.SystemInfo{}, fmt.Errorf("unexpected system info column type %T, expected blob", v)
	}
	return mos.ParseSystemInfo(b)
}

// DecodeMOS decodes a MOS blob using the resolved quantization settings.
// MOS carries no SRID, so callers must apply their configured CRS.
func DecodeMOS(v interface{}, cfg MOSConfig) (geom.Geometry, error) {
	b, ok := blobValue(v)
	if !ok {
		return nil, fmt.Errorf("unexpected MOS geometry column type %T, expected blob", v)
	}
	g, err := mos.Decode(b, cfg.Options())
	if err != nil {
		return nil, fmt.Errorf("error decoding MOS geometry: %v", err)
	}
	return g, nil
}

// DecodeWKB decodes a plain OGC WKB value ([]byte or string).
func DecodeWKB(v interface{}) (geom.Geometry, error) {
	b, ok := blobValue(v)
	if !ok {
		return nil, fmt.Errorf("unexpected geometry column type %T, expected blob", v)
	}
	g, err := wkb.DecodeBytes(b)
	if err != nil {
		// %w keeps wkb.ErrUnknownGeometryType intact so providers can
		// detect it with errors.As and skip unsupported 3D geometries.
		return nil, fmt.Errorf("error decoding WKB geometry: %w", err)
	}
	return g, nil
}

// DecodeWKT decodes a WKT text value (string or []byte).
func DecodeWKT(v interface{}) (geom.Geometry, error) {
	switch s := v.(type) {
	case string:
		g, err := wkt.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("error decoding WKT geometry: %v", err)
		}
		return g, nil
	case []byte:
		g, err := wkt.DecodeBytes(s)
		if err != nil {
			return nil, fmt.Errorf("error decoding WKT geometry: %v", err)
		}
		return g, nil
	default:
		return nil, fmt.Errorf("unexpected geometry column type %T, expected text", v)
	}
}

// GeometryIntersectsExtent reports whether a geometry's bounding box
// intersects the given extent. It backs the exact in-memory spatial filter
// for formats whose blobs cannot be filtered by the database (e.g. MOS).
// Undecodable bounds keep the geometry (conservative).
func GeometryIntersectsExtent(g geom.Geometry, e *geom.Extent) bool {
	if g == nil || e == nil {
		return true
	}
	gb, err := geom.NewExtentFromGeometry(g)
	if err != nil || gb == nil {
		return true
	}
	// geom.Extent.Intersect rejects zero-width/height extents. That is
	// correct for area intersections but drops valid Point and degenerate
	// geometry features before they reach MVT encoding. Bounds overlap is
	// inclusive here because touching the tile boundary still intersects it.
	return gb.MinX() <= e.MaxX() &&
		gb.MaxX() >= e.MinX() &&
		gb.MinY() <= e.MaxY() &&
		gb.MaxY() >= e.MinY()
}

// GeomTypeName returns the OGC-style name ("POINT", "MULTIPOLYGON", ...)
// of a decoded geometry, used for the !GEOM_TYPE! token.
func GeomTypeName(g geom.Geometry) string {
	switch g.(type) {
	case geom.Point:
		return "POINT"
	case geom.MultiPoint:
		return "MULTIPOINT"
	case geom.LineString:
		return "LINESTRING"
	case geom.MultiLineString:
		return "MULTILINESTRING"
	case geom.Polygon:
		return "POLYGON"
	case geom.MultiPolygon:
		return "MULTIPOLYGON"
	case geom.Collection:
		return "GEOMETRYCOLLECTION"
	}
	return ""
}

// GeometryTypeFromName maps an OGC-style geometry type name (case
// insensitive) to the corresponding empty tegola geometry value. It is the
// shared parser behind the common `geometry_type` layer key.
func GeometryTypeFromName(name string) (geom.Geometry, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "point":
		return geom.Point{}, nil
	case "linestring":
		return geom.LineString{}, nil
	case "polygon":
		return geom.Polygon{}, nil
	case "multipoint":
		return geom.MultiPoint{}, nil
	case "multilinestring":
		return geom.MultiLineString{}, nil
	case "multipolygon":
		return geom.MultiPolygon{}, nil
	case "geometrycollection":
		return geom.Collection{}, nil
	}
	return nil, fmt.Errorf("unsupported geometry_type (%v)", name)
}

// ResolveGeometryType reads the common `geometry_type` layer key. It returns
// (geometry, true, nil) when the key is present and valid, (nil, false, nil)
// when the key is absent, empty, or the explicit "auto" alias (which selects
// type inference), and an error for an unsupported value.
// Providers use the explicit value to fix the layer geometry type before any
// data is read, which skips startup type inspection (including the sampling
// query that would otherwise infer the type).
func ResolveGeometryType(layerConf dict.Dicter, layerName string) (geom.Geometry, bool, error) {
	var raw string
	raw, err := layerConf.String(ConfigKeyGeometryType, &raw)
	if err != nil {
		return nil, false, fmt.Errorf("layer (%v) %v: %w", layerName, ConfigKeyGeometryType, err)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, false, nil
	}
	if strings.EqualFold(strings.TrimSpace(raw), "auto") {
		return nil, false, nil
	}
	g, err := GeometryTypeFromName(raw)
	if err != nil {
		return nil, false, fmt.Errorf("layer (%v): %w", layerName, err)
	}
	return g, true, nil
}

// warnedGeomTypeMismatch tracks layers that already produced a mixed-content
// geometry-type warning, so the warning is emitted at most once per layer.
var warnedGeomTypeMismatch sync.Map

// WarnOnceGeometryTypeMismatch implements the mixed-content policy for
// explicitly configured `geometry_type` values: features whose decoded type
// differs from the declared type are permitted, but the mismatch is logged
// once per layer. It reports whether this call emitted the warning.
func WarnOnceGeometryTypeMismatch(layerName string, declared, got geom.Geometry) bool {
	if declared == nil || got == nil {
		return false
	}
	if GeomTypeName(declared) == GeomTypeName(got) {
		return false
	}
	if _, loaded := warnedGeomTypeMismatch.LoadOrStore(layerName, struct{}{}); loaded {
		return false
	}
	log.Warnf("layer (%v): feature geometry type %v does not match configured geometry_type %v; permitting mixed content",
		layerName, GeomTypeName(got), GeomTypeName(declared))
	return true
}

func ValidateRawCustomSQL(layerName, geometryFormat, customSQL string, bboxTokens ...string) error {
	return Legacy.ValidateRawCustomSQL(layerName, geometryFormat, customSQL, bboxTokens...)
}

func SQLHasBBoxToken(customSQL string, bboxTokens ...string) bool {
	return Legacy.SQLHasBBoxToken(customSQL, bboxTokens...)
}

func RequireBBoxCustomSQL(layerName string, boundsBacked bool, customSQL string, bboxTokens ...string) error {
	return Legacy.RequireBBoxCustomSQL(layerName, boundsBacked, customSQL, bboxTokens...)
}
