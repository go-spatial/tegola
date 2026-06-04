package tegola

import (
	"fmt"
	"math"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/slippy"
)

// TileGridSize returns the number of tiles in x and y for the given tile SRID.
// EPSG:4326 uses the WorldCRS84Quad matrix set: 2^(z+1) columns by 2^z rows.
func TileGridSize(srid uint64, z slippy.Zoom) (width, height uint, err error) {
	switch srid {
	case WebMercator:
		n := uint(math.Exp2(float64(z)))
		return n, n, nil
	case WGS84:
		return uint(math.Exp2(float64(z + 1))), uint(math.Exp2(float64(z))), nil
	default:
		return 0, 0, fmt.Errorf("unsupported tile_srid %d", srid)
	}
}

// WorldCRS84QuadExtent returns the CRS84 lon/lat extent for a WorldCRS84Quad tile.
func WorldCRS84QuadExtent(tile slippy.Tile) (*geom.Extent, error) {
	width, height, err := TileGridSize(WGS84, tile.Z)
	if err != nil {
		return nil, err
	}
	if tile.X >= width || tile.Y >= height {
		return nil, fmt.Errorf("tile %v outside WorldCRS84Quad bounds", tile)
	}

	tileWidth := 360.0 / float64(width)
	tileHeight := 180.0 / float64(height)
	minLon := -180.0 + float64(tile.X)*tileWidth
	maxLon := minLon + tileWidth
	maxLat := 90.0 - float64(tile.Y)*tileHeight
	minLat := maxLat - tileHeight

	return geom.NewExtent([2]float64{minLon, minLat}, [2]float64{maxLon, maxLat}), nil
}

func IsSupportedTileSRID(srid uint64) bool {
	return srid == WebMercator || srid == WGS84
}
