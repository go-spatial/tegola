//tegola server
package main

import (
	"log"
	"net/http"

	"github.com/golang/protobuf/proto"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/mvt/vector_tile"
)

var vtile *vectorTile.Tile

func exampleTile() {
	// We have our point in wkb format.
	geo := &basic.Line{basic.Point{0, 0}, basic.Point{2048, 2048}}
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
		Geometry: geo,
	}
	// Create a new Layer, a Layer requires a name. This name should be unique within a tile.
	layer1 := mvt.Layer{
		Name: "tunnel",
	}

	layer1.AddFeatures(feature1)

	var tile mvt.Tile
	var err error
	// Add the layer to the tile
	if err = tile.AddLayers(&layer1); err != nil {
		panic(err)
	}

	// VTile is the protobuff representation of the tile. This is what you can
	// send to the protobuff Marshal functions.
	vtile, err = tile.VTile()
	if err != nil {
		panic(err)
	}
}

func handleZXY(w http.ResponseWriter, r *http.Request) {
	// Mime type should be application/vnd.mapbox-vector-tile
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Content-Type", "application/x-protobuf")
	log.Println("Go!")
	//	proto.MarshalText(w, vtile)
	pbyte, err := proto.Marshal(vtile)
	if err != nil {
		panic(err)
	}
	log.Println("Content Length should be:", len(pbyte))
	w.Write(pbyte)
}
func main() {

	log.Println("hello tegola")
	exampleTile()
	http.HandleFunc("/z/", handleZXY)

	log.Fatal(http.ListenAndServe(":9080", nil))
}
