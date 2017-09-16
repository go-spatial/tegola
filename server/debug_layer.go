package server

import (
	"fmt"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/mvt"
)

//	creates a debug layers with z/x/y encoded as a point
func debugLayer(tile tegola.Tile) []*mvt.Layer {
	var layers []*mvt.Layer

	//	get tile bounding box
	ext := tile.BoundingBox()

	xlen := ext.Maxx - ext.Minx
	ylen := ext.Maxy - ext.Miny

	//	debug outlines
	debugTileOutline := mvt.Layer{
		Name: "debug-tile-outline",
	}
	debugOutline := mvt.Feature{
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
	debugTileOutline.AddFeatures(debugOutline)

	layers = append(layers, &debugTileOutline)

	//	debug center points
	debugTileCenter := mvt.Layer{
		Name: "debug-tile-center",
	}
	debugCenter := mvt.Feature{
		Tags: map[string]interface{}{
			"type": "debug_text",
			"zxy":  fmt.Sprintf("Z:%v, X:%v, Y:%v", tile.Z, tile.X, tile.Y),
		},
		Geometry: &basic.Point{ //	middle of the tile
			ext.Minx + (xlen / 2),
			ext.Miny + (ylen / 2),
		},
	}
	debugTileCenter.AddFeatures(debugCenter)

	layers = append(layers, &debugTileCenter)

	return layers
}
