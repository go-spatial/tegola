package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/dimfeld/httptreemux"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/maths"
)

type HandleMapLayerZXY struct {
	// required
	mapName string
	// optional
	layerName string
	// zoom
	z uint
	// row
	x uint
	// column
	y uint
	// the requests extension (i.e. pbf or json)
	// defaults to "pbf"
	extension string
	// debug
	debug bool
	// the Atlas to use, nil (default) is the default atlas
	Atlas *atlas.Atlas
}

// parseURI reads the request URI and extracts the various values for the request
func (req *HandleMapLayerZXY) parseURI(r *http.Request) error {
	var err error

	params := httptreemux.ContextParams(r.Context())

	// set map name
	req.mapName = params["map_name"]
	req.layerName = params["layer_name"]

	var placeholder uint64

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

	// trim the "y" param in the url in case it has an extension
	y := params["y"]
	yParts := strings.Split(y, ".")
	placeholder, err = strconv.ParseUint(yParts[0], 10, 32)
	if err != nil || placeholder > maxXYatZ {
		log.Warnf("invalid Y value (%v)", yParts[0])
		return fmt.Errorf("invalid Y value (%v)", yParts[0])
	}

	req.y = uint(placeholder)

	// check if we have a file extension
	if len(yParts) > 2 {
		req.extension = yParts[len(yParts)-1]
	} else {
		req.extension = "pbf"
	}

	// check for debug request
	if r.URL.Query().Get("debug") == "true" {
		req.debug = true
	}

	return nil
}

func logAndError(w http.ResponseWriter, code int, format string, vals ...interface{}) {
	msg := fmt.Sprintf(format, vals)
	log.Info(msg)
	http.Error(w, msg, code)
}

// URI scheme: /maps/:map_name/:layer_name/:z/:x/:y
// map_name - map name in the config file
// layer_name - name of the single map layer to render
// z, x, y - tile coordinates as described in the Slippy Map Tilenames specification
// 	z - zoom level
// 	x - row
// 	y - column
func (req HandleMapLayerZXY) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// parse our URI
	if err := req.parseURI(r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// lookup our Map
	m, err := req.Atlas.Map(req.mapName)
	if err != nil {
		errMsg := fmt.Sprintf("map (%v) not configured. check your config file", req.mapName)
		log.Errorf(errMsg)
		http.Error(w, errMsg, http.StatusNotFound)
		return
	}

	// filter down the layers we need for this zoom
	m = m.FilterLayersByZoom(req.z)
	if len(m.Layers) == 0 {
		logAndError(w, http.StatusNotFound, "map (%v) has no layers, at zoom %v", req.mapName, req.z)
		return
	}

	if req.layerName != "" {
		m = m.FilterLayersByName(req.layerName)
		if len(m.Layers) == 0 {
			logAndError(w, http.StatusNotFound, "map (%v) has no layers, for LayerName %v at zoom %v", req.mapName, req.layerName, req.z)
			return
		}
	}

	tile := slippy.NewTile(req.z, req.x, req.y)

	{
		// Check to see that the zxy is within the bounds of the map.
		textent := tile.Extent4326()
		if !m.Bounds.Contains(textent) {
			logAndError(w, http.StatusNotFound, "map (%v -- %v) does not contains tile at %v/%v/%v -- %v", req.mapName, m.Bounds, req.z, req.x, req.y, textent)
			return
		}
	}

	// check for the debug query string
	if req.debug {
		m = m.AddDebugLayers()
	}

	pbyte, err := m.Encode(r.Context(), tile)
	if err != nil {
		switch err {
		case context.Canceled:
			// TODO: add debug logs
			return
		default:
			errMsg := fmt.Sprintf("error marshalling tile: %v", err)
			log.Error(errMsg)
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}
	}

	// mimetype for protocol buffers
	w.Header().Add("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	w.Write(pbyte)

	// check for tile size warnings
	if len(pbyte) > MaxTileSize {
		log.Infof("tile z:%v, x:%v, y:%v is rather large - %v", req.z, req.x, req.y, len(pbyte))
	}
}
