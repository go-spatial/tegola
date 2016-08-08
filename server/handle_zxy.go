//Package server implements the http frontend
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

//	URI scheme: /maps/:map_name/:z/:x/:y
//	map_name - map name in the config file
//	z, x, y - tile coordinates as described in the Slippy Map Tilenames specification
//		z - zoom level
//		x - row
//		y - column
func handleZXY(w http.ResponseWriter, r *http.Request) {
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
		//	pop off URI prefix
		uri := r.URL.Path[len("/maps/"):]

		//	break apart our URI
		uriParts := strings.Split(uri, "/")

		//	check that we have the correct number of arguments in our URI
		if len(uriParts) != 4 {
			http.Error(w, "uri requires four params: /:map_id/:z/:x/:y", http.StatusBadRequest)
			return
		}

		//	lookup our map layers
		layers, ok := maps[uriParts[0]]
		if !ok {
			http.Error(w, "no map configured: "+uriParts[0], http.StatusBadRequest)
			return
		}

		//	trim the "y" param in the url in case it has an extension
		yparts := strings.Split(uriParts[3], ".")
		uriParts[3] = yparts[0]

		//	parse our URL vals to ints
		z, err := strconv.Atoi(uriParts[1])
		if err != nil {
			http.Error(w, "invalid z value: "+uriParts[1], http.StatusBadRequest)
			return
		}

		x, err := strconv.Atoi(uriParts[2])
		if err != nil {
			http.Error(w, "invalid x value: "+uriParts[2], http.StatusBadRequest)
			return
		}

		y, err := strconv.Atoi(uriParts[3])
		if err != nil {
			http.Error(w, "invalid y value: "+uriParts[3], http.StatusBadRequest)
			return
		}

		//	new tile
		tile := tegola.Tile{
			Z: z,
			X: x,
			Y: y,
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

			//	iterate our layers
			for i, l := range ls {
				//	incriment our waitgroup
				wg.Add(1)
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
		debug := r.URL.Query().Get("debug")
		if debug == "true" {
			//	add debug layer
			debugLayer := debugLayer(tile)
			mvtTile.AddLayers(debugLayer)
		}

		//	generate our vector tile
		vtile, err := mvtTile.VTile(tile.BoundingBox())
		if err != nil {
			http.Error(w, fmt.Sprintf("Error Getting VTile: %v", err.Error()), http.StatusBadRequest)
			return
		}

		//	marshal our tile into a protocol buffer
		var pbyte []byte
		pbyte, err = proto.Marshal(vtile)
		if err != nil {
			http.Error(w, "error marshalling tile", http.StatusInternalServerError)
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
