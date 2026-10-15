package basic

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/alexeydott/proj"
)

// This file extends the three EPSG codes built into github.com/alexeydott/proj
// (3395, 3857, 4087) with a built-in table of commonly used projected
// coordinate systems and a RegisterProj4SRID hook so configs can register any
// additional SRID via a PROJ.4 string. Projections are described in PROJ.4
// syntax and registered with proj.CustomProjection; the actual math is
// performed by proj's ported operations (merc, utm/etmerc, aea, leac, aeqd,
// eqc, airy, august), plus geographic definitions (longlat aliases) with supported datum shifts.
// Geographic definitions with unsupported datum/unit/axis modifiers fail closed.
// Definitions using other PROJ.4 projections are
// rejected at registration time.

var (
	// proj4RegisterOnce guards proj.CustomProjection calls, which mutate a
	// global map inside the vendored library.
	proj4RegisterOnce sync.Once
	// proj4ProjectionMu serializes all accesses that mutate or inspect the
	// vendored projection registry. Provider initialization can register
	// several CRS definitions concurrently.
	proj4ProjectionMu sync.Mutex
	// proj4Registered tracks EPSG codes registered through RegisterProj4SRID
	// so the same code can be registered twice safely.
	proj4RegisteredMu sync.Mutex
	proj4Registered   = map[uint64]string{}
)

// builtinProj4SRIDs maps EPSG codes of widely used projected coordinate
// systems to their PROJ.4 definitions. EPSG:3395 / 3857 / 4087 are already
// built into the proj library and are not repeated here. EPSG:4326 is
// geographic and handled natively by basic.ToWebMercator / FromWebMercator.
//
//	326xx  — WGS 84 / UTM northern zones (32601..32660)
//	327xx  — WGS 84 / UTM southern zones (32701..32760)
//	284xx  — Pulkovo 1942 / Gauss-Kruger zones (28401..28432; x_0 includes the
//	         zone number prefix, the classic 8-digit easting convention)
//	2463..2491 — Pulkovo 1995 / Gauss-Kruger zones
//	2492..2522 — Pulkovo 1942 / Gauss-Kruger zones (x_0 = 500000, no zone prefix)
//	3785   — Popular Visualisation Pseudo Mercator (GDAL legacy alias of 3857)
//	900913 — the OSM "google" mercator alias of 3857
//	53004  — Sphere Mercator (ESRI:53004, spherical authalic earth)
var builtinProj4SRIDs = map[uint64]string{}

// gkZoneDef builds the PROJ.4 definition of a Gauss-Kruger zone on the
// Krassowsky ellipsoid. etmerc (extended transverse mercator) is the
// accurate tmerc implementation available in the vendored proj library.
//
// The datum transformation is part of the EPSG coordinate-system definition,
// not an optional rendering detail: without it, Pulkovo coordinates are
// incorrectly treated as WGS84 coordinates when they are converted to
// WebMercator.
func gkZoneDef(lon0, x0 float64, towgs84 string) string {
	return fmt.Sprintf("+proj=etmerc +lat_0=0 +lon_0=%v +k_0=1 +x_0=%v +y_0=0 +ellps=krass %s +units=m +no_defs", lon0, x0, towgs84)
}

const (
	// EPSG's area-specific transformation used by the Pulkovo 1942
	// Gauss-Kruger systems represented by 284xx and 2492..2522.
	pulkovo1942ToWGS84 = "+towgs84=25,-141,-78.5,0,-0.35,-0.736,0"
	// EPSG's transformation used by the Pulkovo 1995 Gauss-Kruger family.
	pulkovo1995ToWGS84 = "+towgs84=24.47,-130.89,-81.56,0,0,-0.13,-0.22"
)

