package server

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/dimfeld/httptreemux"
	"github.com/golang/protobuf/proto"
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/mvt"
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
	if err != nil {
		log.Printf("invalid Z value (%v)", z)
		return fmt.Errorf("invalid Z value (%v)", z)
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
		m, ok := maps[req.mapName]
		if !ok {
			errMsg := fmt.Sprintf("map (%v) not configured. check your config file", req.mapName)
			log.Println(errMsg)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}

		//	new tile
		tile := tegola.Tile{
			Z: req.z,
			X: req.x,
			Y: req.y,
		}

		//	generate a tile
		var mvtTile mvt.Tile

		//	check that our request is below max zoom and we have layers to render
		if tile.Z <= MaxZoom && len(m.Layers) != 0 {

			//	wait group for concurrent layer fetching
			var wg sync.WaitGroup
			//	filter down the layers we need for this zoom
			ls := m.FilterLayersByZoom(tile.Z)

			//	if our request has a layerName defined only render
			if req.layerName != "" {
				ls = m.FilterLayersByName(req.layerName)
				if len(ls) == 0 {
					errMsg := fmt.Sprintf("layer (%v) not configured for map (%v)", req.layerName, req.mapName)
					log.Println(errMsg)
					http.Error(w, errMsg, http.StatusBadRequest)
				}
			}

			//	layer stack
			mvtLayers := make([]*mvt.Layer, len(ls))

			//	set our waitgroup count
			wg.Add(len(ls))

			//	iterate our layers
			for i, l := range ls {
				//	go routine for rendering the layer
				go func(i int, l Layer) {
					//	on completion let the wait group know
					defer wg.Done()

					//	fetch layer from data provider
					mvtLayer, err := l.Provider.MVTLayer(l.ProviderLayer, tile, l.DefaultTags)
					mvtLayer.Name = l.Name
					if err != nil {
						errMsg := fmt.Sprintf("Error Getting MVTLayer: %v", err)
						log.Println(errMsg)
						http.Error(w, errMsg, http.StatusBadRequest)
						return
					}
					//	add the layer to the slice position
					mvtLayers[i] = mvtLayer
				}(i, l)
			}

			//	wait for the waitgroup to finish
			wg.Wait()

			//	add layers to our tile
			mvtTile.AddLayers(mvtLayers...)
		}

		//	check for the debug query string
		if req.debug {
			//	add debug layer
			debugLayer := debugLayer(tile)
			mvtTile.AddLayers(debugLayer)
		}

		//	generate our vector tile
		vtile, err := mvtTile.VTile(tile.BoundingBox())
		if err != nil {
			errMsg := fmt.Sprintf("Error Getting VTile: %v", err.Error())
			log.Println(errMsg)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}

		//	marshal our tile into a protocol buffer
		var pbyte []byte
		pbyte, err = proto.Marshal(vtile)
		if err != nil {
			errMsg := fmt.Sprintf("Error marshalling tile: %v", err)
			log.Println(errMsg)
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}

		//	TODO: how configurable do we want the CORS policy to be?
		//	set CORS header
		w.Header().Add("Access-Control-Allow-Origin", "*")

		//	mimetype for protocol buffers
		w.Header().Add("Content-Type", "application/x-protobuf")

		w.Write(pbyte)

		//	check for tile size warnings
		if len(pbyte) > MaxTileSize {
			log.Printf("tile z:%v, x:%v, y:%v is rather large - %v", tile.Z, tile.X, tile.Y, len(pbyte))
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
