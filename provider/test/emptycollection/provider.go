package emptycollection

import (
	"context"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/provider"

	"github.com/go-spatial/tegola/dict"
)

const Name = "emptycollection"

var Count int

func init() {
	provider.Register(provider.TypeStd.Prefix()+Name, NewTileProvider, Cleanup)
}

// NewProvider setups a test provider. there are not currently any config params supported
func NewTileProvider(config dict.Dicter, maps []provider.Map) (provider.Tiler, error) {
	Count++
	return &TileProvider{}, nil
}

// Cleanup cleans up all the test providers.
func Cleanup() { Count = 0 }

type TileProvider struct{}

func (tp *TileProvider) Layers() ([]provider.LayerInfo, error) {
	return []provider.LayerInfo{
		layer{
			name:     "empty_geom_collection",
			geomType: geom.Collection{},
			srid:     tegola.WebMercator,
		},
	}, nil
}

// TileFeatures always returns a feature with a polygon outlining the tile's Extent (not Buffered Extent)
func (tp *TileProvider) TileFeatures(ctx context.Context, layer string, t provider.Tile, queryParams provider.Params, fn func(f *provider.Feature) error) error {
	// get tile bounding box
	_, srid := t.Extent()

	debugTileOutline := provider.Feature{
		ID:       0,
		Geometry: geom.Collection{}, // empty geometry collection
		SRID:     srid,
	}

	return fn(&debugTileOutline)
}