func init() {
	// WGS 84 / UTM zones 1N..60N and 1S..60S
	for zone := 1; zone <= 60; zone++ {
		builtinProj4SRIDs[uint64(32600+zone)] = fmt.Sprintf("+proj=utm +zone=%d +datum=WGS84 +units=m +no_defs", zone)
		builtinProj4SRIDs[uint64(32700+zone)] = fmt.Sprintf("+proj=utm +zone=%d +south +datum=WGS84 +units=m +no_defs", zone)
	}
	// Pulkovo 1942 / Gauss-Kruger zones 1..32 (eastings prefixed with the
	// zone number via x_0 = zone*1e6 + 500000)
	for zone := 1; zone <= 32; zone++ {
		builtinProj4SRIDs[uint64(28400+zone)] = gkZoneDef(float64(zone*6-3), float64(zone*1000000+500000), pulkovo1942ToWGS84)
	}
	// Pulkovo 1995 / Gauss-Kruger zones: EPSG 2463..2491, central meridians
	// 21+E, step 6, wrapping at 180; false easting 500000 (no zone prefix).
	for i := 0; i <= 28; i++ {
		lon := 21 + i*6
		if lon > 180 {
			lon -= 360
		}
		builtinProj4SRIDs[uint64(2463+i)] = gkZoneDef(float64(lon), 500000, pulkovo1995ToWGS84)
	}
	// Pulkovo 1942 / Gauss-Kruger zones: EPSG 2492..2522, central meridians
	// 9+E, step 6, wrapping at 180; false easting 500000 (no zone prefix).
	for i := 0; i <= 30; i++ {
		lon := 9 + i*6
		if lon > 180 {
			lon -= 360
		}
		builtinProj4SRIDs[uint64(2492+i)] = gkZoneDef(float64(lon), 500000, pulkovo1942ToWGS84)
	}
	// Popular Visualisation Pseudo Mercator (legacy GDAL alias of 3857).
	builtinProj4SRIDs[3785] = "+proj=merc +lon_0=0 +k_0=1 +x_0=0 +y_0=0 +a=6378137 +b=6378137 +towgs84=0,0,0,0,0,0,0 +units=m +no_defs"
	// The OSM "google" mercator alias of 3857.
	builtinProj4SRIDs[900913] = "+proj=merc +a=6378137 +b=6378137 +lat_ts=0.0 +lon_0=0.0 +x_0=0.0 +y_0=0 +k_0=1.0 +units=m +no_defs"
	// Sphere Mercator (ESRI:53004) on the authalic sphere.
	builtinProj4SRIDs[53004] = "+proj=merc +lon_0=0 +k_0=1 +x_0=0 +y_0=0 +a=6371000 +b=6371000 +units=m +no_defs"
}

// RegisterProj4SRID registers (or replaces) the PROJ.4 definition of an EPSG
// code, making ToWebMercator / FromWebMercator able to convert geometries in
// that SRID. Providers call this during startup for SRIDs configured in their
// config files. Registration is idempotent per (srid, proj4) pair.
func RegisterProj4SRID(srid uint64, proj4 string) error {
	proj4 = strings.TrimSpace(proj4)
	if srid == 0 {
		return fmt.Errorf("RegisterProj4SRID: srid must be a positive EPSG code")
	}
	if proj4 == "" {
		return fmt.Errorf("RegisterProj4SRID: empty proj4 definition for srid %v", srid)
	}
	// reject unsupported-operation definitions early by validating a probe
	// point through proj itself
	if !isSupportedProj4(proj4) {
		return fmt.Errorf("RegisterProj4SRID: proj4 definition for srid %v is not a supported projection: %v", srid, proj4)
	}

	proj4RegisteredMu.Lock()
	defer proj4RegisteredMu.Unlock()
	if prev, ok := proj4Registered[srid]; ok && prev == proj4 {
		return nil
	}
	for defn, code := range proj4DefnCodes {
		if code == srid && defn != proj4 {
			return fmt.Errorf("RegisterProj4SRID: srid %d is reserved for registered crs_defn %q; choose a different explicit srid", srid, defn)
		}
	}
	proj4Registered[srid] = proj4

	proj4RegisterOnce.Do(func() {})
	proj4ProjectionMu.Lock()
	proj.CustomProjection(proj.EPSGCode(srid), proj4)
	proj4ProjectionMu.Unlock()
	return nil
}

// SyntheticSRIDMin is the first SRID assigned to a raw PROJ.4 definition.
// These identifiers exist only in the Tegola process and must not be sent to
// a database spatial-function API as if they were registered database SRSs.
const SyntheticSRIDMin uint64 = 340000001

