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
	Bounds       [4]float64          `json:"bounds"`
	Center       [3]float64          `json:"center"`
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
	//	CORS preflight
	case "OPTIONS":
		//	TODO: how configurable do we want the CORS policy to be?
		//	set CORS header
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusNoContent)

		//	options call does not have a body
		w.Write(nil)
		return

	case "GET":
		//	new capabilities struct
		var capabilities Capabilities
		capabilities.Version = Version

		//	iterate our registered maps
		for _, m := range maps {
			//	build the map details
			cMap := CapabilitiesMap{
				Name:   m.Name,
				Bounds: m.Bounds,
				Center: m.Center,
				Tiles: []string{
					fmt.Sprintf("%v%v/maps/%v/{z}/{x}/{y}.pbf", r.URL.Scheme, r.Host, m.Name),
				},
				Capabilities: fmt.Sprintf("%v%v/capabilities/%v.json", r.URL.Scheme, r.Host, m.Name),
			}

			for _, layer := range m.Layers {
				//	build the layer details
				cLayer := CapabilitiesLayer{
					Name: layer.Name,
					Tiles: []string{
						fmt.Sprintf("%v%v/maps/%v/%v/{z}/{x}/{y}.pbf", r.URL.Scheme, r.Host, m.Name, layer.Name),
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
