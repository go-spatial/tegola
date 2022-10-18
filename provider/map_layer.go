package provider

import (
	"fmt"
	"strings"

	"github.com/go-spatial/tegola/internal/env"
)

// MapLayer represents a the config for a layer in a map
type MapLayer struct {
	// Name is optional. If it's not defined the name of the ProviderLayer will be used.
	// Name can also be used to group multiple ProviderLayers under the same namespace.
	Name          env.String `toml:"name"`
	ProviderLayer env.String `toml:"provider_layer"`
	MinZoom       *env.Uint  `toml:"min_zoom"`
	MaxZoom       *env.Uint  `toml:"max_zoom"`
	DefaultTags   env.Dict   `toml:"default_tags"`
	// DontSimplify indicates whether feature simplification should be applied.
	// We use a negative in the name so the default is to simplify
	DontSimplify env.Bool `toml:"dont_simplify"`
	// DontClip indicates whether feature clipping should be applied.
	// We use a negative in the name so the default is to clipping
	DontClip env.Bool `toml:"dont_clip"`
	// DontClip indicates whether feature cleaning (e.g. make valid) should be applied.
	// We use a negative in the name so the default is to clean
	DontClean env.Bool `toml:"dont_clean"`
}

// ProviderLayerName returns the names of the layer and provider or an error
func (ml MapLayer) ProviderLayerName() (provider, layer string, err error) {
	// split the provider layer (syntax is provider.layer)
	plParts := strings.Split(string(ml.ProviderLayer), ".")
	if len(plParts) != 2 {
		// TODO (beymak): Properly handle the error
		return "", "", fmt.Errorf("config: invalid provider layer name (%v)", ml.ProviderLayer)
		// return "", "", ErrInvalidProviderLayerName{ProviderLayerName: string(ml.ProviderLayer)}
	}
	return plParts[0], plParts[1], nil
}

// GetName will return the user-defined Layer name from the config,
// or if the name is empty, return the name of the layer associated with
// the provider
func (ml MapLayer) GetName() (string, error) {
	if ml.Name != "" {
		return string(ml.Name), nil
	}
	_, name, err := ml.ProviderLayerName()
	return name, err
}