// IsSyntheticSRID reports whether srid is a Tegola-only identifier allocated
// for a textual PROJ.4 definition rather than a database/EPSG SRS.
func IsSyntheticSRID(srid uint64) bool {
	return srid >= SyntheticSRIDMin
}

// Synthetic SRIDs sit above the probe code space (320000000+) and mark SRIDs
// synthesized from raw PROJ.4 definitions (crs_defn config options) rather
// than assigned a real EPSG code.
//
// Synthetic SRIDs are derived from the trimmed definition using a stable hash.
// A collision is rejected without changing any prior registration: providers
// retain the numeric SRID returned during startup, so it must never be rebound.
const defnSRIDSpan = 100000000

// proj4DefnCodes maps PROJ.4 definitions registered through RegisterProj4Defn
// to their synthetic SRIDs so repeated registrations of the same definition
// are stable within a process.
var proj4DefnCodes = map[string]uint64{}

// defnHash returns a stable FNV-1a hash of a normalized PROJ.4 definition. The
// algorithm is implemented here (rather than relying on the runtime's map
// hash) so the value is identical across processes and Go versions.
func defnHash(s string) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	h := uint64(offset64)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime64
	}
	return h
}

// defnFirstChoice returns the deterministic SRID a definition maps to before
// collision resolution: always within [SyntheticSRIDMin, SyntheticSRIDMin+defnSRIDSpan).
func defnFirstChoice(defn string) uint64 {
	return SyntheticSRIDMin + defnHash(defn)%defnSRIDSpan
}

// resolveDefnSRID returns the sole deterministic candidate for defn. A
// different owner is a collision, never permission to move an existing CRS or
// select an alternative code dependent on registration order.
func resolveDefnSRID(defn string, owners map[uint64]string) (uint64, bool) {
	code := defnFirstChoice(defn)
	owner, taken := owners[code]
	return code, !taken || owner == defn
}

// RegisterProj4Defn registers an arbitrary PROJ.4 coordinate system definition
// and returns a synthetic SRID standing in for it. Configs that carry a full
// textual CRS description (crs_defn) instead of a numeric EPSG code pass the
// definition here and use the returned SRID everywhere a numeric SRID is
// expected: layer config, !BBOX! reprojection and feature SRIDs. Registering
// the same definition twice returns the same synthetic SRID, and the SRID is
// derived deterministically from the definition, so the same crs_defn resolves
// to the same SRID across processes and config load orders. A hash collision
// with a different registered definition fails without altering either CRS.
func RegisterProj4Defn(proj4 string) (uint64, error) {
	proj4 = strings.TrimSpace(proj4)
	if proj4 == "" {
		return 0, fmt.Errorf("RegisterProj4Defn: empty proj4 definition")
	}
	if !isSupportedProj4(proj4) {
		return 0, fmt.Errorf("RegisterProj4Defn: proj4 definition is not a supported projection: %v", proj4)
	}

	proj4RegisteredMu.Lock()
	defer proj4RegisteredMu.Unlock()
	if code, ok := proj4DefnCodes[proj4]; ok {
		return code, nil
	}
	code, ok := resolveDefnSRID(proj4, proj4Registered)
	if !ok {
		return 0, fmt.Errorf("RegisterProj4Defn: synthetic SRID collision at %d between %q and registered definition %q; use distinct explicit SRIDs via RegisterProj4SRID instead of crs_defn", code, proj4, proj4Registered[code])
	}
	proj4Registered[code] = proj4
	proj4DefnCodes[proj4] = code

	proj4RegisterOnce.Do(func() {})
	proj4ProjectionMu.Lock()
	proj.CustomProjection(proj.EPSGCode(code), proj4)
	proj4ProjectionMu.Unlock()
	return code, nil
}

// Proj4DefnSRID returns the synthetic SRID previously assigned to a PROJ.4
// definition by RegisterProj4Defn, if any.
func Proj4DefnSRID(proj4 string) (uint64, bool) {
	proj4RegisteredMu.Lock()
	defer proj4RegisteredMu.Unlock()
	code, ok := proj4DefnCodes[strings.TrimSpace(proj4)]
	return code, ok
}

