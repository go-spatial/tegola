package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/dimfeld/httptreemux"
	"github.com/dustin/go-humanize"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/geom/slippy"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/maths"
)

type HandleMapZXY struct {
	//	required
	mapName string
	//	zoom
	z uint
	//	row
	x uint
	//	column
	y uint
	//	the requests extension (i.e. pbf or json)
	//	defaults to "pbf"
	extension string
	//	debug
	debug bool
}

//	parseURI reads the request URI and extracts the various values for the request
func (req *HandleMapZXY) parseURI(r *http.Request) error {
	var err error

	params := httptreemux.ContextParams(r.Context())

	//	set map name
	req.mapName = params["map_name"]

	var placeholder uint64

	//	parse our URL vals to ints
	z := params["z"]
	placeholder, err = strconv.ParseUint(z, 10, 32)
	if err != nil || placeholder > tegola.MaxZ {
		log.Warnf("invalid Z value (%v)", z)
		return fmt.Errorf("invalid Z value (%v)", z)
	}

	req.z = uint(placeholder)
	maxXYatZ := maths.Exp2(placeholder) - 1

	x := params["x"]
	placeholder, err = strconv.ParseUint(x, 10, 32)
	if err != nil || placeholder > maxXYatZ {
		log.Warnf("invalid X value (%v)", x)
		return fmt.Errorf("invalid X value (%v)", x)
	}

	req.x = uint(placeholder)

	//	trim the "y" param in the url in case it has an extension
	y := params["y"]
	yParts := strings.Split(y, ".")
	placeholder, err = strconv.ParseUint(yParts[0], 10, 32)
	if err != nil || placeholder > maxXYatZ {
		log.Warnf("invalid Y value (%v)", y)
		return fmt.Errorf("invalid Y value (%v)", y)
	}

	req.y = uint(placeholder)

	//	check if we have a file extension
	if len(yParts) > 1 {
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

//	URI scheme: /maps/:map_name/:z/:x/:y
//	map_name - map name in the config file
//	z, x, y - tile coordinates as described in the Slippy Map Tilenames specification
//		z - zoom level
//		x - row
//		y - column
func (req HandleMapZXY) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//	parse our URI
	if err := req.parseURI(r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//	lookup our Map
	m, err := atlas.GetMap(req.mapName)
	if err != nil {
		log.Errorf("map (%v) not configured. check your config file", req.mapName)
		http.Error(w, "map ("+req.mapName+") not configured. check your config file", http.StatusBadRequest)
		return
	}

	tile := slippy.NewTile(req.z, req.x, req.y, TileBuffer, tegola.WebMercator)

	//	filter down the layers we need for this zoom
	m = m.FilterLayersByZoom(req.z)

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
			errMsg := fmt.Sprintf("Error marshalling tile: %v", err)
			log.Errorf(errMsg)
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
		log.Infof("tile z:%v, x:%v, y:%v is rather large - %v", req.z, req.x, req.y, humanize.Bytes(uint64(len(pbyte))))
	}
}
