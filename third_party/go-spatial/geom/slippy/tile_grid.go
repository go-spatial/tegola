package slippy

import (
	"fmt"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/proj"
)

// TileGridder contains the tile layout, including ability to get WGS84 coordinates for tile extents
type TileGridder interface {
	// SRID returns the SRID of the coordinate system of the
	// implementer. The geometries returned by the other methods
	// will be in these coordinates.
	SRID() proj.EPSGCode

	// Size returns a tile where the X and Y are the size of that zoom's
	// tile grid. AKA:
	//	Tile{z, MaxX + 1, MaxY + 1
	Size(z Zoom) (Tile, bool)

	// FromNative converts from a point (in the Grid's coordinates system) and zoom
	// to a tile.
	FromNative(z Zoom, pt geom.Point) (tile Tile, err error)

	// ToNative returns the tiles upper left point. ok will be false if
	// the tile is not valid. A note on implementation is that this method
	// should be able to take tiles with x and y values 1 higher than the max,
	// this is to fetch the bottom right corner of the grid
	ToNative(Tile) (pt geom.Point, err error)
}

// NewGrid will return a grid for the requested EPSGCode.
// if tileSize is zero, then the DefaultTileSize is used
func NewGrid(srid proj.EPSGCode, tileSize uint32) TileGridder {
	if tileSize == 0 {
		tileSize = DefaultTileSize
	}
	if srid == proj.EPSG4326 {
		return Grid4326{tileSize: tileSize}
	}
	return Grid{
		tileSize: tileSize,
		Srid:     srid,
	}
}

func Extent(g TileGridder, t Tile) (*geom.Extent, error) {
	topLeft, err := g.ToNative(t)
	if err != nil {
		return nil, fmt.Errorf("failed get top left point: %w", err)
	}
	bottomRight, err := g.ToNative(Tile{Z: t.Z, X: t.X + 1, Y: t.Y + 1})
	if err != nil {
		return nil, fmt.Errorf("failed get bottom right point: %w", err)
	}
	return geom.NewExtentFromPoints(topLeft, bottomRight), nil
}

func NewTileMinMaxer(g TileGridder, ext geom.MinMaxer) (Tile, error) {
	tile, err := g.FromNative(MaxZoom, geom.Point{ext.MinX(), ext.MinY()})
	if err != nil {
		return Tile{}, fmt.Errorf("failed get tile for min points: %w", err)
	}
	var (
		ret   Tile
		found bool
		ext1  *geom.Extent
	)

	for z := Zoom(MaxZoom); int(z) >= 0 && !found; z-- {
		err = nil
		tile.FamilyAt(z)(func(tile Tile) bool {
			ext1, err = Extent(g, tile)
			if err != nil {
				// stop iteration
				return false
			}
			if ext1.Contains(geom.Point(ext1.Max())) {
				ret = tile
				found = true
				return false
			}
			return true
		})
		if err != nil {
			return Tile{}, fmt.Errorf("failed get min tile: %w", err)
		}
	}
	if !found {
		return Tile{}, fmt.Errorf("tile for min point not found")
	}

	return ret, nil
}

type Grid struct {
	tileSize uint32
	Srid     proj.EPSGCode
}

func (g Grid) SRID() proj.EPSGCode { return g.Srid }
func (g Grid) Size(z Zoom) (Tile, bool) {
	if z > MaxZoom {
		return Tile{}, false
	}
	return z.TileSize(), true
}
func (g Grid) FromNative(z Zoom, pt geom.Point) (tile Tile, err error) {
	pts, err := proj.Inverse(g.Srid, pt[:])
	if err != nil {
		return Tile{}, fmt.Errorf("failed to convert to 4326: %w", err)
	}
	x := lon2Num(g.tileSize, z, pts[0])
	y := lat2Num(g.tileSize, z, pts[1])
	return Tile{
		Z: z,
		X: uint(x),
		Y: uint(y),
	}, nil
}
func (g Grid) ToNative(tile Tile) (pt geom.Point, err error) {
	lat := y2deg(tile.Z, int(tile.Y))
	lon := x2deg(tile.Z, int(tile.X))
	pts, err := proj.Convert(g.Srid, []float64{lon, lat})
	if err != nil {
		return geom.Point{}, fmt.Errorf("failed to convert from 4326: %w", err)
	}
	return geom.Point{pts[0], pts[1]}, nil
}

type Grid4326 struct {
	// TileSize if 0 will default to DefaultTileSize
	tileSize uint32
}

func (Grid4326) SRID() proj.EPSGCode { return proj.EPSG4326 }
func (Grid4326) Size(z Zoom) (Tile, bool) {
	if z > MaxZoom {
		return Tile{}, false
	}
	return z.TileSize(), true
}

func (g Grid4326) TileSize() uint32 {
	if g.tileSize == 0 {
		return DefaultTileSize
	}
	return g.tileSize
}

// FromNative will convert a pt in 3857 coordinates and a zoom to a Tile coordinate
func (g Grid4326) FromNative(z Zoom, pt geom.Point) (tile Tile, err error) {
	y := lat2Num(g.tileSize, z, pt.Lat())
	x := lon2Num(g.tileSize, z, pt.Lon())
	return Tile{
		Z: z,
		X: uint(x),
		Y: uint(y),
	}, nil
}
func (g Grid4326) ToNative(tile Tile) (pt geom.Point, err error) {
	lat := y2deg(tile.Z, int(tile.Y))
	lon := x2deg(tile.Z, int(tile.X))
	return PtFromLatLon(lat, lon), nil
}
