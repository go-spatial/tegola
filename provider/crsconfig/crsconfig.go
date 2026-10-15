// Package crsconfig implements the shared srid/crs_defn resolution contract
// used by the standard storage providers (postgis, gpkg, mysql, hana).
//
// Contract (identical for provider and layer config levels):
//
//   - srid: a numeric spatial reference id applied to the layer's source CRS.
//   - crs_defn: a full PROJ.4 definition. When present it wins over a numeric
//     srid on the same config level and is registered under a synthetic SRID
//     (basic.SyntheticSRIDMin and above) that flows through the regular
//     reprojection path.
//
// Precedence across levels:
//
//	layer crs_defn > layer srid > provider crs_defn > provider srid >
//	source auto-detect (geometry header / system info / spatial metadata)
//
// An explicit srid or crs_defn always wins over any value inferred from the
// data source, which is why Resolve reports an Explicit flag.
//
// SRID range reservation: numeric values at or above basic.SyntheticSRIDMin
// (340000001) are reserved for Tegola-internal synthetic codes allocated by
// RegisterProj4Defn and must not be configured as a plain numeric srid.
// (HANA's planar-equivalent offset 1000000000 sits inside that range; HANA
// treats planar-equivalent ids as real database SRSs, never as synthetic.)
package crsconfig

import (
	"fmt"
	"strings"

	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/log"
)

// Config keys shared by every standard storage provider.
const (
	KeySRID    = "srid"
	KeyCRSDefn = "crs_defn"
)

// CRS is the result of resolving the srid/crs_defn contract for one config
// level (provider or layer).
type CRS struct {
	// SRID is the effective numeric SRID. When a crs_defn was configured at
	// this level, this is the synthetic SRID under which the PROJ.4
	// definition was registered.
	SRID int

	// Explicit reports whether the CRS was configured at this config level
	// (via srid or a non-empty crs_defn), as opposed to being inherited from
	// the fallback. Explicit values always win over source-inferred SRIDs
	// (geometry headers, gpkg_contents.srs_id, system-info projections).
	Explicit bool
}

// ResolveProvider resolves the CRS contract from a provider-level config
// dictionary, starting from the given default SRID.
func ResolveProvider(cfg dict.Dicter, fallback int) (CRS, error) {
	return resolve(cfg, fallback, "provider")
}

// ResolveLayer resolves the CRS contract from a layer-level config
// dictionary on top of a fallback that has already been through the
// provider-level resolution and any source auto-detection.
func ResolveLayer(cfg dict.Dicter, fallback int) (CRS, error) {
	return resolve(cfg, fallback, "layer")
}

func resolve(cfg dict.Dicter, fallback int, level string) (CRS, error) {
	if cfg == nil {
		return CRS{SRID: fallback}, nil
	}

	srid := fallback
	explicit := false
	var err error

	// an explicitly configured numeric srid beats any source-inferred value
	if v, ok := cfg.Interface(KeySRID); ok && v != nil {
		explicit = true
	}
	if srid, err = cfg.Int(KeySRID, &srid); err != nil {
		return CRS{}, fmt.Errorf("invalid %v %v: %w", level, KeySRID, err)
	}

	// crs_defn wins over the numeric srid on the same level
	defnDefault := ""
	defn, err := cfg.String(KeyCRSDefn, &defnDefault)
	if err != nil {
		return CRS{}, fmt.Errorf("invalid %v %v: %w", level, KeyCRSDefn, err)
	}
	if strings.TrimSpace(defn) != "" {
		code, rerr := basic.RegisterProj4Defn(defn)
		if rerr != nil {
			return CRS{}, fmt.Errorf("invalid %v %v: %w", level, KeyCRSDefn, rerr)
		}
		srid = int(code)
		explicit = true
		log.Infof("registered %v %v as synthetic srid %v", level, KeyCRSDefn, code)
	}

	return CRS{SRID: srid, Explicit: explicit}, nil
}

// ApplySystemInfoCRS applies a source-provided projection (e.g. the
// MapplGIS LayerInfo projection blob carried by MOS system-info records) on
// top of an already resolved CRS. It is the shared contract used by all
// providers that consume system-info blobs, so the projection registration
// and explicit-CRS precedence behave identically everywhere:
//
//   - when the CRS was configured explicitly (via srid or crs_defn at either
//     the provider or layer level), the source projection is ignored;
//   - otherwise the projection is registered as a synthetic SRID via
//     basic.RegisterProj4Defn and becomes the effective SRID.
//
// Returns the effective SRID and whether the projection was applied.
func ApplySystemInfoCRS(currentSRID int, explicit bool, projection string) (int, bool, error) {
	if strings.TrimSpace(projection) == "" {
		return currentSRID, false, nil
	}
	if explicit {
		return currentSRID, false, nil
	}
	code, err := basic.RegisterProj4Defn(projection)
	if err != nil {
		return currentSRID, false, fmt.Errorf("invalid system info projection %q: %w", projection, err)
	}
	log.Infof("registered system info projection %q as synthetic srid %v", projection, code)
	return int(code), true, nil
}
