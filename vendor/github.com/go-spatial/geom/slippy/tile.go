package slippy

import (
	"errors"
	"math"
	"strconv"

	"github.com/go-spatial/geom"
)

var (
	ErrNilBounds = errors.New("slippy: Bounds cannot be nil")
)

// MaxZoom is the lowest zoom (furthest in)
const MaxZoom = 22

// Zoom represents a zoom level; this usually goes from 0 to 22
type Zoom uint

func (z Zoom) N() float64 { return math.Exp2(float64(z)) }
func (z Zoom) TileSize() Tile {
	n := uint(z.N())
	return Tile{
		Z: z,
		X: n,
		Y: n,
	}
}

// Tile describes a slippy tile.
type Tile struct {
	// zoom
	Z Zoom
	// column
	X uint
	// row
	Y uint
}

func (tile Tile) ZXY() (Zoom, uint, uint) { return tile.Z, tile.X, tile.Y }

// Equal tests for equality
func (tile Tile) Equal(other Tile) bool {
	return tile.Z == other.Z && tile.X == other.X && tile.Y == other.Y
}

// Less weather the `other` tile is less than the tile, by first checking the zoom, then the x coordinate, and finally the y coordinate.
func (tile Tile) Less(other Tile) bool {
	zi, xi, yi := tile.ZXY()
	zj, xj, yj := other.ZXY()
	switch {
	case zi != zj:
		return zi < zj
	case xi != xj:
		return xi < xj
	default:
		return yi < yj
	}
}

func (tile Tile) String() string {
	return strconv.FormatInt(int64(tile.Z), 10) + "/" +
		strconv.FormatInt(int64(tile.X), 10) + "/" +
		strconv.FormatInt(int64(tile.Y), 10)
}

// FamilyAt returns an iterator function that will call the yield function with every related tile at the requested
// zoom. This will include the provided tile itself. (if the same zoom is provided). The parent (overlapping tile at a lower zoom level),
// or children (overlapping tiles at a higher zoom level).
//
//		This function is structured so that it can take advantage of go1.23's Range Funcs. e.g.:
//	    for tile := range aTile.FamilyAt(10) {
//	       fmt.Printf("got tile: %v\n",tile)
//	    }
func (tile Tile) FamilyAt(zoom Zoom) func(yield func(Tile) bool) {
	return func(yield func(Tile) bool) {
		// handle ancestors and self
		if zoom <= tile.Z {
			mag := tile.Z - zoom
			yield(Tile{Z: zoom, X: tile.X >> mag, Y: tile.Y >> mag})
			return
		}

		// handle descendants
		mag := int(zoom) - int(tile.Z)
		delta := int(math.Exp2(float64(mag)))
		leastX := int(tile.X) << mag
		leastY := int(tile.Y) << mag
		for x := leastX; x < leastX+delta; x++ {
			for y := leastY; y < leastY+delta; y++ {
				if !yield(Tile{Z: zoom, X: uint(x), Y: uint(y)}) {
					// stop iterating
					return
				}
			}
		}
	}
}

// RangeFamilyAt returns an iterator function that will call the yield function with every related tile at the requested
// zoom. This will include the provided tile itself. (if the same zoom is provided). The parent (overlapping tile at a lower zoom level),
// or children (overlapping tiles at a higher zoom level).
func RangeFamilyAt(tile Tile, zoom Zoom, yield func(Tile) bool) { tile.FamilyAt(zoom)(yield) }

// FromBounds returns a list of tiles that make up the bound given.
// The bounds should be defined as the following lng/lat points [4]float64{west,south,east,north}
//
//	The only errors this generates are if the bounds is nil, or any errors grid returns from
//	transformation the bounds points.
func FromBounds(g TileGridder, bounds geom.PtMinMaxer, z Zoom) ([]Tile, error) {
	if bounds == nil {
		return nil, ErrNilBounds
	}

	p1, err := g.FromNative(z, bounds.Min())
	if err != nil {
		return nil, err
	}

	p2, err := g.FromNative(z, bounds.Max())
	if err != nil {
		return nil, err
	}

	minx, maxx := p1.X, p2.X
	if minx > maxx {
		minx, maxx = maxx, minx
	}

	miny, maxy := p1.Y, p2.Y
	if miny > maxy {
		miny, maxy = maxy, miny
	}

	ret := make([]Tile, 0, (maxx-minx+1)*(maxy-miny+1))

	for x := minx; x <= maxx; x++ {
		for y := miny; y <= maxy; y++ {
			ret = append(ret, Tile{z, x, y})
		}
	}

	return ret, nil
}

// MvtTileDim is the number of pixels in a tile
const MvtTileDim = 4096.0

// PixelRatioForZoom returns the ratio of pixels to projected units at the given zoom. Multiply this value by
// the pixel count in tile.buffer to get the expected conversion.
//
// if zoom is larger the MaxZoom, it will be set to MaxZoom
// if tileDim is 0, it will be set to MvtTileDim
func PixelRatioForZoom(g TileGridder, zoom Zoom, tileDim uint64) float64 {
	if zoom > MaxZoom {
		zoom = MaxZoom
	}
	if tileDim == 0 {
		tileDim = MvtTileDim
	}
	ext, _ := Extent(g, Tile{Z: zoom})
	return ext.XSpan() / float64(tileDim)
}

// MvtPixelRationForZoom returns the ratio of pixels to projected units at the given zoom.
// This assumes an MVT tile is being used.
func MvtPixelRationForZoom(g TileGridder, zoom Zoom) float64 {
	return PixelRatioForZoom(g, zoom, MvtTileDim)
}
