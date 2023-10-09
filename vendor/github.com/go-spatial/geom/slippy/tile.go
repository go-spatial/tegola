package slippy

import (
	"errors"
	"fmt"

	"github.com/go-spatial/geom"
)

// MaxZoom is the lowest zoom (furthest in)
const MaxZoom = 22

// NewTile returns a Tile of Z,X,Y passed in
func NewTile(z, x, y uint) *Tile {
	return &Tile{
		Z: z,
		X: x,
		Y: y,
	}
}

// Tile describes a slippy tile.
type Tile struct {
	// zoom
	Z uint
	// column
	X uint
	// row
	Y uint
}

// NewTileMinMaxer returns the smallest tile which fits the
// geom.MinMaxer. Note: it assumes the values of ext are
// EPSG:4326 (lng/lat)
func NewTileMinMaxer(g Grid, ext geom.MinMaxer) (*Tile, bool) {
	tile, ok := g.FromNative(MaxZoom, geom.Point{
		ext.MinX(),
		ext.MinY(),
	})

	if !ok {
		return nil, false
	}


	var ret *Tile

	for z := uint(MaxZoom); int(z) >= 0 && ret == nil; z-- {
		RangeFamilyAt(g, tile, z, func(tile *Tile) error {
			if ext, ok := Extent(g, tile); ok && ext.Contains(geom.Point(ext.Max())) {
				ret = tile
				return errors.New("stop iter")
			}

			return nil
		})
	}

	return ret, true
}

// FromBounds returns a list of tiles that make up the bound given. The bounds should be defined as the following lng/lat points [4]float64{west,south,east,north}
func FromBounds(g Grid, bounds *geom.Extent, z uint) []Tile {
	if bounds == nil {
		return nil
	}

	p1, ok := g.FromNative(z, bounds.Min())
	if !ok {
		return nil
	}

	p2, ok := g.FromNative(z, bounds.Max())
	if !ok {
		return nil
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

	return ret
}

// ZXY returns back the z,x,y of the tile
func (t Tile) ZXY() (uint, uint, uint) { return t.Z, t.X, t.Y }

type Iterator func(*Tile) error

// RangeFamilyAt calls f on every tile vertically related to t at the specified zoom
// TODO (ear7h): sibling support
func RangeFamilyAt(g Grid, t *Tile, zoom uint, f Iterator) error {
	tl, ok := g.ToNative(t)
	if !ok {
		return fmt.Errorf("tile %v not valid for grid", t)
	}

	br, ok := g.ToNative(NewTile(t.Z, t.X+1, t.Y+1))
	if !ok {
		return fmt.Errorf("tile %v not valid for grid", t)
	}

	tlt, ok := g.FromNative(zoom, tl)
	if !ok {
		return fmt.Errorf("tile %v not valid for grid", t)
	}

	brt, ok := g.FromNative(zoom, br)
	if !ok {
		return fmt.Errorf("tile %v not valid for grid", t)
	}

	for x := tlt.X; x < brt.X; x++ {
		for y := tlt.Y; y < brt.Y; y++ {
			err := f(NewTile(zoom, x, y))
			if err != nil {
				return err
			}
		}
	}

	return nil
}
