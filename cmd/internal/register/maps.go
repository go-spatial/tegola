package register

import (
	"html"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/mvtprovider"
	"github.com/go-spatial/tegola/provider"
)

func webMercatorMapFromConfigMap(cfg config.Map) (newMap atlas.Map) {
	newMap = atlas.NewWebMercatorMap(string(cfg.Name))
	newMap.Attribution = html.EscapeString(string(cfg.Attribution))

	// convert from env package
	for i, v := range cfg.Center {
		newMap.Center[i] = float64(v)
	}

	if len(cfg.Bounds) == 4 {
		newMap.Bounds = geom.NewExtent(
			[2]float64{float64(cfg.Bounds[0]), float64(cfg.Bounds[1])},
			[2]float64{float64(cfg.Bounds[2]), float64(cfg.Bounds[3])},
		)
	}

	if cfg.TileBuffer != nil {
		newMap.TileBuffer = uint64(*cfg.TileBuffer)
	}
	return newMap

}

func layerInfosFindByName(infos []provider.LayerInfo, name string) provider.LayerInfo {
	if len(infos) == 0 {
		return nil
	}
	for i := range infos {
		if infos[i].Name() == name {
			return infos[i]
		}
	}
	return nil
}

func atlasLayerFromConfigLayer(cfg *config.MapLayer, mapName string, layerProvider provider.Layerer) (layer atlas.Layer, err error) {
	var (
		// providerLayer is primary used for error reporting.
		providerLayer = string(cfg.ProviderLayer)
		ok            bool
	)
	// read the provider's layer names
	// don't care about the error.
	providerName, layerName, _ := cfg.ProviderLayerName()
	layerInfos, err := layerProvider.Layers()
	if err != nil {
		return layer, ErrFetchingLayerInfo{
			Provider: providerName,
			Err:      err,
		}
	}
	layerInfo := layerInfosFindByName(layerInfos, layerName)
	if layerInfo == nil {
		return layer, ErrProviderLayerNotRegistered{
			MapName:       mapName,
			ProviderLayer: providerLayer,
			Provider:      providerName,
		}
	}
	layer.GeomType = layerInfo.GeomType()

	if cfg.DefaultTags != nil {
		if layer.DefaultTags, ok = cfg.DefaultTags.(map[string]interface{}); !ok {
			return layer, ErrDefaultTagsInvalid{
				ProviderLayer: providerLayer,
			}
		}
	}

	// if layerProvider is not a provider.Tiler this will return nil, so
	// no need to check ok, as nil is what we want here.
	layer.Provider, _ = layerProvider.(provider.Tiler)

	layer.Name = string(cfg.Name)
	layer.ProviderLayerName = layerName
	layer.DontSimplify = bool(cfg.DontSimplify)
	layer.DontClip = bool(cfg.DontClip)

	if cfg.MinZoom != nil {
		layer.MinZoom = uint(*cfg.MinZoom)
	}
	if cfg.MaxZoom != nil {
		layer.MaxZoom = uint(*cfg.MaxZoom)
	}
	return layer, nil
}

func selectProvider(name string, mapName string, newMap *atlas.Map, providers map[string]provider.Tiler, mvtProviders map[string]mvtprovider.Tiler) (provider.Layerer, error) {
	if newMap.HasMVTProvider() {
		if newMap.MVTProviderName() != name {
			return nil, config.ErrMVTDifferentProviders{
				Original: newMap.MVTProviderName(),
				Current:  name,
			}
		}
		return newMap.MVTProvider(), nil
	}
	if prvd, ok := providers[name]; ok {
		return prvd, nil
	}
	if mvtprvd, ok := mvtProviders[name]; ok {
		if len(newMap.Layers) != 0 {
			return nil, config.ErrMixedProviders{
				Map: string(mapName),
			}
		}
		return newMap.SetMVTProvider(name, mvtprvd), nil
	}
	return nil, ErrProviderNotFound{name}
}

// Maps registers maps with with atlas
func Maps(a *atlas.Atlas, maps []config.Map, providers map[string]provider.Tiler, mvtProviders map[string]mvtprovider.Tiler) error {

	var (
		layerer provider.Layerer
	)

	// iterate our maps
	for _, m := range maps {
		newMap := webMercatorMapFromConfigMap(m)

		// iterate our layers
		for _, l := range m.Layers {
			providerName, _, err := l.ProviderLayerName()
			if err != nil {
				return ErrProviderLayerInvalid{
					ProviderLayer: string(l.ProviderLayer),
					Map:           string(m.Name),
				}
			}

			// find our layer provider
			layerer, err = selectProvider(providerName, string(m.Name), &newMap, providers, mvtProviders)
			if err != nil {
				return err
			}

			layer, err := atlasLayerFromConfigLayer(&l, string(m.Name), layerer)
			if err != nil {
				return err
			}
			newMap.Layers = append(newMap.Layers, layer)
		}
		a.AddMap(newMap)
	}
	return nil
}
