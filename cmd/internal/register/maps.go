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
	"github.com/go-spatial/tegola/internal/env"
	"github.com/go-spatial/tegola/provider"
)

const (
	DefaultMapName = env.String("Default")
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

func AutoConfigMapLayers(providers map[string]provider.Tiler) []config.MapLayer {
	mapLayers := make([]config.MapLayer, len(providers))
	count := 0
	for pname := range providers {
		mapLayers[count] = config.MapLayer{
			ProviderLayer: env.String(fmt.Sprintf("%s.*", pname)),
		}
		count++
	}
	return mapLayers
}

func AutoConfigMap(providers map[string]provider.Tiler) []config.Map {
	return []config.Map{
		{
			Name:   DefaultMapName,
			Layers: AutoConfigMapLayers(providers),
		},
	}
}

// Maps registers maps with with atlas
// note that we are pulling the full config file to get both providers (to check if auto) and maps
func Maps(a *atlas.Atlas, confMaps []config.Map, providers map[string]provider.Tiler) error {

	maps := confMaps

	if len(maps) == 0 {
		maps = AutoConfigMap(providers)
	}

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

		mapLayers := m.Layers

		if len(mapLayers) == 0 {
			mapLayers = AutoConfigMapLayers(providers)
		}

		// iterate our layers
		for _, l := range mapLayers {
			// split our provider name (provider.layer) into [provider,layer]
			providerLayer := strings.SplitN(string(l.ProviderLayer), ".", 2)

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

			fmt.Println("printing provider:", provider)

			// read the provider's layer names
			layerInfos, err := provider.Layers()

			if err != nil {
				return ErrFetchingLayerInfo{
					Provider: providerLayer[0],
				}
			}

			// confirm our providerLayer name is registered
			// this must be an array because auto providers may have multiple layers that match regex
			var provLayers []string
			var layerGeomType tegola.Geometry
			var isregex bool
		LayerLoop:
			for _, info := range layerInfos {
				if providerLayer[1] == "*" {
					// return all the layers:
					provLayers = append(provLayers, info.Name())
					layerGeomType = info.GeomType()
					continue
				}
				// check to see if string contains regex
				isregex = len(providerLayer[1]) != len(regexp.QuoteMeta(providerLayer[1]))

				switch {
				case isregex:
					r, err := regexp.Compile("^" + providerLayer[1])
					if err != nil {
						log.Printf("Error when parsing regex (layer: %v): %v", info.Name(), err)
						continue LayerLoop // add a Providers label at 111
					}
					if !r.MatchString(info.Name()) {
						continue
					}
				case info.Name() == providerLayer[1]:
					// do nothing
				default:
					// ignore this layer
					continue

				}
				provLayers = append(provLayers, info.Name())
				layerGeomType = info.GeomType()
			}

			if len(provLayers) == 0 {
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

			minZoom := uint(1)
			if l.MinZoom != nil {
				minZoom = uint(*l.MinZoom)
			}

			maxZoom := uint(20)
			if l.MaxZoom != nil {
				maxZoom = uint(*l.MaxZoom)
			}

			// for each provider layer, get map name
			// this is a loop to capture auto provider layers with multiple layers that match regex
			for _, name := range provLayers {
				var lname string
				if string(l.Name) == "" || isregex == true {
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
