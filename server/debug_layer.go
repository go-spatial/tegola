package server

import (
	"fmt"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/mvt"
)

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
