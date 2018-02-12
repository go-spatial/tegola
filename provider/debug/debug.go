//	The debug provider returns features that are helpful for debugging a tile
//	including a box for the tile edges and a point in the middle of the tile
//	with z,x,y values encoded
package debug

import (
	"context"
	"fmt"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/geom"
	"github.com/terranodo/tegola/provider"
)

const Name = "debug"

const (
	LayerDebugTileOutline = "debug-tile-outline"
	LayerDebugTileCenter  = "debug-tile-center"
)

func init() {
	provider.Register(Name, NewTileProvider, nil)
}

// NewProvider Setups a debug provider. there are not currently any config params supported
func NewTileProvider(config map[string]interface{}) (provider.Tiler, error) {
	return &Provider{}, nil
}

// Provider provides the debug provider
type Provider struct{}

func (p *Provider) TileFeatures(ctx context.Context, layer string, tile provider.Tile, fn func(f *provider.Feature) error) error {

	// get tile bounding box
	ext, srid := tile.Extent()

	switch layer {
	case "debug-tile-outline":
		debugTileOutline := provider.Feature{
			ID: 0,
			Geometry: geom.Polygon{
				[][2]float64{
					[2]float64{ext[0][0], ext[0][1]}, // Minx, Miny
					[2]float64{ext[1][0], ext[0][1]}, // Maxx, Miny
					[2]float64{ext[1][0], ext[1][1]}, // Maxx, Maxy
					[2]float64{ext[0][0], ext[1][1]}, // Minx, Maxy
				},
			},
			SRID: srid,
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
		z, x, y := tile.ZXY()

		debugTileCenter := provider.Feature{
			ID: 1,
			Geometry: geom.Point{
				//	Minx
				ext[0][0] + (xlen / 2),
				//	Miny
				ext[0][1] + (ylen / 2),
			},
			SRID: srid,
			Tags: map[string]interface{}{
				"type": "debug_text",
				"zxy":  fmt.Sprintf("Z:%v, X:%v, Y:%v", z, x, y),
			},
		}

		if err := fn(&debugTileCenter); err != nil {
			return err
		}
	}

	return nil
}

// Layers returns information about the various layers the provider supports
func (p *Provider) Layers() ([]provider.LayerInfo, error) {
	layers := []Layer{
		{
			name:     "debug-tile-outline",
			geomType: geom.Line{},
			srid:     tegola.WebMercator,
		},
		{
			name:     "debug-tile-center",
			geomType: geom.Point{},
			srid:     tegola.WebMercator,
		},
	}

	var ls []provider.LayerInfo

	for i := range layers {
		ls = append(ls, layers[i])
	}

	return ls, nil
}
