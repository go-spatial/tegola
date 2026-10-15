package config

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/alexeydott/tegola/internal/env"
	"github.com/alexeydott/tegola/provider"
)

// FeaturesConfig explicitly publishes raw provider layers; the default is disabled.
type FeaturesConfig struct {
	MaxResponseBytes *env.Int                  `toml:"max_response_bytes"`
	QueryTimeoutMS   *env.Int                  `toml:"query_timeout_ms"`
	Enabled          bool                      `toml:"enabled"`
	BasePath         env.String                `toml:"basepath"`
	DefaultLimit     *env.Int                  `toml:"default_limit"`
	MaxLimit         *env.Int                  `toml:"max_limit"`
	Title            env.String                `toml:"title"`
	Description      env.String                `toml:"description"`
	Collections      []FeatureCollectionConfig `toml:"collections"`
	// Write gates Part 4 and WFS-T mutations. Disabled by default;
	// everything stays read-only unless explicitly listed here.
	Write FeaturesWriteConfig `toml:"write"`
}

type FeatureCollectionConfig struct {
	ID            env.String `toml:"id"`
	ProviderLayer env.String `toml:"provider_layer"`
	Title         env.String `toml:"title"`
	Description   env.String `toml:"description"`
}

// Resolved returns detached settings with defaults, without changing the caller.
func (f FeaturesConfig) Resolved() FeaturesConfig {
	if f.BasePath == "" {
		f.BasePath = "/features"
	}
	defaultLimit, maxLimit := env.Int(100), env.Int(10000)
	if f.DefaultLimit != nil {
		defaultLimit = *f.DefaultLimit
	}
	if f.MaxLimit != nil {
		maxLimit = *f.MaxLimit
	}
	responseBytes, timeoutMS := env.Int(16<<20), env.Int(30000)
	if f.MaxResponseBytes != nil {
		responseBytes = *f.MaxResponseBytes
	}
	if f.QueryTimeoutMS != nil {
		timeoutMS = *f.QueryTimeoutMS
	}
	f.MaxResponseBytes, f.QueryTimeoutMS = &responseBytes, &timeoutMS
	f.DefaultLimit, f.MaxLimit = &defaultLimit, &maxLimit
	f.Collections = append([]FeatureCollectionConfig(nil), f.Collections...)
	f.Write = f.Write.Resolved()
	return f
}

func (f FeaturesConfig) Validate() error {
	f = f.Resolved()
	if *f.MaxResponseBytes < 1024 {
		return fmt.Errorf("features: max_response_bytes must be at least 1024")
	}
	if *f.QueryTimeoutMS <= 0 || int64(*f.QueryTimeoutMS) > math.MaxInt64/int64(time.Millisecond) {
		return fmt.Errorf("features: query_timeout_ms is outside supported positive duration")
	}
	if err := ValidateFeatureBasePath(string(f.BasePath)); err != nil {
		return err
	}
	if *f.DefaultLimit <= 0 || *f.MaxLimit <= 0 || *f.DefaultLimit > *f.MaxLimit {
		return fmt.Errorf("features: limits must be positive and default_limit must not exceed max_limit")
	}
	if f.Enabled && len(f.Collections) == 0 {
		return fmt.Errorf("features: enabled publication requires collections")
	}
	ids := make(map[string]bool, len(f.Collections))
	for _, collection := range f.Collections {
		id := string(collection.ID)
		if err := ValidateFeatureCollectionID(id); err != nil {
			return err
		}
		if ids[id] {
			return fmt.Errorf("features: duplicate collection ID %q", id)
		}
		ids[id] = true
		binding := provider.MapLayer{ProviderLayer: collection.ProviderLayer}
		name, layer, err := binding.ProviderLayerName()
		if err != nil {
			return fmt.Errorf("features: collection %q source binding: %w", id, err)
		}
		if strings.TrimSpace(name) == "" || strings.TrimSpace(layer) == "" {
			return fmt.Errorf("features: collection %q has an empty source binding", id)
		}
	}
	if err := f.Write.Validate(ids); err != nil {
		return err
	}
	return nil
}

// ValidateFeatureCollectionID checks literal unreserved ASCII IDs without normalization.
func ValidateFeatureCollectionID(id string) error {
	if id == "" || id == "." || id == ".." {
		return fmt.Errorf("features: invalid collection ID %q", id)
	}
	for i := 0; i < len(id); i++ {
		if !featureUnreserved(id[i]) {
			return fmt.Errorf("features: invalid collection ID %q", id)
		}
	}
	return nil
}

// ValidateFeatureBasePath checks config syntax; viewer collisions are checked by server.
func ValidateFeatureBasePath(path string) error {
	if !strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return fmt.Errorf("features: invalid basepath %q", path)
	}
	segments := strings.Split(path[1:], "/")
	for _, segment := range segments {
		if err := ValidateFeatureCollectionID(segment); err != nil {
			return fmt.Errorf("features: invalid basepath %q: %w", path, err)
		}
	}
	switch segments[0] {
	case "maps", "capabilities", "metrics":
		return fmt.Errorf("features: reserved basepath %q", path)
	}
	return nil
}

func featureUnreserved(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~'
}
