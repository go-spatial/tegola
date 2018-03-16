package test

import (
	"context"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/geom"
	"github.com/go-spatial/tegola/provider"
)

const Name = "test"

var Count int

func init() {
	provider.Register(Name, NewTileProvider, Cleanup)
}

// NewProvider setups a test provider. there are not currently any config params supported
func NewTileProvider(config map[string]interface{}) (provider.Tiler, error) {
	Count++
	return &TileProvider{}, nil
}

// Cleanup cleans up all the test providers.
func Cleanup() { Count = 0 }

type TileProvider struct{}

func (tp *TileProvider) Layers() ([]provider.LayerInfo, error) {
	return []provider.LayerInfo{
		layer{
			name:     "test-layer",
			geomType: geom.Polygon{},
			srid:     tegola.WebMercator,
		},
	}, nil
}

//	TilFeatures always returns a feature with a polygon outlining the tile's Extent (not Buffered Extent)
func (tp *TileProvider) TileFeatures(ctx context.Context, layer string, t provider.Tile, fn func(f *provider.Feature) error) error {
	//	get tile bounding box
	ext, srid := t.Extent()

	debugTileOutline := provider.Feature{
		ID: 0,
		Geometry: geom.Polygon{
			[][2]float64{
				{ext[0][0], ext[0][1]}, // Minx, Miny
				{ext[1][0], ext[0][1]}, // Maxx, Miny
				{ext[1][0], ext[1][1]}, // Maxx, Maxy
				{ext[0][0], ext[1][1]}, // Minx, Maxy
			},
		},
		SRID: srid,
		Tags: map[string]interface{}{
			"type": "debug_buffer_outline",
		},
	}

	return fn(&debugTileOutline)
}
