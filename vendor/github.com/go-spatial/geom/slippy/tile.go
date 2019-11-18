package slippy

import (
	"math"

	"errors"

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
func NewTileMinMaxer(ext geom.MinMaxer) *Tile {
	upperLeft := NewTileLatLon(MaxZoom, ext.MaxY(), ext.MinX())
	point := &geom.Point{ext.MaxX(), ext.MinY()}

	var ret *Tile

	for z := uint(MaxZoom); int(z) >= 0 && ret == nil; z-- {
		upperLeft.RangeFamilyAt(z, func(tile *Tile) error {
			if tile.Extent4326().Contains(point) {
				ret = tile
				return errors.New("stop iter")
			}

			return nil
		})

	}
	return ret
}

// NewTileLatLon instantiates a tile containing the coordinate with the specified zoom
func NewTileLatLon(z uint, lat, lon float64) *Tile {
	x := Lon2Tile(z, lon)
	y := Lat2Tile(z, lat)

	return &Tile{
		Z: z,
		X: x,
		Y: y,
	}
}

func minmax(a, b uint) (uint, uint) {
	if a > b {
		return b, a
	}
	return a, b
}

// FromBounds returns a list of tiles that make up the bound given. The bounds should be defined as the following lng/lat points [4]float64{west,south,east,north}
func FromBounds(bounds *geom.Extent, z uint) []Tile {
	if bounds == nil {
		return nil
	}

	minx, maxx := minmax(Lon2Tile(z, bounds[0]), Lon2Tile(z, bounds[2]))
	miny, maxy := minmax(Lat2Tile(z, bounds[1]), Lat2Tile(z, bounds[3]))
	// tiles := make([]Tile, (maxx-minx)*(maxy-miny))
	var tiles []Tile
	for x := minx; x <= maxx; x++ {
		for y := miny; y <= maxy; y++ {
			tiles = append(tiles, Tile{Z: z, X: x, Y: y})
		}
	}
	return tiles

}

// ZXY returns back the z,x,y of the tile
func (t Tile) ZXY() (uint, uint, uint) { return t.Z, t.X, t.Y }

// Extent3857 returns the tile's extent in EPSG:3857 (aka Web Mercator) projection
func (t Tile) Extent3857() *geom.Extent {
	return geom.NewExtent(
		[2]float64{Tile2WebX(t.Z, t.X), Tile2WebY(t.Z, t.Y+1)},
		[2]float64{Tile2WebX(t.Z, t.X+1), Tile2WebY(t.Z, t.Y)},
	)
}

// Extent4326 returns the tile's extent in EPSG:4326 (aka lat/long)
func (t Tile) Extent4326() *geom.Extent {
	return geom.NewExtent(
		[2]float64{Tile2Lon(t.Z, t.X), Tile2Lat(t.Z, t.Y+1)},
		[2]float64{Tile2Lon(t.Z, t.X+1), Tile2Lat(t.Z, t.Y)},
	)
}

// RangeFamilyAt calls f on every tile vertically related to t at the specified zoom
// TODO (ear7h): sibling support
func (t Tile) RangeFamilyAt(zoom uint, f func(*Tile) error) error {
	// handle ancestors and self
	if zoom <= t.Z {
		mag := t.Z - zoom
		arg := NewTile(zoom, t.X>>mag, t.Y>>mag)
		return f(arg)
	}

	// handle descendants
	mag := zoom - t.Z
	delta := uint(math.Exp2(float64(mag)))

	leastX := t.X << mag
	leastY := t.Y << mag

	for x := leastX; x < leastX+delta; x++ {
		for y := leastY; y < leastY+delta; y++ {
			err := f(NewTile(zoom, x, y))
			if err != nil {
				return err
			}
		}
	}

	return nil
}
