//tegola server
package server

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/mvt/vector_tile"
)

const (
	//	500k
	MaxTileSize = 500000
	//	suggested as max by Slippy Map Tilenames spec
	MaxZoom = 18
)

//	encode an example tile for demo purposes
func exampleTile() (*vectorTile.Tile, error) {
	var err error
	var tile mvt.Tile

	//	create a line
	line1 := &basic.Line{
		basic.Point{0, 0},
		basic.Point{2048, 0},
	}

	//	create a polygon
	poly1 := &basic.Polygon{
		basic.Line{
			basic.Point{0, 250},
			basic.Point{2048, 259},
			basic.Point{2048, 1000},
			basic.Point{0, 1000},
		},
	}

	// Now we need to crate a feature. The way Tiles work, is that each tiles is
	// made up of a set of layers. Each layer contains more or more features, which
	// are geometeries with some meta data. So, first we must construct the feature
	// then we can create a layer, which we will add to a tile.

	// First we create the feature. A feature has a set of name value pairs. Most
	// base types, and any types that implements a Stringer interfaces are supported.
	feature1 := mvt.Feature{
		Tags: map[string]interface{}{
			"class": "path",
		},
		Geometry: line1,
	}

	// Create a new Layer, a Layer requires a name. This name should be unique within a tile.
	layer1 := mvt.Layer{
		Name: "tunnel",
	}

	//	add feature to layer
	layer1.AddFeatures(feature1)

	// Add the layer to the tile
	if err = tile.AddLayer(&layer1); err != nil {
		return nil, err
	}

	feature2 := mvt.Feature{
		Tags: map[string]interface{}{
			"class": "park",
		},
		Geometry: poly1,
	}

	// Create a new Layer, a Layer requires a name. This name should be unique within a tile.
	layer2 := mvt.Layer{
		Name: "landuse",
	}

	layer2.AddFeatures(feature2)

	if err = tile.AddLayer(&layer2); err != nil {
		return nil, err
	}

	// VTile is the protobuff representation of the tile. This is what you can
	// send to the protobuff Marshal functions.
	return tile.VTile()
}

//	URI scheme: /maps/:map_id/:z/:x/:y
//		map_id - id in the config file with an accompanying data source
//		z, x, y - tile coordinates as described in the Slippy Map Tilenames specification
//			z - zoom level
//			x - row
//			y - column
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
		//	TODO: look up layer data source provided by config

		//	pop off URI prefix
		uri := r.URL.Path[len("/maps/"):]

		//	break apart our URI
		uriParts := strings.Split(uri, "/")

		//	check that we have the correct number of arguments in our URI
		if len(uriParts) != 4 {
			http.Error(w, "uri requires four params: /:map_id/:z/:x/:y", http.StatusBadRequest)
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

		y, err := strconv.Atoi(uriParts[2])
		if err != nil {
			http.Error(w, "invalid y value: "+uriParts[3], http.StatusBadRequest)
			return
		}

		//	new tile
		tile := Tile{
			Z: z,
			X: x,
			Y: y,
		}

		//	check for the max zoom level
		if tile.Z > MaxZoom {
			msg := fmt.Sprintf("zoom level %v is great than max supported zoom of %v", tile.Z, MaxZoom)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		//	TODO: calculate the web mercator bounding box with the slippy math function
		/*
			lat, lng := tile.Deg2Num()
			log.Printf("tile %+v\n", tile)
			log.Printf("lat: %v, lng: %v\n", lat, lng)
		*/
		//	generate a tile
		vtile, err := exampleTile()
		if err != nil {
			http.Error(w, "error generating tile tile", http.StatusInternalServerError)
			return
		}

		//	fetch tile from datasource
		//	marshal our tile into a protocol buffer
		pbyte, err := proto.Marshal(vtile)
		if err != nil {
			http.Error(w, "error marshalling tile", http.StatusInternalServerError)
			return
		}

		//	check for tile size warnings
		if len(pbyte) > MaxTileSize {
			log.Println("tile is too large!", len(pbyte))
		}

		//	TODO: how configurable do we want the CORS policy to be?
		//	set CORS header
		w.Header().Add("Access-Control-Allow-Origin", "*")

		//	mimetype for protocol buffers
		w.Header().Add("Content-Type", "application/x-protobuf")

		w.Write(pbyte)

	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
}
