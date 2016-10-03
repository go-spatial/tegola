package server

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/pressly/chi"
	"github.com/terranodo/tegola"
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

	//	set map name
	req.mapName = chi.URLParam(r, "map_name")

	//	parse our URL vals to ints
	z := chi.URLParam(r, "z")
	req.z, err = strconv.Atoi(z)
	if err != nil {
		log.Printf("invalid Z value (%v)", z)
		return fmt.Errorf("invalid Z value (%v)", z)
	}

	x := chi.URLParam(r, "x")
	req.x, err = strconv.Atoi(x)
	if err != nil {
		log.Printf("invalid X value (%v)", x)
		return fmt.Errorf("invalid X value (%v)", x)
	}

	//	trim the "y" param in the url in case it has an extension
	y := chi.URLParam(r, "y")
	yParts := strings.Split(y, ".")
	req.y, err = strconv.Atoi(yParts[0])
	if err != nil {
		log.Printf("invalid Y value (%v)", y)
		return fmt.Errorf("invalid Y value (%v)", y)
	}

	//	check if we have a file extension
	if len(yParts) == 2 {
		req.extension = yParts[1]
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

		//	lookup our map layers
		layers, ok := maps[req.mapName]
		if !ok {
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

		//	generate a tile
		var mvtTile mvt.Tile

		//	check that our request is below max zoom and we have layers to render
		if tile.Z <= MaxZoom && len(layers) != 0 {

			//	wait group for concurrent layer fetching
			var wg sync.WaitGroup
			//	filter down the layers we need for this zoom
			ls := layers.FilterByZoom(tile.Z)

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
					mvtLayer, err := l.Provider.MVTLayer(l.Name, tile, l.DefaultTags)
					if err != nil {
						log.Printf("Error Getting MVTLayer: %v", err)
						http.Error(w, fmt.Sprintf("Error Getting MVTLayer: %v", err.Error()), http.StatusBadRequest)
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
			log.Printf("Error Getting VTile: %v", err)
			http.Error(w, fmt.Sprintf("Error Getting VTile: %v", err.Error()), http.StatusBadRequest)
			return
		}

		//	marshal our tile into a protocol buffer
		var pbyte []byte
		pbyte, err = proto.Marshal(vtile)
		if err != nil {
			log.Printf("Error marshalling tile: %v", err)
			http.Error(w, "Error marshalling tile", http.StatusInternalServerError)
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
