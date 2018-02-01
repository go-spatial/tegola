package test_provider

import (
	"context"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/geom"
	"github.com/terranodo/tegola/provider"
)

type TestTileProvider struct{}

func (tp *TestTileProvider) Layers() ([]provider.LayerInfo, error) {
	return []provider.LayerInfo{
		layer{
			name:     "test-layer",
			geomType: geom.Polygon{},
			srid:     tegola.WebMercator,
		},
	}, nil
}

//	TilFeatures always returns a feature with a polygon outlining the tile's Extent (not Buffered Extent)
func (tp *TestTileProvider) TileFeatures(ctx context.Context, layer string, t provider.Tile, fn func(f *provider.Feature) error) error {
	//	get tile bounding box
	ext, srid := t.Extent()

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

	return fn(&debugTileOutline)
}
