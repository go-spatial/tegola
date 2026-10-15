package hana

import (
	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/slippy"
	"github.com/alexeydott/tegola/provider"
)

// scaleTile normalizes HANA planar-equivalent labels without changing bounds
// or losing the optional pixel dimensions of a custom tile.
type scaleTile struct{ provider.Tile }

func (t scaleTile) Extent() (*geom.Extent, uint64) {
	extent, srid := t.Tile.Extent()
	return extent, transformSRID(srid)
}
func (t scaleTile) PixelSize() (uint, uint) {
	if sized, ok := t.Tile.(interface{ PixelSize() (uint, uint) }); ok {
		return sized.PixelSize()
	}
	return slippy.DefaultTileSize, slippy.DefaultTileSize
}
func tileScale(tile provider.Tile, layerSRID uint64) (width, height, denominator float64, err error) {
	return provider.TileScale(scaleTile{tile}, transformSRID(layerSRID))
}
