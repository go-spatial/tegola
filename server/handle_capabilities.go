package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/atlas"
)

type Capabilities struct {
	Version string            `json:"version"`
	Maps    []CapabilitiesMap `json:"maps"`
}

type CapabilitiesMap struct {
	Name         string              `json:"name"`
	Attribution  string              `json:"attribution"`
	Bounds       *geom.Extent        `json:"bounds"`
	Center       [3]float64          `json:"center"`
	Tiles        []string            `json:"tiles"`
	Capabilities string              `json:"capabilities"`
	Layers       []CapabilitiesLayer `json:"layers"`
}

type CapabilitiesLayer struct {
	Name    string   `json:"name"`
	Tiles   []string `json:"tiles"`
	MinZoom uint     `json:"minzoom"`
	MaxZoom uint     `json:"maxzoom"`
}

type HandleCapabilities struct{}

func (req HandleCapabilities) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// new capabilities struct
	capabilities := Capabilities{
		Version: Version,
	}

	// parse our query string
	var query = r.URL.Query()

	// iterate our registered maps
	for _, m := range atlas.AllMaps() {
		var debugQuery string

		// if we have a debug param add it to our URLs
		if query.Get("debug") == "true" {
			debugQuery = "?debug=true"

			// update our map to include the debug layers
			m = m.AddDebugLayers()
		}

		// build the map details
		cMap := CapabilitiesMap{
			Name:        m.Name,
			Attribution: m.Attribution,
			Bounds:      m.Bounds,
			Center:      m.Center,
			Tiles: []string{
				fmt.Sprintf("%v/maps/%v/{z}/{x}/{y}.pbf%v", URLRoot(r), m.Name, debugQuery),
			},
			Capabilities: fmt.Sprintf("%v/capabilities/%v.json%v", URLRoot(r), m.Name, debugQuery),
		}

		for i := range m.Layers {
			// check if the layer already exists in our slice. this can happen if the config
			// is using the "name" param for a layer to override the providerLayerName
			var skip bool
			for j := range cMap.Layers {
				if cMap.Layers[j].Name == m.Layers[i].MVTName() {
					// we need to use the min and max of all layers with this name
					if cMap.Layers[j].MinZoom > m.Layers[i].MinZoom {
						cMap.Layers[j].MinZoom = m.Layers[i].MinZoom
					}

					if cMap.Layers[j].MaxZoom < m.Layers[i].MaxZoom {
						cMap.Layers[j].MaxZoom = m.Layers[i].MaxZoom
					}

					skip = true
					break
				}
			}
			// entry for layer already exists. move on
			if skip {
				continue
			}

			// build the layer details
			cLayer := CapabilitiesLayer{
				Name: m.Layers[i].MVTName(),
				Tiles: []string{
					fmt.Sprintf("%v/maps/%v/%v/{z}/{x}/{y}.pbf%v", URLRoot(r), m.Name, m.Layers[i].MVTName(), debugQuery),
				},
				MinZoom: m.Layers[i].MinZoom,
				MaxZoom: m.Layers[i].MaxZoom,
			}

			// add the layer to the map
			cMap.Layers = append(cMap.Layers, cLayer)
		}

		// add the map to the capabilities struct
		capabilities.Maps = append(capabilities.Maps, cMap)

		// content type
		w.Header().Add("Content-Type", "application/json")

		// cache control headers (no-cache)
		w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Add("Pragma", "no-cache")
		w.Header().Add("Expires", "0")
	}

	// setup a new json encoder and encode our capabilities
	json.NewEncoder(w).Encode(capabilities)
}