// proj4ProbePoint returns the geographic point used to validate a PROJ.4
// definition's forward and inverse operations. When the definition declares a
// projection center via +lon_0 / +lat_0 (or a UTM +zone), the center is used
// so that definitions whose valid domain excludes the historical fixed probe
// point (10, 50) — e.g. an orthographic projection centered on the Pacific —
// are not incorrectly rejected. Probing the declared center also catches
// definitions whose own center is not projectable. Definitions that declare no
// center fall back to the fixed point (10, 50).
func proj4ProbePoint(proj4 string) (lon, lat float64) {
	haveCenter := false
	haveZone := false
	var zone float64
	for _, field := range strings.Fields(proj4) {
		if !strings.HasPrefix(field, "+") {
			continue
		}
		key, val, ok := strings.Cut(strings.TrimPrefix(field, "+"), "=")
		if !ok {
			continue
		}
		switch key {
		case "lon_0":
			if v, err := strconv.ParseFloat(val, 64); err == nil {
				lon, haveCenter = v, true
			}
		case "lat_0":
			if v, err := strconv.ParseFloat(val, 64); err == nil {
				lat, haveCenter = v, true
			}
		case "zone":
			if v, err := strconv.ParseFloat(val, 64); err == nil {
				zone, haveZone = v, true
			}
		}
	}
	if haveCenter {
		// wrap the probe longitude into (-180, 180] so it is canonical
		lon = math.Mod(lon, 360)
		switch {
		case lon > 180:
			lon -= 360
		case lon <= -180:
			lon += 360
		}
		return lon, lat
	}
	if haveZone {
		// UTM central meridian; the natural center latitude is the equator
		return 6*zone - 183, 0
	}
	return 10, 50
}

// isSupportedProj4 validates a PROJ.4 string by asking proj to build and
// round-trip a conversion for it via a temporary EPSG code outside the
// standard range. Each validation uses a fresh probe code because proj caches
// conversions per EPSG code; reusing one would validate a definition against
// a previously cached conversion. Panics are recovered because some vendored
// operations (e.g. airy, august) panic in Inverse instead of returning an
// error.
func isSupportedProj4(proj4 string) (ok bool) {
	probe := nextProbeCode()
	proj4ProjectionMu.Lock()
	defer proj4ProjectionMu.Unlock()
	proj.CustomProjection(probe, proj4)
	defer proj.RemoveCustomProjection(probe)
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	// probe the projection's declared center when it has one (P6-21): a
	// single fixed point (10, 50) incorrectly rejects valid definitions
	// whose domain excludes that point.
	lon, lat := proj4ProbePoint(proj4)
	// forward then inverse must both succeed and produce finite results
	// (P6-8): a definition whose forward output overflows to Inf/NaN, or
	// whose inverse silently returns NaN instead of an error, is not usable.
	out, err := proj.Convert(probe, []float64{lon, lat})
	if err != nil {
		return false
	}
	if anyNonFinite(out) {
		return false
	}
	back, err := proj.Inverse(probe, out)
	if err != nil {
		return false
	}
	if anyNonFinite(back) {
		return false
	}
	// and the round trip must return close to the probe point (P6-8)
	if math.Abs(normalizeLonDelta(back[0]-lon)) > proj4RoundTripTolerance {
		return false
	}
	return math.Abs(back[1]-lat) <= proj4RoundTripTolerance
}

// proj4RoundTripTolerance bounds the forward-then-inverse error, in degrees,
// accepted when validating a PROJ.4 definition. The vendored spherical-merc
// forward/inverse path has a latitude-dependent round-trip artifact of up to
// ~4e-4 deg on definitions mixing a spherical forward with an ellipsoidal
// datum path (seen on EPSG:3785-style +towgs84 definitions at non-zero
// latitudes), so 1e-3 deg is the practical floor. Genuinely broken
// definitions fail by orders of magnitude (tens of degrees) or return
// non-finite values.
const proj4RoundTripTolerance = 1e-3

// anyNonFinite reports whether the slice contains a NaN or Inf value.
func anyNonFinite(v []float64) bool {
	for _, f := range v {
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return true
		}
	}
	return false
}

// normalizeLonDelta wraps a longitude difference into (-180, 180].
func normalizeLonDelta(d float64) float64 {
	d = math.Mod(d, 360)
	switch {
	case d > 180:
		d -= 360
	case d <= -180:
		d += 360
	}
	return d
}

