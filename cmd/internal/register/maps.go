package register

import (
	"fmt"
	"html"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/provider"
)

type ErrProviderLayerInvalid struct {
	ProviderLayer string
	Map           string
}

func (e ErrProviderLayerInvalid) Error() string {
	return fmt.Sprintf("invalid provider layer (%v) for map (%v)", e.ProviderLayer, e.Map)
}

type ErrProviderNotFound struct {
	Provider string
}

func (e ErrProviderNotFound) Error() string {
	return fmt.Sprintf("provider (%v) not defined", e.Provider)
}

type ErrProviderLayerNotRegistered struct {
	MapName       string
	ProviderLayer string
	Provider      string
}

func (e ErrProviderLayerNotRegistered) Error() string {
	return fmt.Sprintf("map (%v) 'provider_layer' (%v) is not registered with provider (%v)", e.MapName, e.ProviderLayer, e.Provider)
}

type ErrFetchingLayerInfo struct {
	Provider string
}

func (e ErrFetchingLayerInfo) Error() string {
	return fmt.Sprintf("error fetching layer info from provider (%v)", e.Provider)
}

type ErrDefaultTagsInvalid struct {
	ProviderLayer string
}

func (e ErrDefaultTagsInvalid) Error() string {
	return fmt.Sprintf("'default_tags' for 'provider_layer' (%v) should be a TOML table", e.ProviderLayer)
}

func initLayer(l *config.MapLayer, mapName string, layerProvider provider.Layerer) (atlas.Layer, error) {
	// read the provider's layer names
	providerName, layerName, _ := l.ProviderLayerName()
	layerInfos, err := layerProvider.Layers()
	if err != nil {
		return atlas.Layer{}, ErrFetchingLayerInfo{
			Provider: providerName,
		}
	}
	providerLayer := string(l.ProviderLayer)

	// confirm our providerLayer name is registered
	var found bool
	var layerGeomType geom.Geometry
	for i := range layerInfos {
		if layerInfos[i].Name() == layerName {
			found = true

			// read the layerGeomType
			layerGeomType = layerInfos[i].GeomType()
			break
		}
	}
	if !found {
		return atlas.Layer{}, ErrProviderLayerNotRegistered{
			MapName:       mapName,
			ProviderLayer: providerLayer,
			Provider:      providerName,
		}
	}

	var defaultTags map[string]interface{}
	if l.DefaultTags != nil {
		var ok bool
		defaultTags, ok = l.DefaultTags.(map[string]interface{})
		if !ok {
			return atlas.Layer{}, ErrDefaultTagsInvalid{
				ProviderLayer: providerLayer,
			}
		}
	}

	var minZoom uint
	if l.MinZoom != nil {
		minZoom = uint(*l.MinZoom)
	}

	var maxZoom uint
	if l.MaxZoom != nil {
		maxZoom = uint(*l.MaxZoom)
	}

	prvd, _ := layerProvider.(provider.Tiler)

	// add our layer to our layers slice
	return atlas.Layer{
		Name:              string(l.Name),
		ProviderLayerName: layerName,
		MinZoom:           minZoom,
		MaxZoom:           maxZoom,
		Provider:          prvd,
		DefaultTags:       defaultTags,
		GeomType:          layerGeomType,
		DontSimplify:      bool(l.DontSimplify),
		DontClip:          bool(l.DontClip),
	}, nil
}

// Maps registers maps with with atlas
func Maps(a *atlas.Atlas, maps []config.Map, providers map[string]provider.Tiler) error {

	// iterate our maps
	for _, m := range maps {
		newMap := atlas.NewWebMercatorMap(string(m.Name))
		newMap.Attribution = html.EscapeString(string(m.Attribution))

		// convert from env package
		centerArr := [3]float64{}
		for i, v := range m.Center {
			centerArr[i] = float64(v)
		}

		newMap.Center = centerArr

		if len(m.Bounds) == 4 {
			newMap.Bounds = geom.NewExtent(
				[2]float64{float64(m.Bounds[0]), float64(m.Bounds[1])},
				[2]float64{float64(m.Bounds[2]), float64(m.Bounds[3])},
			)
		}

		if m.TileBuffer == nil {
			newMap.TileBuffer = tegola.DefaultTileBuffer
		} else {
			newMap.TileBuffer = uint64(*m.TileBuffer)
		}

		// iterate our layers
		for _, l := range m.Layers {
			providerName, _, err := l.ProviderLayerName()
			if err != nil {
				return ErrProviderLayerInvalid{
					ProviderLayer: string(l.ProviderLayer),
					Map:           string(m.Name),
				}
			}
			// search for provider in our providers
			prvd, ok := providers[providerName]
			if !ok {
				return ErrProviderNotFound{providerName}
			}
			newLayer, err := initLayer(&l, string(m.Name), prvd)
			if err != nil {
				return err
			}
			newMap.Layers = append(newMap.Layers, newLayer)
		}
		a.AddMap(newMap)
	}

	return nil
}
