package slippy

import (
	"fmt"
	"math"

	"github.com/go-spatial/geom"
)

// TileGrid contains the tile layout, including ability to get WGS84 coordinates for tile extents
type Grid interface {
	// SRID returns the SRID of the coordinate system of the
	// implementer. The geomtries returned by the other methods
	// will be in these coordinates.
	SRID() uint

	// Size returns a tile where the X and Y are the size of that zoom's
	// tile grid. AKA:
	//	Tile{z, MaxX + 1, MaxY + 1
	Size(z uint) (*Tile, bool)

	// FromNative converts from a point (in the Grid's coordinates system) and zoom
	// to a tile. ok will be false if the point is not valid for this coordinate
	// system.
	FromNative(z uint, pt geom.Point) (tile *Tile, ok bool)

	// ToNative returns the tiles upper left point. ok will be false if
	// the tile is not valid. A note on implemetation is that this method
	// should be able to take tiles with x and y values 1 higher than the max,
	// this is to fetch the bottom right corner of the grid
	ToNative(*Tile) (pt geom.Point, ok bool)
}

func Extent(g Grid, t *Tile) (ext *geom.Extent, ok bool) {
	if t == nil {
		return nil, false
	}

	tl, ok := g.ToNative(t)
	if !ok {
		return nil, false
	}
	br, ok := g.ToNative(NewTile(t.Z, t.X + 1, t.Y + 1))
	if !ok {
		return nil, false
	}

	return geom.NewExtentFromPoints(tl, br), true
}

// NewGrid returns the grid conventionally used with the
// given SRID. Errors if the SRID is not supported. The
// currently supported SRID's are:
//	4326
//	3857
func NewGrid(srid uint) (Grid, error) {
	switch srid {
	case 4326:
		return &grid{
			srid: srid,
			tilingRatio: 2,
			maxx: LonMax,
			maxy: LatMax,
		}, nil
	case 3857:
		return &grid{
			srid: srid,
			tilingRatio: 1,
			maxx: WebMercatorMax,
			maxy: WebMercatorMax,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported srid: %v", srid)
	}
}

// WebMercatorMax is the max size in meters of a tile
const WebMercatorMax = 20037508.34
const LatMax = 90
const LonMax = 180

type grid struct {
	srid uint
	// aspect ratio of tile scheme (colums / rows)
	// TODO(ear7h) this will break tall maps(the x from Size(0) will be 0)
	tilingRatio float64
	// TODO(ear7h) change this to ranges and origin point
	// might be needed for future coordinate systems.
	maxx, maxy float64 // in native coords
}

func (g *grid) SRID() uint {
	return g.srid
}

func (g *grid) Size(zoom uint) (*Tile, bool) {
	if zoom > MaxZoom {
		return nil, false
	}

	dim := uint(1) << zoom // hopefully the zoom isn't larger than 64
	return NewTile(zoom, dim * uint(g.tilingRatio), dim), true
}

func (g *grid) FromNative(zoom uint, pt geom.Point) (*Tile, bool) {
	if zoom > MaxZoom || pt.X() > g.maxx || pt.Y() > g.maxy {
		return nil, false
	}

	res := g.maxx * 2 / g.tilingRatio / math.Exp2(float64(zoom))
	x := uint((pt.X() + g.maxx) / res)

	res = g.maxy * 2 / math.Exp2(float64(zoom))
	y := uint(-(pt.Y() - g.maxy) / res)

	return NewTile(zoom, x, y), true
}

func (g *grid) ToNative(tile *Tile) (geom.Point, bool) {
	if max, ok := g.Size(tile.Z); !ok || tile.X > max.X || tile.Y > max.Y {
		return geom.Point{}, false
	}

	res := g.maxx * 2 / g.tilingRatio / math.Exp2(float64(tile.Z))
	x := -g.maxx + float64(tile.X)*res

	res = g.maxy * 2 / math.Exp2(float64(tile.Z))
	y := g.maxy - float64(tile.Y)*res

	return geom.Point{x, y}, true
}
