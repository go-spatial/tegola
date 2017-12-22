package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/dimfeld/httptreemux"
	"github.com/dustin/go-humanize"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/atlas"
	"github.com/terranodo/tegola/mvt"
)

type HandleMapZXY struct {
	//	required
	mapName string
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
func (req *HandleMapZXY) parseURI(r *http.Request) error {
	var err error

	params := httptreemux.ContextParams(r.Context())

	//	set map name
	req.mapName = params["map_name"]

	//	parse our URL vals to ints
	z := params["z"]
	req.z, err = strconv.Atoi(z)
	if err != nil {
		log.Printf("invalid Z value (%v)", z)
		return fmt.Errorf("invalid Z value (%v)", z)
	}
	if req.z < 0 {
		log.Printf("invalid Z value (%v)", req.z)
		return fmt.Errorf("negative zoom levels are not allowed")
	}

	x := params["x"]
	req.x, err = strconv.Atoi(x)
	if err != nil {
		log.Printf("invalid X value (%v)", x)
		return fmt.Errorf("invalid X value (%v)", x)
	}

	//	trim the "y" param in the url in case it has an extension
	y := params["y"]
	yParts := strings.Split(y, ".")
	req.y, err = strconv.Atoi(yParts[0])
	if err != nil {
		log.Printf("invalid Y value (%v)", y)
		return fmt.Errorf("invalid Y value (%v)", y)
	}

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
	//	check http verb
	switch r.Method {
	//	preflight check for CORS request
	case "OPTIONS":
		//	TODO: how configurable do we want the CORS policy to be?
		//	set CORS header
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusNoContent)

		//	options call does not have a body
		w.Write(nil)
		return

	//	tile request
	case "GET":

		//	parse our URI
		if err := req.parseURI(r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		//	lookup our Map
		m, err := atlas.GetMap(req.mapName)
		if err != nil {
			log.Printf("map (%v) not configured. check your config file", req.mapName)
			http.Error(w, "map ("+req.mapName+") not configured. check your config file", http.StatusBadRequest)
			return
		}

		//	new tile
		tile := tegola.Tile{
			Z: req.z,
			X: req.x,
			Y: req.y,
		}

		//	filter down the layers we need for this zoom
		m = m.DisableAllLayers().EnableLayersByZoom(tile.Z)

		//	check for the debug query string
		if req.debug {
			m = m.EnableDebugLayers()
		}

		pbyte, err := m.Encode(r.Context(), tile)
		if err != nil {
			switch err {
			case mvt.ErrCanceled:
				//	TODO: add debug logs
				return
			case context.Canceled:
				//	TODO: add debug logs
				return
			default:
				errMsg := fmt.Sprintf("Error marshalling tile: %v", err)
				log.Printf(errMsg)
				http.Error(w, errMsg, http.StatusInternalServerError)
				return
			}
		}

		//	TODO: how configurable do we want the CORS policy to be?
		//	set CORS header
		w.Header().Add("Access-Control-Allow-Origin", "*")

		//	mimetype for protocol buffers
		w.Header().Add("Content-Type", "application/x-protobuf")

		w.Write(pbyte)

		//	check for tile size warnings
		if len(pbyte) > MaxTileSize {
			log.Printf("tile z:%v, x:%v, y:%v is rather large - %v", tile.Z, tile.X, tile.Y, humanize.Bytes(uint64(len(pbyte))))
		}

		//	log the request
		L.Log(logItem{
			X:         tile.X,
			Y:         tile.Y,
			Z:         tile.Z,
			RequestIP: r.RemoteAddr,
		})

	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
}
