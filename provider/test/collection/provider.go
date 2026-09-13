// Package collection provides a test provider that returns a single feature
// whose geometry is a non-empty geom.Collection (GEOMETRYCOLLECTION). This is
// used to verify that atlas splits collection geometries into multiple MVT
// features, since the MVT/vector tile spec has no "collection" feature type.
package collection

import (
	"context"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/provider"
)

const Name = "collection"

var Count int

func init() {
	provider.Register(provider.TypeStd.Prefix()+Name, NewTileProvider, Cleanup)
}

// NewTileProvider setups a test provider. there are not currently any config params supported
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
			name:     "geom_collection",
			geomType: geom.Collection{},
			srid:     tegola.WebMercator,
		},
	}, nil
}

// TileFeatures always returns a single feature whose geometry is a
// GEOMETRYCOLLECTION containing a point and a line string that both fall
// inside the tile's extent.
func (tp *TileProvider) TileFeatures(ctx context.Context, layerName string, t provider.Tile, queryParams provider.Params, fn func(f *provider.Feature) error) error {
	ext, srid := t.Extent()

	minx, miny := ext.MinX(), ext.MinY()
	maxx, maxy := ext.MaxX(), ext.MaxY()
	midx, midy := (minx+maxx)/2, (miny+maxy)/2

	feature := provider.Feature{
		ID: 0,
		Geometry: geom.Collection{
			geom.Point{midx, midy},
			geom.LineString{{minx, miny}, {maxx, maxy}},
		},
		SRID: srid,
	}

	return fn(&feature)
}
