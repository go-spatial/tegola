package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/dimfeld/httptreemux"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/atlas"
	"github.com/terranodo/tegola/geom/slippy"
	"github.com/terranodo/tegola/internal/log"
)

type HandleMapLayerZXY struct {
	//	required
	mapName string
	//	optional
	layerName string
	//	zoom
	z int
	//	row
	x int
	//	column
	y int
	//	the requests extension (i.e. pbf or json)
	//	defaults to "pbf"
	extension string
	//	debug
	debug bool
}

//	parseURI reads the request URI and extracts the various values for the request
func (req *HandleMapLayerZXY) parseURI(r *http.Request) error {
	var err error

	params := httptreemux.ContextParams(r.Context())

	//	set map name
	req.mapName = params["map_name"]
	req.layerName = params["layer_name"]

	//	parse our URL vals to ints
	z := params["z"]
	req.z, err = strconv.Atoi(z)
	if err != nil || req.z < 0 {
		log.Warnf("invalid Z value (%v)", z)
		return fmt.Errorf("invalid Z value (%v)", z)
	}

	x := params["x"]
	req.x, err = strconv.Atoi(x)
	if err != nil || req.x < 0 {
		log.Warnf("invalid X value (%v)", x)
		return fmt.Errorf("invalid X value (%v)", x)
	}

	//	trim the "y" param in the url in case it has an extension
	y := params["y"]
	yParts := strings.Split(y, ".")
	req.y, err = strconv.Atoi(yParts[0])
	if err != nil || req.y < 0 {
		log.Warnf("invalid Y value (%v)", y)
		return fmt.Errorf("invalid Y value (%v)", y)
	}

	//	check if we have a file extension
	if len(yParts) > 2 {
		req.extension = yParts[len(yParts)-1]
	} else {
		req.extension = "pbf"
	}

	//	check for debug request
	if r.URL.Query().Get("debug") == "true" {
		req.debug = true
	}

	return nil
}

//	URI scheme: /maps/:map_name/:layer_name/:z/:x/:y
//	map_name - map name in the config file
//	layer_name - name of the single map layer to render
//	z, x, y - tile coordinates as described in the Slippy Map Tilenames specification
//		z - zoom level
//		x - row
//		y - column
func (req HandleMapLayerZXY) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//	parse our URI
	if err := req.parseURI(r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//	lookup our Map
	m, err := atlas.GetMap(req.mapName)
	if err != nil {
		errMsg := fmt.Sprintf("map (%v) not configured. check your config file", req.mapName)
		log.Errorf(errMsg)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	tile := slippy.NewTile(uint64(req.z), uint64(req.x), uint64(req.y), TileBuffer, tegola.WebMercator)

	//	filter down the layers we need for this zoom
	m = m.FilterLayersByZoom(req.z).FilterLayersByName(req.layerName)

	//	check for the debug query string
	if req.debug {
		m = m.AddDebugLayers()
	}

	pbyte, err := m.Encode(r.Context(), tile)
	if err != nil {
		switch err {
		case context.Canceled:
			//	TODO: add debug logs
			return
		default:
			errMsg := fmt.Sprintf("error marshalling tile: %v", err)
			log.Error(errMsg)
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}
	}

	//	mimetype for protocol buffers
	w.Header().Add("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	w.Write(pbyte)

	//	check for tile size warnings
	if len(pbyte) > MaxTileSize {
		log.Infof("tile z:%v, x:%v, y:%v is rather large - %v", req.z, req.x, req.y, len(pbyte))
	}
	/*
		//	log the request
		L.Log(logItem{
			X:         tile.X,
			Y:         tile.Y,
			Z:         tile.Z,
			RequestIP: r.RemoteAddr,
		})
	*/
}
