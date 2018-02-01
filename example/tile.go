//!build
package main

import (
	"bytes"
	"log"

	"context"

	"github.com/golang/protobuf/proto"
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/geom/encoding/wkb"
	"github.com/terranodo/tegola/internal/convert"
	"github.com/terranodo/tegola/mvt"
)

// TileExample is a quick example of how to use the interface to marshall a tile.
func TileExample() {
	// We have our point in wkb format.
	var point = []byte{0, 0, 0, 0, 1, 70, 129, 246, 35, 46, 74, 93, 192, 3, 70, 27, 60, 175, 91, 64, 64}
	pointReader := bytes.NewReader(point)
	ggeo, err := wkb.Decode(pointReader)
	if err != nil {
		panic(err)
	}

	// Need to convert to a tegola type for now. (This is teomporary till we convert over to
	// geom types)
	geo, err := convert.ToTegola(ggeo)
	if err != nil {
		panic(err)
	}

	// Now we need to crate a feature. The way Tiles work, is that each tiles is
	// made up of a set of layers. Each layer contains more or more features, which
	// are geometeries with some meta data. So, first we must construct the feature
	// then we can create a layer, which we will add to a tile.

	// First we create the feature. A feature has a set of name value pairs. Most
	// base types, and any types that implements a Stringer interfaces are supported.
	feature1 := mvt.Feature{
		Tags: map[string]interface{}{
			"Name": "Point Item",
			"Foo":  "Point Item",
		},
		Geometry: geo,
	}
	// Create a new Layer, a Layer requires a name. This name should be unique within a tile.
	layer1 := mvt.Layer{
		Name: "Layer 1",
	}

	feature2 := mvt.Feature{
		Tags: map[string]interface{}{
			"Name": "Point Item",
			"Foo":  "Point Item",
		},
		Geometry: geo,
	}

	layer1.AddFeatures(feature1, feature2)

	var tile mvt.Tile
	// Add the layer to the tile
	if err = tile.AddLayers(&layer1); err != nil {
		panic(err)
	}

	layer2 := mvt.Layer{
		Name: "Layer 2",
	}

	layer2.AddFeatures(feature1)

	if err = tile.AddLayers(&layer2); err != nil {
		panic(err)
	}

	// VTile is the protobuff representation of the tile. This is what you can
	// send to the protobuff Marshal functions.
	ttile := tegola.NewTile(0, 0, 0)
	vtile, err := tile.VTile(context.Background(), ttile)
	if err != nil {
		panic(err)
	}
	// Print out the Marshaled tile as a string.
	log.Println(proto.MarshalTextString(vtile))
}

func main() {
	TileExample()
}
