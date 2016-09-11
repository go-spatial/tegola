package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

//	set at runtime from main
var Version string

type Capabilities struct {
	Version string            `json:"version"`
	Maps    []CapabilitiesMap `json:"maps"`
}

type CapabilitiesMap struct {
	Name   string              `json:"name"`
	URI    string              `json:"uri"`
	Layers []CapabilitiesLayer `json:"layers"`
}

type CapabilitiesLayer struct {
	Name    string `json:"name"`
	URI     string `json:"uri"`
	MinZoom int    `json:"minZoom"`
	MaxZoom int    `json:"maxZoom"`
}

type handleCapabilities struct{}

func (req handleCapabilities) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case "GET":
		//	new capabilities struct
		var capabilities Capabilities
		capabilities.Version = Version

		//	iterate our registered maps
		for name, m := range maps {
			//	build the map details
			cMap := CapabilitiesMap{
				Name: name,
				URI:  fmt.Sprintf("/maps/%v", name),
			}

			for _, layer := range m {
				//	build the layer details
				cLayer := CapabilitiesLayer{
					Name:    layer.Name,
					URI:     fmt.Sprintf("/maps/%v/%v", name, layer.Name),
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
