package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/dimfeld/httptreemux"
	"github.com/terranodo/tegola/tilejson"
)

type HandleMapCapabilities struct {
	//	required
	mapName string
	//	the requests extension defaults to "json"
	extension string
}

//	returns details about a map according to the
//	tileJSON spec (https://github.com/mapbox/tilejson-spec/tree/master/2.1.0)
//
//	URI scheme: /capabilities/:map_name.json
//		map_name - map name in the config file
func (req HandleMapCapabilities) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	//	check http verb
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

	//	build payload
	case "GET":
		params := httptreemux.ContextParams(r.Context())

		mapName := params["map_name"]
		mapNameParts := strings.Split(mapName, ".")

		req.mapName = mapNameParts[0]
		//	check if we have a provided extension
		if len(mapNameParts) > 2 {
			req.extension = mapNameParts[len(mapNameParts)-1]
		} else {
			req.extension = "json"
		}

		tileJSON := tilejson.TileJSON{
			Bounds:   [4]float64{-180, -85.05112877980659, 180, 85.0511287798066},
			Format:   "pbf",
			Name:     &req.mapName,
			Scheme:   "zxy",
			TileJSON: "2.1.0",
			Version:  "1.0.0",
			Grids:    make([]string, 0),
			Data:     make([]string, 0),
		}

		//	lookup our map layers
		layers, ok := maps[req.mapName]
		if !ok {
			log.Printf("map (%v) not configured. check your config file", req.mapName)
			http.Error(w, "map ("+req.mapName+") not configured. check your config file", http.StatusBadRequest)
			return
		}

		//	determing the min and max zoom for this map
		for i, l := range layers {
			//	set our min and max using the first layer
			if i == 0 {
				tileJSON.MinZoom = l.MinZoom
				tileJSON.MaxZoom = l.MaxZoom
			}

			//	check if we have a min zoom lower then our current min
			if tileJSON.MinZoom > l.MinZoom {
				tileJSON.MinZoom = l.MinZoom
			}

			//	check if we have a max zoom higher then our current max
			if tileJSON.MinZoom < l.MaxZoom {
				tileJSON.MaxZoom = l.MaxZoom
			}

			//	build our vector layer details
			layer := tilejson.VectorLayer{
				Version: 2,
				Extent:  4096,
				ID:      l.Name,
				Name:    l.Name,
				MinZoom: l.MinZoom,
				MaxZoom: l.MaxZoom,
				Tiles: []string{
					fmt.Sprintf("%v%v/maps/%v/%v/{z}/{x}/{y}.pbf", r.URL.Scheme, r.Host, req.mapName, l.Name),
				},
			}

			//	add our layer to our tile layer response
			tileJSON.VectorLayers = append(tileJSON.VectorLayers, layer)
		}

		//	build our URL scheme for the tile grid
		tileJSON.Tiles = append(tileJSON.Tiles, fmt.Sprintf("%v%v/maps/%v/{z}/{x}/{y}.pbf", r.URL.Scheme, r.Host, req.mapName))

		//	TODO: how configurable do we want the CORS policy to be?
		//	set CORS header
		w.Header().Add("Access-Control-Allow-Origin", "*")

		//	mimetype for protocol buffers
		w.Header().Add("Content-Type", "application/json")

		if err = json.NewEncoder(w).Encode(tileJSON); err != nil {
			log.Printf("error encoding tileJSON for map (%v)", req.mapName)
		}

	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
}
