package config

import (
	"fmt"
	"strings"

	"github.com/alexeydott/tegola/internal/env"
)

// WFSConfig publishes WFS 1.1.0 / 2.0 endpoints. Disabled by default;
// enabling it never enables writes by itself (see FeaturesWriteConfig).
type WFSConfig struct {
	Enabled  bool       `toml:"enabled"`
	BasePath env.String `toml:"basepath"`
	// Versions lists the advertised wire versions, e.g. ["1.1.0", "2.0.0"].
	Versions []string `toml:"versions"`
}

var supportedWFSVersions = map[string]bool{
	"1.1.0": true,
	"2.0.0": true,
	"2.0.2": true,
}

// Resolved returns detached settings with defaults.
func (w WFSConfig) Resolved() WFSConfig {
	if w.BasePath == "" {
		w.BasePath = "/wfs"
	}
	if len(w.Versions) == 0 {
		w.Versions = []string{"1.1.0", "2.0.0"}
	}
	w.Versions = append([]string(nil), w.Versions...)
	for i := range w.Versions {
		w.Versions[i] = strings.TrimSpace(w.Versions[i])
	}
	return w
}

// Validate checks the WFS configuration. Write stays disabled unless
// FeaturesWriteConfig explicitly allows it per collection.
func (w WFSConfig) Validate() error {
	w = w.Resolved()
	if err := ValidateFeatureBasePath(string(w.BasePath)); err != nil {
		return fmt.Errorf("wfs: %w", err)
	}
	seen := make(map[string]bool, len(w.Versions))
	for _, v := range w.Versions {
		v = strings.TrimSpace(v)
		if !supportedWFSVersions[v] {
			return fmt.Errorf("wfs: unsupported version %q", v)
		}
		if seen[v] {
			return fmt.Errorf("wfs: duplicate version %q", v)
		}
		seen[v] = true
	}
	return nil
}

// FeaturesWriteConfig gates all mutation paths (Part 4 and WFS-T).
// Everything is read-only unless a collection is listed here AND its
// provider layer passes write admission at startup.
type FeaturesWriteConfig struct {
	Enabled     bool                    `toml:"enabled"`
	Collections []WriteCollectionConfig `toml:"collections"`
	// RequireIfMatch, when true, rejects PUT/PATCH/DELETE without an
	// If-Match header with 428 Precondition Required. Disabled by
	// default (If-Match is optional per RFC 7232).
	RequireIfMatch bool `toml:"require_if_match"`
	// AuthMode selects the write authorization profile (A04):
	//   "dev"        - anonymous writes allowed (explicitly unsafe;
	//                  for trusted networks / development only).
	//   "production" - authenticator required; anonymous principals are
	//                  denied (deny-by-default). This is the default when
	//                  write is enabled.
	AuthMode string `toml:"auth_mode"`
}

// IsProductionAuth reports whether production auth mode is active.
func (c FeaturesWriteConfig) IsProductionAuth() bool {
	// Only an explicit development profile can relax authorization, including
	// for embedded callers that have not yet run configuration validation.
	return c.AuthMode != "dev"
}

// WriteCollectionConfig allows mutation operations on one published
// collection. Operations use Part 4 vocabulary: create, replace,
// update, delete.
type WriteCollectionConfig struct {
	ID         env.String `toml:"id"`
	Operations []string   `toml:"operations"`
}

var supportedWriteOperations = map[string]bool{
	"create":  true,
	"replace": true,
	"update":  true,
	"delete":  true,
}

// Resolved returns a detached copy.
func (w FeaturesWriteConfig) Resolved() FeaturesWriteConfig {
	w.Collections = append([]WriteCollectionConfig(nil), w.Collections...)
	for i := range w.Collections {
		w.Collections[i].Operations = append([]string(nil), w.Collections[i].Operations...)
	}
	return w
}

// Validate checks the write configuration against the published
// collections. Unknown collections or operations are startup errors;
// a requested write on a non-admitted layer fails at provider
// admission, never opens a fake mutable endpoint.
func (w FeaturesWriteConfig) Validate(published map[string]bool) error {
	w = w.Resolved()
	// R02: strict auth_mode validation. Only exact "dev" or "production"
	// (or empty = production) are allowed. Typos like "prodution" or
	// "Production" must fail at startup, not silently disable protection.
	switch w.AuthMode {
	case "", "production", "dev":
		// ok
	default:
		return fmt.Errorf("features.write: unknown auth_mode %q (must be \"dev\" or \"production\")", w.AuthMode)
	}
	seen := make(map[string]bool, len(w.Collections))
	for _, c := range w.Collections {
		id := string(c.ID)
		if err := ValidateFeatureCollectionID(id); err != nil {
			return err
		}
		if seen[id] {
			return fmt.Errorf("features.write: duplicate collection %q", id)
		}
		seen[id] = true
		if !published[id] {
			return fmt.Errorf("features.write: collection %q is not published", id)
		}
		if len(c.Operations) == 0 {
			return fmt.Errorf("features.write: collection %q needs at least one operation", id)
		}
		for _, op := range c.Operations {
			if !supportedWriteOperations[strings.ToLower(op)] {
				return fmt.Errorf("features.write: collection %q has unsupported operation %q", id, op)
			}
		}
	}
	return nil
}

// AllowsOperation reports whether op (create/replace/update/delete) is
// configured for the collection.
func (w FeaturesWriteConfig) AllowsOperation(collection, op string) bool {
	if !w.Enabled {
		return false
	}
	op = strings.ToLower(op)
	for _, c := range w.Collections {
		if string(c.ID) == collection {
			for _, o := range c.Operations {
				if strings.ToLower(o) == op {
					return true
				}
			}
		}
	}
	return false
}
