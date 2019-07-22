package register

import (
	"fmt"
	"html"
	"log"
	"regexp"
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
// note that we are pulling the full config file to get both providers (to check if auto) and maps

func Maps(a *atlas.Atlas, conf config.Config, providers map[string]provider.Tiler) error {

	maps := conf.Maps

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
			fmt.Println("printing provider:", provider)

			// determine if layers should automatically be created from provider regex
			auto := false
			for _, p := range conf.Providers {
				if p["name"] == providerLayer[0] && p["auto"] == true {
					auto = true
				}
			}

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
			// this must be an array because auto providers may have multiple layers that match regex
			var provLayers []string
			var layerGeomType tegola.Geometry
			for i, info := range layerInfos {
				// check if provider layers are automatically generated
				if auto {
					// regex to check against. ^ to match only beginning of phrase
					exp := fmt.Sprintf("^%v", providerLayer[1])
					// if * (wildcard), select all
					if exp == "^*" {
						provLayers = append(provLayers, info.Name())
						found = true
						layerGeomType = info.GeomType()
					} else {
						// must compile the regex first before testing phrase
						r, err := regexp.Compile(exp)
						if err != nil {
							log.Printf("Error when parsing regex (layer: %v): %v", info.Name(), err)
						} else {
							// if regex matches, push provider layer. Note that there is no break--we need to find all layers that match
							if r.MatchString(info.Name()) {
								provLayers = append(provLayers, info.Name())
								found = true
								// because this is the same for all auto provider layers, we don't need to worry about which loop we are on
								layerGeomType = info.GeomType()
							}
						}
					}

				} else {
					if layerInfos[i].Name() == providerLayer[1] {
						found = true
						provLayers = append(provLayers, providerLayer[1])
						// read the layerGeomType
						layerGeomType = layerInfos[i].GeomType()
						break

					}
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

			// for each provider layer, get map name
			// this is a loop to capture auto provider layers with multiple layers that match regex
			for _, name := range provLayers {
				var lname string
				if auto {
					lname = name
				} else {
					lname = string(l.Name)
				}

				// add our layer to our layers slice
				newMap.Layers = append(newMap.Layers, atlas.Layer{
					Name:              lname,
					ProviderLayerName: name,
					MinZoom:           minZoom,
					MaxZoom:           maxZoom,
					Provider:          provider,
					DefaultTags:       defaultTags,
					GeomType:          layerGeomType,
					DontSimplify:      bool(l.DontSimplify),
					DontClip:          bool(l.DontClip),
				})
			}
		}

		a.AddMap(newMap)
	}

	return nil
}
