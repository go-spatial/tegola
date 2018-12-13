package register

import (
	"fmt"
	"html"
	"strings"

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

		// iterate our layers
		for _, l := range m.Layers {
			// split our provider name (provider.layer) into [provider,layer]
			providerLayer := strings.Split(string(l.ProviderLayer), ".")

			// we're expecting two params in the provider layer definition
			if len(providerLayer) != 2 {
				return ErrProviderLayerInvalid{
					ProviderLayer: string(l.ProviderLayer),
					Map:           string(m.Name),
				}
			}

			// lookup our proivder
			provider, ok := providers[providerLayer[0]]
			if !ok {
				return ErrProviderNotFound{providerLayer[0]}
			}

			// read the provider's layer names
			layerInfos, err := provider.Layers()
			if err != nil {
				return ErrFetchingLayerInfo{
					Provider: providerLayer[0],
				}
			}

			// confirm our providerLayer name is registered
			var found bool
			var layerGeomType tegola.Geometry
			for i := range layerInfos {
				if layerInfos[i].Name() == providerLayer[1] {
					found = true

					// read the layerGeomType
					layerGeomType = layerInfos[i].GeomType()
					break
				}
			}
			if !found {
				return ErrProviderLayerNotRegistered{
					MapName:       string(m.Name),
					ProviderLayer: string(l.ProviderLayer),
					Provider:      providerLayer[0],
				}
			}

			var defaultTags map[string]interface{}
			if l.DefaultTags != nil {
				var ok bool
				defaultTags, ok = l.DefaultTags.(map[string]interface{})
				if !ok {
					return ErrDefaultTagsInvalid{
						ProviderLayer: string(l.ProviderLayer),
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

			// add our layer to our layers slice
			newMap.Layers = append(newMap.Layers, atlas.Layer{
				Name:              string(l.Name),
				ProviderLayerName: providerLayer[1],
				MinZoom:           minZoom,
				MaxZoom:           maxZoom,
				Provider:          provider,
				DefaultTags:       defaultTags,
				GeomType:          layerGeomType,
				DontSimplify:      bool(l.DontSimplify),
				DontClip:      	   bool(l.DontClip),
			})
		}

		a.AddMap(newMap)
	}

	return nil
}
