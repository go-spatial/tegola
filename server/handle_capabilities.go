package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Capabilities struct {
	Version string            `json:"version"`
	Maps    []CapabilitiesMap `json:"maps"`
}

type CapabilitiesMap struct {
	Name         string              `json:"name"`
	Tiles        []string            `json:"tiles"`
	Capabilities string              `json:"capabilities"`
	Layers       []CapabilitiesLayer `json:"layers"`
}

type CapabilitiesLayer struct {
	Name    string   `json:"name"`
	Tiles   []string `json:"tiles"`
	MinZoom int      `json:"minzoom"`
	MaxZoom int      `json:"maxzoom"`
}

type HandleCapabilities struct{}

func (req HandleCapabilities) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case "GET":
		//	new capabilities struct
		var capabilities Capabilities
		capabilities.Version = Version

		//	iterate our registered maps
		for mapName, m := range maps {
			//	build the map details
			cMap := CapabilitiesMap{
				Name: mapName,
				Tiles: []string{
					fmt.Sprintf("%v%v/maps/%v/{z}/{x}/{y}.pbf", r.URL.Scheme, r.Host, mapName),
				},
				Capabilities: fmt.Sprintf("%v%v/capabilities/%v.json", r.URL.Scheme, r.Host, mapName),
			}

			for _, layer := range m {
				//	build the layer details
				cLayer := CapabilitiesLayer{
					Name: layer.Name,
					Tiles: []string{
						fmt.Sprintf("%v%v/maps/%v/%v/{z}/{x}/{y}.pbf", r.URL.Scheme, r.Host, mapName, layer.Name),
					},
					MinZoom: layer.MinZoom,
					MaxZoom: layer.MaxZoom,
				}

				//	add the layer to the map
				cMap.Layers = append(cMap.Layers, cLayer)
			}

			//	add the map to the capabilities struct
			capabilities.Maps = append(capabilities.Maps, cMap)
		}

		//	setup a new json encoder and encode our capabilities
		json.NewEncoder(w).Encode(capabilities)
	}
}
