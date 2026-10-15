package register

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/mapbox/tilejson"
	"github.com/alexeydott/tegola/provider"
)

func webMercatorMapFromConfigMap(cfg provider.Map) (newMap atlas.Map) {
	newMap = atlas.NewWebMercatorMap(string(cfg.Name))
	newMap.Attribution = SanitizeAttribution(string(cfg.Attribution))
	newMap.Params = cfg.Parameters

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

func atlasLayerFromConfigLayer(cfg *provider.MapLayer, mapName string, layerProvider provider.Layerer) (layer atlas.Layer, err error) {
	// providerLayer is primary used for error reporting.
	providerLayer := string(cfg.ProviderLayer)
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
		layer.DefaultTags = cfg.DefaultTags
	}

	// if layerProvider is not a provider.Tiler this will return nil, so
	// no need to check ok, as nil is what we want here.
	layer.Provider, _ = layerProvider.(provider.Tiler)

	layer.Name = string(cfg.Name)
	layer.ProviderLayerName = layerName
	layer.DontSimplify = bool(cfg.DontSimplify)
	layer.DontClip = bool(cfg.DontClip)
	layer.DontClean = bool(cfg.DontClean)

	if cfg.MinZoom != nil {
		layer.MinZoom = uint(*cfg.MinZoom)
	}
	if cfg.MaxZoom != nil {
		layer.MaxZoom = uint(*cfg.MaxZoom)
	}
	// reject declared inverted zoom ranges (part13 P6-29). nil means unset;
	// config.Validate fills the defaults and is the full authority there.
	if cfg.MinZoom != nil && cfg.MaxZoom != nil && *cfg.MinZoom > *cfg.MaxZoom {
		return layer, config.ErrInvalidLayerZoomRange{
			MapName:       mapName,
			ProviderLayer: providerLayer,
			MinZoom:       uint(*cfg.MinZoom),
			MaxZoom:       uint(*cfg.MaxZoom),
		}
	}
	return layer, nil
}

func selectProvider(name string, mapName string, newMap *atlas.Map, providers map[string]provider.TilerUnion) (provider.Layerer, error) {
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
		// Need to see what type of provider we got.
		if prvd.Std != nil {
			return prvd.Std, nil
		}
		if prvd.Mvt == nil {
			return nil, ErrProviderNotFound{name}
		}
		if len(newMap.Layers) != 0 {
			return nil, config.ErrMixedProviders{
				Map: string(mapName),
			}
		}
		return newMap.SetMVTProvider(name, prvd.Mvt), nil
	}
	return nil, ErrProviderNotFound{name}
}

// Maps registers maps with with atlas.
func Maps(a *atlas.Atlas, maps []provider.Map, providers map[string]provider.TilerUnion) error {
	var layerer provider.Layerer

	// map names must be unique (part13 P6-29)
	seen := map[string]struct{}{}

	// iterate our maps
	for _, m := range maps {
		if _, ok := seen[string(m.Name)]; ok {
			return config.ErrMapNameDuplicate{MapName: string(m.Name)}
		}
		seen[string(m.Name)] = struct{}{}

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

			// if our map requests TileJSON v3 specifically
			// we have to validate that the provider supports it.
			// if m.TileJSONVersion is not being provided tegola determines
			// the TileJSON version for the user.
			if m.TileJSONVersion == tilejson.Version3 {
				p := providers[providerName]
				if ok, err := p.IsTileJSONV3Compatible(); !ok {
					return fmt.Errorf("%w: %s", err, providerName)
				}
			}

			// find our layer provider
			layerer, err = selectProvider(providerName, string(m.Name), &newMap, providers)
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

// Find allow HTML tag
var allowTags = regexp.MustCompile(`&lt;(a\s(.+?)|/a)&gt;`)

// Escapes HTML special characters except allow tags
func SanitizeAttribution(attribution string) string {
	result := html.EscapeString(attribution)
	tags := allowTags.FindAllString(result, -1)
	for _, tag := range tags {
		result = strings.Replace(result, tag, html.UnescapeString(tag), 1)
	}
	return result
}
