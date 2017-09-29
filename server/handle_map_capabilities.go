package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/dimfeld/httptreemux"
	"github.com/terranodo/tegola"
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
		var rScheme = scheme(r)

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
			Scheme:      tilejson.SchemeXYZ,
			TileJSON:    tilejson.Version,
			Version:     "1.0.0",
			Grids:       make([]string, 0),
			Data:        make([]string, 0),
		}

		//	parse our query string
		var query = r.URL.Query()

		//	determing the min and max zoom for this map
		for i, l := range m.Layers {
			var tileURL = fmt.Sprintf("%v%v/maps/%v/%v/{z}/{x}/{y}.pbf", rScheme, hostName(r), req.mapName, l.Name)

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
			if tileJSON.MaxZoom < l.MaxZoom {
				tileJSON.MaxZoom = l.MaxZoom
			}

			tiles := fmt.Sprintf("%v%v/maps/%v/%v/{z}/{x}/{y}.pbf", rScheme, hostName(r), req.mapName, l.Name)
			if r.URL.Query().Get("debug") != "" {
				tiles = tiles + "?debug=true"
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

			switch l.GeomType.(type) {
			case tegola.Point, tegola.MultiPoint:
				layer.GeometryType = tilejson.GeomTypePoint
			case tegola.LineString, tegola.MultiLine:
				layer.GeometryType = tilejson.GeomTypeLine
			case tegola.Polygon, tegola.MultiPolygon:
				layer.GeometryType = tilejson.GeomTypePolygon
			default:
				layer.GeometryType = tilejson.GeomTypeUnknown
			}

			//	add our layer to our tile layer response
			tileJSON.VectorLayers = append(tileJSON.VectorLayers, layer)
		}

		//	if we have a debug param add it to our URLs
		if query.Get("debug") == "true" {
			//	build the layer details
			debugTileOutline := tilejson.VectorLayer{
				Version: 2,
				Extent:  4096,
				ID:      "debug-tile-outline",
				Name:    "debug-tile-outline",
				MinZoom: 0,
				MaxZoom: MaxZoom,
				Tiles: []string{
					fmt.Sprintf("%v%v/maps/%v/%v/{z}/{x}/{y}.pbf?debug=true", rScheme, hostName(r), m.Name, "debug-tile-outline"),
				},
				GeometryType: tilejson.GeomTypeLine,
			}

			//	add our layer to our tile layer response
			tileJSON.VectorLayers = append(tileJSON.VectorLayers, debugTileOutline)

			debugTileCenter := tilejson.VectorLayer{
				Version: 2,
				Extent:  4096,
				ID:      "debug-tile-center",
				Name:    "debug-tile-center",
				MinZoom: 0,
				MaxZoom: MaxZoom,
				Tiles: []string{
					fmt.Sprintf("%v%v/maps/%v/%v/{z}/{x}/{y}.pbf?debug=true", rScheme, hostName(r), m.Name, "debug-tile-center"),
				},
				GeometryType: tilejson.GeomTypePoint,
			}

			//	add our layer to our tile layer response
			tileJSON.VectorLayers = append(tileJSON.VectorLayers, debugTileCenter)
		}

		tileURL := fmt.Sprintf("%v%v/maps/%v/{z}/{x}/{y}.pbf", rScheme, hostName(r), req.mapName)

		if r.URL.Query().Get("debug") == "true" {
			tileURL += "?debug=true"
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
