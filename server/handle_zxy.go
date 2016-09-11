package server

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/mvt"
)

const (
	//	MaxTileSize is 500k. Currently just throws a warning when tile
	//	is larger than MaxTileSize
	MaxTileSize = 500000
	//	MaxZoom will not render tile beyond this zoom level
	MaxZoom = 20
)

type HandleZXY struct {
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
	//	debug
	debug bool
}

//	parseURI reads the request URI and extracts the various values for the request
func (req *HandleZXY) parseURI(r *http.Request) error {
	var err error

	//	pop off URI prefix
	uri := r.URL.Path[len("/maps/"):]

	//	break apart our URI
	uriParts := strings.Split(uri, "/")

	//	check that we have the correct number of arguments in our URI
	if len(uriParts) < 4 || len(uriParts) > 6 {
		log.Printf("invalid URI format (%v). expecting /maps/:map_name/:z/:x/:y", r.URL.Path)
		return fmt.Errorf("invalid URI format (%v). expecting /maps/:map_name/:z/:x/:y", r.URL.Path)
	}

	//	set map name
	req.mapName = uriParts[0]

	//	check for possible layer name (i.e. /maps/:map_name/:layer_name/:z/:x/:y)
	if len(uriParts) == 5 {
		req.layerName = uriParts[1]
	}

	//	parse our URL vals to ints
	z := uriParts[len(uriParts)-3]
	req.z, err = strconv.Atoi(z)
	if err != nil {
		log.Println("invalid Z value (%v)", z)
		return fmt.Errorf("invalid Z value (%v)", z)
	}

	x := uriParts[len(uriParts)-2]
	req.x, err = strconv.Atoi(x)
	if err != nil {
		log.Println("invalid X value (%v)", x)
		return fmt.Errorf("invalid X value (%v)", x)
	}

	//	trim the "y" param in the url in case it has an extension
	y := uriParts[len(uriParts)-1]
	yParts := strings.Split(y, ".")
	req.y, err = strconv.Atoi(yParts[0])
	if err != nil {
		log.Println("invalid Y value (%v)", y)
		return fmt.Errorf("invalid Y value (%v)", y)
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
func (req HandleZXY) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

			//	if our request has a layerName defined only render
			if req.layerName != "" {
				ls = layers.FilterByName(req.layerName)
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

//	creates a debug layer with z/x/y encoded as a point
func debugLayer(tile tegola.Tile) *mvt.Layer {
	//	get tile bounding box
	ext := tile.BoundingBox()

	//	create a new layer and name it
	layer := mvt.Layer{
		Name: "debug",
	}
	xlen := ext.Maxx - ext.Minx
	ylen := ext.Maxy - ext.Miny

	//	tile outlines
	outline := mvt.Feature{
		Tags: map[string]interface{}{
			"type": "debug_outline",
		},
		Geometry: &basic.Line{ //	tile outline
			basic.Point{ext.Minx, ext.Miny},
			basic.Point{ext.Maxx, ext.Miny},
			basic.Point{ext.Maxx, ext.Maxy},
			basic.Point{ext.Minx, ext.Maxy},
		},
	}

	//	new feature
	zxy := mvt.Feature{
		Tags: map[string]interface{}{
			"type": "debug_text",
			"zxy":  fmt.Sprintf("Z:%v, X:%v, Y:%v", tile.Z, tile.X, tile.Y),
		},
		Geometry: &basic.Point{ //	middle of the tile
			ext.Minx + (xlen / 2),
			ext.Miny + (ylen / 2),
		},
	}

	layer.AddFeatures(outline, zxy)

	return &layer
}
