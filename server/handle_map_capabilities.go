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
		var rScheme string
		//	check if the request is http or https. the scheme is needed for the TileURLs and
		//	r.URL.Scheme can be empty if a relative request is issued from the client. (i.e. GET /foo.html)
		if r.TLS != nil {
			rScheme = "https://"
		} else {
			rScheme = "http://"
		}

		params := httptreemux.ContextParams(r.Context())

		//	read the map_name value from the request
		mapName := params["map_name"]
		mapNameParts := strings.Split(mapName, ".")

		req.mapName = mapNameParts[0]
		//	check if we have a provided extension
		if len(mapNameParts) > 2 {
			req.extension = mapNameParts[len(mapNameParts)-1]
		} else {
			req.extension = "json"
		}

		//	lookup our Map
		m, ok := maps[req.mapName]
		if !ok {
			log.Printf("map (%v) not configured. check your config file", req.mapName)
			http.Error(w, "map ("+req.mapName+") not configured. check your config file", http.StatusBadRequest)
			return
		}

		tileJSON := tilejson.TileJSON{
			Attribution: &m.Attribution,
			Bounds:      m.Bounds,
			Center:      m.Center,
			Format:      "pbf",
			Name:        &m.Name,
			Scheme:      "zxy",
			TileJSON:    tilejson.Version,
			Version:     "1.0.0",
			Grids:       make([]string, 0),
			Data:        make([]string, 0),
		}

		//	parse our query string
		var query = r.URL.Query()

		//	determing the min and max zoom for this map
		for i, l := range m.Layers {
			var tileURL = fmt.Sprintf("%v%v/maps/%v/%v/{z}/{x}/{y}.pbf", rScheme, r.Host, req.mapName, l.Name)

			//	if we have a debug param add it to our URLs
			if query.Get("debug") == "true" {
				tileURL = tileURL + "?debug=true"
			}

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
					tileURL,
				},
			}

			//	add our layer to our tile layer response
			tileJSON.VectorLayers = append(tileJSON.VectorLayers, layer)
		}

		tileURL := fmt.Sprintf("%v%v/maps/%v/{z}/{x}/{y}.pbf", rScheme, r.Host, req.mapName)

		//	if we have a debug param add it to our URLs
		if query.Get("debug") == "true" {
			tileURL = tileURL + "?debug=true"

			debugTileURL := fmt.Sprintf("%v%v/maps/%v/%v/{z}/{x}/{y}.pbf?debug=true", rScheme, r.Host, req.mapName, "debug")

			//	we also need to add a debug vector layer
			//	build our vector layer details
			layer := tilejson.VectorLayer{
				Version: 2,
				Extent:  4096,
				ID:      "debug",
				Name:    "debug",
				MinZoom: 0,
				MaxZoom: MaxZoom,
				Tiles: []string{
					debugTileURL,
				},
			}

			//	add our layer to our tile layer response
			tileJSON.VectorLayers = append(tileJSON.VectorLayers, layer)
		}

		//	build our URL scheme for the tile grid
		tileJSON.Tiles = append(tileJSON.Tiles, tileURL)

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
