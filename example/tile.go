//!build
package main

import (
	"bytes"
	"log"

	"github.com/golang/protobuf/proto"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/wkb"
)

// TileExample is a quick example of how to use the interface to marshal a tile.
func TileExample() {

	// We have our point in wkb format.
	var point = []byte{0, 0, 0, 0, 1, 70, 129, 246, 35, 46, 74, 93, 192, 3, 70, 27, 60, 175, 91, 64, 64}
	pointReader := bytes.NewReader(point)
	geo, err := wkb.Decode(pointReader)
	if err != nil {
		panic(err)
	}
	// Now we need to crate a feature. The way Tiles work, is that each tiles is
	// made up of a set of layers. Each layer contains more or more features, which
	// are geometeries with some meta data. So, first we must construct the feature
	// then we can create a layer, which we will add to a tile.

	// First we create the feature. A feature has a set of name value pairs. Most
	// base types, and any types that implements a Stringer interfaces are supported.
	feature := mvt.Feature{
		Tags: map[string]interface{}{
			"Name": "Point Item",
		},
		Geometry: geo,
	}
	// Create a new Layer, a Layer requires a name. This name should be unique within a tile.
	layer := mvt.Layer{
		Name: "Layer 1",
	}
	layer.AddFeatures(feature)

	var tile mvt.Tile
	// Add the layer to the tile
	if err = tile.AddLayer(&layer); err != nil {
		panic(err)
	}
	layer1 := mvt.Layer{
		Name: "Layer 2",
	}
	if err = tile.AddLayer(&layer1); err != nil {
		panic(err)
	}
	log.Println(tile)

	// VTile is the protobuff representation of the tile. This is what you can
	// send to the protobuff Marshal functions.
	vtile, err := tile.VTile()
	if err != nil {
		panic(err)
	}
	// Print out the Marshaled tile as a string.
	log.Println(proto.MarshalTextString(vtile))
}

func main() {
	TileExample()
}
