//	The debug provider returns features that are helpful for debugging a tile
//	including a box for the tile edges and a point in the middle of the tile
//	with z,x,y values encoded
package debug

import (
	"context"
	"fmt"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/geom"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/mvt/provider"
	newProvider "github.com/terranodo/tegola/provider"
)

const Name = "debug"

const (
	LayerDebugTileOutline = "debug-tile-outline"
	LayerDebugTileCenter  = "debug-tile-center"
)

func init() {
	provider.Register(Name, NewProvider)
}

//	NewProvider Setups a debug provider. there are not currently any config params supported
func NewProvider(config map[string]interface{}) (mvt.Provider, error) {
	return &Provider{}, nil
}

//	NewProvider Setups a debug provider. there are not currently any config params supported
func NewTileProvider(config map[string]interface{}) (newProvider.Tiler, error) {
	return &Provider{}, nil
}

// Provider provides the debug provider
type Provider struct{}

func (p *Provider) MVTLayer(ctx context.Context, layerName string, tile *tegola.Tile, dtags map[string]interface{}) (*mvt.Layer, error) {
	var layer mvt.Layer

	//	get tile bounding box
	ext := tile.BoundingBox()

	xlen := ext.Maxx - ext.Minx
	ylen := ext.Maxy - ext.Miny

	switch layerName {
	case "debug-tile-outline":
		//	debug outlines
		layer = mvt.Layer{
			Name:         LayerDebugTileOutline,
			DontSimplify: true,
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
		layer.AddFeatures(debugOutline)
		ext1, err := tile.BufferedBoundingBox()
		if err != nil {
			return nil, err
		}
		debugBufferOutline := mvt.Feature{
			Tags: map[string]interface{}{
				"type": "debug_buffer_outline",
			},
			Geometry: &basic.Line{ //	tile outline
				basic.Point{ext1.Minx, ext1.Miny},
				basic.Point{ext1.Maxx, ext1.Miny},
				basic.Point{ext1.Maxx, ext1.Maxy},
				basic.Point{ext1.Minx, ext1.Maxy},
			},
		}
		layer.AddFeatures(debugBufferOutline)

	case "debug-tile-center":
		//	debug center points
		layer = mvt.Layer{
			Name:         LayerDebugTileCenter,
			DontSimplify: true,
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
		layer.AddFeatures(debugCenter)
	}

	return &layer, nil
}

func (p *Provider) TileFeatures(ctx context.Context, layer string, tile newProvider.Tile, fn func(f *newProvider.Feature) error) error {

	//	get tile bounding box
	ext, _ := tile.Extent()

	switch layer {
	case "debug-tile-outline":
		debugTileOutline := newProvider.Feature{
			ID: 0,
			Geometry: geom.Polygon{
				[][2]float64{
					[2]float64{ext[0][0], ext[0][1]}, // Minx, Miny
					[2]float64{ext[1][0], ext[0][1]}, // Maxx, Miny
					[2]float64{ext[1][0], ext[1][1]}, // Maxx, Maxy
					[2]float64{ext[0][0], ext[1][1]}, // Minx, Maxy
				},
			},
			SRID: tegola.WebMercator,
			Tags: map[string]interface{}{
				"type": "debug_buffer_outline",
			},
		}

		if err := fn(&debugTileOutline); err != nil {
			return err
		}

	case "debug-tile-center":
		xlen := ext[1][0] - ext[0][0] // Maxx - Minx
		ylen := ext[1][1] - ext[0][1] // Maxy - Miny

		debugTileCenter := newProvider.Feature{
			ID: 1,
			Geometry: geom.Point{
				//	Minx
				ext[0][0] + (xlen / 2),
				//	Miny
				ext[0][1] + (ylen / 2),
			},
			SRID: tegola.WebMercator,
			Tags: map[string]interface{}{
				"type": "debug_text",
				"zxy":  fmt.Sprintf("Z:%v, X:%v, Y:%v", tile.Z, tile.X, tile.Y),
			},
		}

		if err := fn(&debugTileCenter); err != nil {
			return err
		}
	}

	return nil
}

// Layers returns information about the various layers the provider supports
func (p *Provider) Layers() ([]mvt.LayerInfo, error) {
	layers := []Layer{
		{
			name:     "debug-tile-outline",
			geomType: basic.Line{},
			srid:     tegola.WebMercator,
		},
		{
			name:     "debug-tile-center",
			geomType: basic.Point{},
			srid:     tegola.WebMercator,
		},
	}

	var ls []mvt.LayerInfo

	for i := range layers {
		ls = append(ls, layers[i])
	}

	return ls, nil
}
