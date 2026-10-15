package crsconfig

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

const MaxFeatureProjectionDefinitionBytes = 65536

// FeatureProjection owns immutable horizontal projection mathematics. Definition
// preserves the caller's admitted spelling; custom profiles claim no authority.
type FeatureProjection struct {
	projection *HeightProjection
	definition string
	profile    uint64
	identity   string
}

// CanonicalFeatureDefinition returns shipped internal-XY mathematics without
// consulting or registering a mutable projection. It proves no source authority.
func CanonicalFeatureDefinition(srid uint64) (string, bool) {
	switch {
	case srid == 4326:
		return "+proj=longlat +datum=WGS84", true
	case srid == 3857:
		return "+proj=merc +a=6378137 +b=6378137 +lat_ts=0.0 +lon_0=0.0 +x_0=0.0 +y_0=0 +k=1.0", true
	case srid >= 32601 && srid <= 32660:
		return fmt.Sprintf("+proj=utm +zone=%d +datum=WGS84 +units=m +no_defs", srid-32600), true
	case srid >= 32701 && srid <= 32760:
		return fmt.Sprintf("+proj=utm +zone=%d +south +datum=WGS84 +units=m +no_defs", srid-32700), true
	default:
		return "", false
	}
}

// NewFeatureProjection admits canonical profiles and explicitly validated custom
// transverse Mercator definitions. Unsupported semantics are rejected.
func NewFeatureProjection(definition string) (*FeatureProjection, error) {
	if len(definition) == 0 || len(definition) > MaxFeatureProjectionDefinitionBytes || !utf8.ValidString(definition) || strings.IndexByte(definition, 0) >= 0 {
		return nil, fmt.Errorf("feature projection definition unavailable or invalid")
	}
	parameters, ok := featureCanonicalParameters(definition)
	if !ok {
		return nil, fmt.Errorf("feature projection parameter profile unsupported")
	}
	identifiers := []uint64{4326, 3857}
	for zone := uint64(1); zone <= 60; zone++ {
		identifiers = append(identifiers, 32600+zone, 32700+zone)
	}
	for _, srid := range identifiers {
		canonical, _ := CanonicalFeatureDefinition(srid)
		expected, _ := featureCanonicalParameters(canonical)
		if parameters != expected {
			continue
		}
		projection, err := NewHeightProjection(srid)
		if err != nil {
			return nil, err
		}
		return &FeatureProjection{projection: projection, definition: definition, profile: srid}, nil
	}
	projection, err := newCustomFeatureProjection(definition)
	if err != nil {
		return nil, err
	}
	return &FeatureProjection{projection: projection, definition: definition, identity: parameters}, nil
}

// Equivalent establishes horizontal mathematical identity of admitted profiles.
// It says nothing about source authority, wire axes or vertical coordinates.
func (p *FeatureProjection) Equivalent(other *FeatureProjection) bool {
	if p == nil || other == nil || p.projection == nil || other.projection == nil {
		return false
	}
	if p.profile != 0 || other.profile != 0 {
		return p.profile != 0 && p.profile == other.profile
	}
	return p.identity != "" && p.identity == other.identity
}

func featureCanonicalParameters(definition string) (string, bool) {
	words := strings.Fields(definition)
	if len(words) == 0 || len(words) > 32 {
		return "", false
	}
	keys := make(map[string]bool, len(words))
	kept := make([]string, 0, len(words))
	for _, word := range words {
		if !strings.HasPrefix(word, "+") {
			return "", false
		}
		key, value, hasValue := strings.Cut(word[1:], "=")
		if key == "" || keys[key] {
			return "", false
		}
		keys[key] = true
		if key == "no_defs" {
			if hasValue {
				return "", false
			}
			continue
		}
		if key == "south" {
			if hasValue {
				return "", false
			}
		} else if !hasValue || value == "" {
			return "", false
		}
		kept = append(kept, word)
	}
	sort.Strings(kept)
	return strings.Join(kept, " "), true
}

func (p *FeatureProjection) Definition() string {
	if p == nil {
		return ""
	}
	return p.definition
}

// CanonicalSRID identifies the admitted horizontal mathematical profile. It
// does not establish a source authority, wire axes or vertical coordinates.
func (p *FeatureProjection) CanonicalSRID() uint64 {
	if p == nil || p.projection == nil {
		return 0
	}
	return p.profile
}

// CanonicalDefinition returns the shipped definition of the admitted profile.
func (p *FeatureProjection) CanonicalDefinition() string {
	definition, _ := CanonicalFeatureDefinition(p.CanonicalSRID())
	return definition
}

// mercatorWorldExtent is the canonical Web Mercator world edge: R*π for the
// WGS84 semi-major axis R = 6378137 m. Full-world EPSG:3857 bbox tests use it.
const mercatorWorldExtent = 20037508.342789244

// mercatorLatitudeLimit is the WGS84 latitude whose Web Mercator image is the
// canonical world edge: atan(sinh(π)) in degrees. The conventional rounded
// 85.05112878° projects to ≈20037508.34304, marginally outside the canonical
// extent, so a polar clamp must use the exact latitude to keep the clamped
// vertex inside full-world EPSG:3857 bbox tests.
var mercatorLatitudeLimit = 180 / math.Pi * math.Atan(math.Sinh(math.Pi))

// ClampForwardDomain clamps a WGS84 lon/lat pair into the finite forward domain
// of an admitted projection SRID, reporting whether a clamp was applied. Only
// Web Mercator (3857) needs it: the poles are valid WGS84 source data but have
// no finite Mercator image, and the vendored forward fails there. The clamp
// pins latitude to the exact world edge above, keeping every image inside the
// canonical ±20037508.342789244 m extent. Callers invoke it only as a fallback
// after a forward failure, so legitimate transforms are never altered.
// Non-finite latitudes, pairs of the wrong shape, and every other admitted
// profile (which forwards all finite lon/lat) never clamp.
func ClampForwardDomain(srid uint64, ll []float64) ([]float64, bool) {
	if srid != 3857 || len(ll) != 2 {
		return nil, false
	}
	switch lat := ll[1]; {
	case lat > mercatorLatitudeLimit:
		return []float64{ll[0], mercatorLatitudeLimit}, true
	case lat < -mercatorLatitudeLimit:
		return []float64{ll[0], -mercatorLatitudeLimit}, true
	}
	return nil, false
}

func (p *FeatureProjection) Forward(xy []float64) ([]float64, error) {
	if p == nil || p.projection == nil {
		return nil, fmt.Errorf("feature projection unavailable")
	}
	return p.projection.Forward(xy)
}
func (p *FeatureProjection) Inverse(xy []float64) ([]float64, error) {
	if p == nil || p.projection == nil {
		return nil, fmt.Errorf("feature projection unavailable")
	}
	return p.projection.Inverse(xy)
}
