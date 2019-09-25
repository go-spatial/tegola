package test

import (
	"context"
	"fmt"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/provider"

	"github.com/go-spatial/tegola/dict"
)

const Name = "test"

var Count int

func init() {
	provider.Register(Name, NewTileProvider, Cleanup)
}

// NewProvider setups a test provider. there are not currently any config params supported
func NewTileProvider(config dict.Dicter) (provider.Tiler, error) {
	Count++
	return &TileProvider{}, nil
}

// Cleanup cleans up all the test providers.
func Cleanup() { Count = 0 }

// TileProvider is the concrete type that satisfies the provider.Provider
// interface. The Features field can, optionally, be populated to be used
// in the TileFeatures call.
type TileProvider struct{
	Features []provider.Feature
}

func (tp *TileProvider) Layers() ([]provider.LayerInfo, error) {
	return []provider.LayerInfo{
		layer{
			name:     "test-layer",
			geomType: geom.Polygon{},
			srid:     tegola.WebMercator,
		},
	}, nil
}

// TilFeatures passes features to fn. If tp.Features is nil, then t's un-buffered extent
// is passed. If tp.Features is not nil, then all features with extents that intersect
// with t's extent are returned.
func (tp *TileProvider) TileFeatures(ctx context.Context, layer string, t provider.Tile, fn func(f *provider.Feature) error) error {
	if tp.Features != nil {
		// get features that were given to the provider
		for _, v := range tp.Features {
			ext, srid := t.Extent()
			if v.SRID != srid {
				panic(fmt.Sprintf("please use features in %v for the test provider", srid))
			}

			gext, err := geom.NewExtentFromGeometry(v.Geometry)
			if err != nil {
				return err
			}

			_, does := ext.Intersect(gext)
			if !does {
				continue
			}

			err = fn(&v)
			if err != nil {
				return err
			}
		}
		return nil
	} else {
		// get tile bounding box
		ext, srid := t.Extent()

		debugTileOutline := provider.Feature{
			ID:       0,
			Geometry: ext.AsPolygon(),
			SRID:     srid,
			Tags: map[string]interface{}{
				"type": "debug_buffer_outline",
			},
		}

		return fn(&debugTileOutline)
	}
}
