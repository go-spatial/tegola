package register

import (
	"fmt"
	"reflect"

	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
)

// Features resolves only explicitly published, eligible raw provider layers.
func Features(cfg config.FeaturesConfig, providers map[string]provider.TilerUnion) (*features.Service, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, nil
	}
	sources := make([]features.CollectionSource, 0, len(cfg.Collections))
	for _, collection := range cfg.Collections {
		binding := provider.MapLayer{ProviderLayer: collection.ProviderLayer}
		name, layerName, err := binding.ProviderLayerName()
		if err != nil {
			return nil, err
		}
		tiler, ok := providers[name]
		if !ok {
			return nil, fmt.Errorf("features: collection %q provider %q is missing", collection.ID, name)
		}
		if tiler.Mvt != nil || nilFeatureSource(tiler.Std) {
			return nil, fmt.Errorf("features: collection %q requires a standard provider: %w", collection.ID, provider.ErrUnsupported)
		}
		querier, err := tiler.FeatureQuerier()
		if err != nil {
			return nil, fmt.Errorf("features: collection %q provider eligibility: %w", collection.ID, err)
		}
		layers, err := tiler.Layers()
		if err != nil {
			return nil, fmt.Errorf("features: collection %q source metadata: %w", collection.ID, err)
		}
		var layer provider.LayerInfo
		for _, candidate := range layers {
			if nilFeatureSource(candidate) || candidate.Name() != layerName {
				continue
			}
			if layer != nil {
				return nil, fmt.Errorf("features: collection %q source layer is ambiguous", collection.ID)
			}
			layer = candidate
		}
		if layer == nil {
			return nil, fmt.Errorf("features: collection %q source layer %q is missing", collection.ID, layerName)
		}
		sources = append(sources, features.CollectionSource{ID: string(collection.ID), Title: string(collection.Title), Description: string(collection.Description), Layer: layer, Querier: querier})
	}
	return features.NewService(sources)
}

func nilFeatureSource(value any) bool {
	if value == nil {
		return true
	}
	switch reflect.ValueOf(value).Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflect.ValueOf(value).IsNil()
	}
	return false
}