// probeCodeBase sits above every EPSG code space (standard codes are <= 7
// digits) and below proj.EPSGCode's int range.
var probeCodeBase int64 = 320000000

func nextProbeCode() proj.EPSGCode {
	return proj.EPSGCode(atomic.AddInt64(&probeCodeBase, 1))
}

// KnownProj4SRIDs returns the sorted list of EPSG codes convertible through
// proj (built-in conversions plus the built-in table plus any SRIDs
// registered via RegisterProj4SRID).
func KnownProj4SRIDs() []uint64 {
	codes := proj.AvailableConversions()
	out := make([]uint64, 0, len(codes))
	for _, c := range codes {
		out = append(out, uint64(c))
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// IsBuiltinProj4SRID reports whether the SRID is covered by the built-in
// table (UTM / Gauss-Kruger) without explicit registration.
func IsBuiltinProj4SRID(srid uint64) bool {
	_, ok := builtinProj4SRIDs[srid]
	return ok
}

// BuiltinProj4Def returns the built-in PROJ.4 definition for the SRID, if any.
func BuiltinProj4Def(srid uint64) (string, bool) {
	def, ok := builtinProj4SRIDs[srid]
	return def, ok
}

// EffectiveProj4Definition snapshots a definition tracked by this package.
// Explicit registrations take precedence over the shipped definitions. The
// returned string does not consult or retain the mutable projection engine.
func EffectiveProj4Definition(srid uint64) (string, bool) {
	proj4RegisteredMu.Lock()
	defer proj4RegisteredMu.Unlock()
	if definition, ok := proj4Registered[srid]; ok {
		return definition, true
	}
	if definition, ok := builtinProj4SRIDs[srid]; ok {
		return definition, true
	}
	switch srid {
	case 3395:
		return "+proj=merc +lon_0=0 +k=1 +x_0=0 +y_0=0 +datum=WGS84", true
	case 4326:
		return "+proj=longlat +datum=WGS84", true
	case 3857:
		return "+proj=merc +a=6378137 +b=6378137 +lat_ts=0.0 +lon_0=0.0 +x_0=0.0 +y_0=0 +k=1.0", true
	case 4087:
		return "+proj=eqc +lat_ts=0 +lat_0=0 +lon_0=0 +x_0=0 +y_0=0 +datum=WGS84", true
	default:
		return "", false
	}
}

// RegisterBuiltinProj4SRIDs registers every entry of the built-in table with
// proj. Providers that support arbitrary SRIDs call this once at startup so
// common codes (UTM, Gauss-Kruger) work without any config.
func RegisterBuiltinProj4SRIDs() {
	proj4RegisteredMu.Lock()
	defer proj4RegisteredMu.Unlock()
	for srid, def := range builtinProj4SRIDs {
		if _, ok := proj4Registered[srid]; ok {
			continue
		}
		proj4Registered[srid] = def
		proj.CustomProjection(proj.EPSGCode(srid), def)
	}
	proj4RegisterOnce.Do(func() {})
}

// ParseProj4Config parses a proj4 config value of the form
// "EPSG_CODE=proj4 string" entries joined by newlines or semicolons, e.g.
//
//	proj4 = "2180=+proj=sterea +lat_0=... ; 2177=+proj=tmerc +lon_0=15"
//
// or a TOML table handled by the caller as individual entries. Returns a
// map of srid -> proj4 string.
func ParseProj4Config(entries []string) (map[uint64]string, error) {
	out := make(map[uint64]string)
	for i, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		eq := strings.Index(e, "=")
		if eq <= 0 {
			return nil, fmt.Errorf("proj4 entry %v (%q) must look like SRID=+proj=...", i+1, e)
		}
		key := strings.TrimSpace(e[:eq])
		key = strings.TrimPrefix(strings.TrimPrefix(key, "EPSG:"), "epsg:")
		srid, err := strconv.ParseUint(key, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("proj4 entry %v (%q): invalid srid: %v", i+1, e, err)
		}
		def := strings.TrimSpace(e[eq+1:])
		if def == "" {
			return nil, fmt.Errorf("proj4 entry %v (%q): empty proj4 definition", i+1, e)
		}
		out[srid] = def
	}
	return out, nil
}
