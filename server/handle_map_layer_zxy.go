package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/proj"
	"github.com/go-spatial/tegola/observability"
	"github.com/go-spatial/tegola/provider"

	"github.com/dimfeld/httptreemux"
	"github.com/go-spatial/geom/encoding/mvt"
	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/maths"
)

var (
	webmercatorGrid = slippy.NewGrid(3857, 0)
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

// URI scheme: /maps/:map_name/:layer_name/:z/:x/:y?param=value
// map_name - map name in the config file
// layer_name - name of the single map layer to render
// z, x, y - tile coordinates as described in the Slippy Map Tilenames specification
//
//	z - zoom level
//	x - row
//	y - column
//
// param - configurable query parameters and their values
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
	m = m.FilterLayersByZoom(slippy.Zoom(req.z))
	if len(m.Layers) == 0 {
		msg := fmt.Sprintf("map (%v) has no layers, at zoom %v", req.mapName, req.z)
		log.Debug(msg)
		http.Error(w, msg, http.StatusNotFound)
		return
	}

	if req.layerName != "" {
		m = m.FilterLayersByName(req.layerName)
		if len(m.Layers) == 0 {
			msg := fmt.Sprintf("map (%v) has no layers, for LayerName %v at zoom %v", req.mapName, req.layerName, req.z)
			log.Debug(msg)
			http.Error(w, msg, http.StatusNotFound)
			return

		}
	}

	tile := slippy.Tile{Z: slippy.Zoom(req.z), X: req.x, Y: req.y}

	{
		// Check to see that the zxy is within the bounds of the map.
		// TODO(@ear7h): use a more efficient version of Intersect that doesn't
		// make a new extent
		ext3857, err := slippy.Extent(webmercatorGrid, tile)
		if err != nil {
			msg := fmt.Sprintf("map (%v -- %v) does not contains tile at %v/%v/%v. Unable to generate extent.", req.mapName, m.Bounds, req.z, req.x, req.y)
			log.Debug(msg, err)
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		points4326, err := proj.Inverse(proj.WebMercator, ext3857[:])
		if err != nil {
			msg := fmt.Sprintf("Unable to convert 3857 to 4326 for map (%v -- %v) and tile %v/%v/%v -- %v.", req.mapName, m.Bounds, req.z, req.x, req.y, ext3857)
			log.Error(msg)
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		ext4326 := &geom.Extent{}
		copy(ext4326[:], points4326)
		if _, intersect := m.Bounds.Intersect(ext4326); !intersect {
			msg := fmt.Sprintf("map (%v -- %v) does not contains tile at %v/%v/%v -- %v", req.mapName, m.Bounds, req.z, req.x, req.y, ext4326)
			log.Debug(msg)
			http.Error(w, msg, http.StatusNotFound)
			return
		}
	}

	// check for the debug query string
	if req.debug {
		m = m.AddDebugLayers()
	}

	// check for query parameters and populate param map with their values
	params, err := extractParameters(m, r)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	encodeCtx := context.WithValue(r.Context(), observability.ObserveVarMapName, m.Name)
	pbyte, err := m.Encode(encodeCtx, tile, params)

	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			// TODO: add debug logs
			// do nothing
			return
		case strings.Contains(err.Error(), "operation was canceled"):
			// do nothing
			return
		default:
			errMsg := fmt.Sprintf("error marshalling tile: %v", err)
			log.Error(errMsg)
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}
	}

	// mimetype for mapbox vector tiles
	// https://www.iana.org/assignments/media-types/application/vnd.mapbox-vector-tile
	w.Header().Add("Content-Type", mvt.MimeType)
	w.Header().Add("Content-Length", fmt.Sprintf("%d", len(pbyte)))
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(pbyte)
	if err != nil {
		log.Errorf("error writing tile z:%v, x:%v, y:%v - %v", req.z, req.x, req.y, err)
	}

	// check for tile size warnings
	if len(pbyte) > MaxTileSize {
		log.Infof("tile z:%v, x:%v, y:%v is rather large - %vKb", req.z, req.x, req.y, len(pbyte)/1024)
	}
}

func extractParameters(m atlas.Map, r *http.Request) (provider.Params, error) {
	var params provider.Params
	if m.Params != nil && len(m.Params) > 0 {
		params = make(provider.Params)
		err := r.ParseForm()
		if err != nil {
			return nil, err
		}

		for _, param := range m.Params {
			if r.Form.Has(param.Name) {
				val, err := param.ToValue(r.Form.Get(param.Name))
				if err != nil {
					return nil, err
				}
				params[param.Token] = val
			} else {
				p, err := param.ToDefaultValue()
				if err != nil {
					return nil, err
				}
				params[param.Token] = p
			}
		}
	}
	return params, nil
}
