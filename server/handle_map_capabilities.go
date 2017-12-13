package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/dimfeld/httptreemux"
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/atlas"
	"github.com/terranodo/tegola/mapbox/tilejson"
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
	m, err := atlas.GetMap(req.mapName)
	if err != nil {
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

	var debugQuery string
	//	if we have a debug param add it to our URLs
	if query.Get("debug") == "true" {
		debugQuery = "?debug=true"

		//	update our map to include the debug layers
		m = m.EnableDebugLayers()
	}

	for i := range m.Layers {
		//	skip disabled layers
		if m.Layers[i].Disabled {
			continue
		}

		//	check if the layer already exists in our slice. this can happen if the config
		//	is using the "name" param for a layer to override the providerLayerName
		var skip bool
		for j := range tileJSON.VectorLayers {
			if tileJSON.VectorLayers[j].ID == m.Layers[i].MVTName() {
				//	we need to use the min and max of all layers with this name
				if tileJSON.VectorLayers[j].MinZoom > m.Layers[i].MinZoom {
					tileJSON.VectorLayers[j].MinZoom = m.Layers[i].MinZoom
				}

				if tileJSON.VectorLayers[j].MaxZoom < m.Layers[i].MaxZoom {
					tileJSON.VectorLayers[j].MaxZoom = m.Layers[i].MaxZoom
				}

				skip = true
				break
			}
		}
		//	entry for layer already exists. move on
		if skip {
			continue
		}

		//	the first layer sets the initial min / max otherwise they default to 0/0
		if len(tileJSON.VectorLayers) == 0 {
			tileJSON.MinZoom = m.Layers[i].MinZoom
			tileJSON.MaxZoom = m.Layers[i].MaxZoom
		}

		//	check if we have a min zoom lower then our current min
		if tileJSON.MinZoom > m.Layers[i].MinZoom {
			tileJSON.MinZoom = m.Layers[i].MinZoom
		}

		//	check if we have a max zoom higher then our current max
		if tileJSON.MaxZoom < m.Layers[i].MaxZoom {
			tileJSON.MaxZoom = m.Layers[i].MaxZoom
		}

		//	build our vector layer details
		layer := tilejson.VectorLayer{
			Version: 2,
			Extent:  4096,
			ID:      m.Layers[i].MVTName(),
			Name:    m.Layers[i].MVTName(),
			MinZoom: m.Layers[i].MinZoom,
			MaxZoom: m.Layers[i].MaxZoom,
			Tiles: []string{
				fmt.Sprintf("%v://%v/maps/%v/%v/{z}/{x}/{y}.pbf%v", scheme(r), hostName(r), req.mapName, m.Layers[i].MVTName(), debugQuery),
			},
		}

		switch m.Layers[i].GeomType.(type) {
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

	tileURL := fmt.Sprintf("%v://%v/maps/%v/{z}/{x}/{y}.pbf%v", scheme(r), hostName(r), req.mapName, debugQuery)

	//	build our URL scheme for the tile grid
	tileJSON.Tiles = append(tileJSON.Tiles, tileURL)

	//	content type
	w.Header().Add("Content-Type", "application/json")

	//	cache control headers (no-cache)
	w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Add("Pragma", "no-cache")
	w.Header().Add("Expires", "0")

	if err = json.NewEncoder(w).Encode(tileJSON); err != nil {
		log.Printf("error encoding tileJSON for map (%v)", req.mapName)
	}
}
