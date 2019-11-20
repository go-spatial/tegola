package server

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/dimfeld/httptreemux"

	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/mapbox/tilejson"
)

type HandleMapCapabilities struct {
	// required
	mapName string
	// the requests extension defaults to "json"
	extension string
}

// ServeHTTP returns details about a map according to the
// tileJSON spec (https://github.com/mapbox/tilejson-spec/tree/master/2.1.0)
//
// URI scheme: /capabilities/:map_name.json
// map_name - map name in the config file
func (req HandleMapCapabilities) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	params := httptreemux.ContextParams(r.Context())

	// read the map_name value from the request
	mapName := params["map_name"]
	mapNameParts := strings.Split(mapName, ".")

	req.mapName = mapNameParts[0]
	// check if we have a provided extension
	if len(mapNameParts) > 2 {
		req.extension = mapNameParts[len(mapNameParts)-1]
	} else {
		req.extension = "json"
	}

	// lookup our Map
	m, err := atlas.GetMap(req.mapName)
	if err != nil {
		log.Printf("map (%v) not configured. check your config file", req.mapName)
		http.Error(w, "map ("+req.mapName+") not configured. check your config file", http.StatusBadRequest)
		return
	}

	tileJSON := tilejson.TileJSON{
		Attribution: &m.Attribution,
		Bounds:      m.Bounds.Extent(),
		Center:      m.Center,
		Format:      "pbf",
		Name:        &m.Name,
		Scheme:      tilejson.SchemeXYZ,
		TileJSON:    tilejson.Version,
		Version:     "1.0.0",
		Grids:       make([]string, 0),
		Data:        make([]string, 0),
	}

	// parse our query string
	var query = r.URL.Query()

	debugQuery := url.Values{}
	// if we have a debug param add it to our URLs
	if query.Get("debug") == "true" {
		debugQuery.Set("debug", "true")

		// update our map to include the debug layers
		m = m.AddDebugLayers()
	}

	tileJSON.SetVectorLayers(m.Layers)
	//Build tiles urls
	for i, layer := range tileJSON.VectorLayers {
		tileJSON.VectorLayers[i].Tiles = []string{
			buildCapabilitiesURL(r, []string{"maps", req.mapName, layer.ID, "{z}/{x}/{y}.pbf"}, debugQuery),
		}
	}

	tileURL := buildCapabilitiesURL(r, []string{"maps", req.mapName, "{z}/{x}/{y}.pbf"}, debugQuery)

	// build our URL scheme for the tile grid
	tileJSON.Tiles = append(tileJSON.Tiles, tileURL)

	// content type
	w.Header().Add("Content-Type", "application/json")

	// cache control headers (no-cache)
	w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Add("Pragma", "no-cache")
	w.Header().Add("Expires", "0")

	if err = json.NewEncoder(w).Encode(tileJSON); err != nil {
		log.Printf("error encoding tileJSON for map (%v)", req.mapName)
	}
}
