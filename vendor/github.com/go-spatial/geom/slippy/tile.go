package slippy

import (
	"math"

	"github.com/go-spatial/geom"
)

func NewTile(z, x, y uint, buffer float64, srid uint64) *Tile {
	return &Tile{
		z:      z,
		x:      x,
		y:      y,
		Buffer: buffer,
		SRID:   srid,
	}
}

// Tile describes a slippy tile.
type Tile struct {
	// zoom
	z uint
	// column
	x uint
	// row
	y uint
	// buffer will add a buffer to the tile bounds. this buffer is expected to use the same units as the SRID
	// of the projected tile (i.e. WebMercator = pixels, 3395 = meters)
	Buffer float64
	// spatial reference id
	SRID uint64
}

func (t *Tile) ZXY() (uint, uint, uint) { return t.z, t.x, t.y }

// Tile2Lon will return the west most longitude
func Tile2Lon(x, z uint) float64 { return float64(x)/math.Exp2(float64(z))*360.0 - 180.0 }

// Tile2Lat will return the east most Latitude
func Tile2Lat(y, z uint) float64 {
	var n float64 = math.Pi
	if y != 0 {
		n = math.Pi - 2.0*math.Pi*float64(y)/math.Exp2(float64(z))
	}

	return 180.0 / math.Pi * math.Atan(0.5*(math.Exp(n)-math.Exp(-n)))
}

// Bounds returns the bounds of the Tile as defined by the East most longitude, North most latitude, West most longitude, South most latitude.
func (t *Tile) Bounds() [4]float64 {
	east := Tile2Lon(t.x, t.z)
	west := Tile2Lon(t.x+1, t.z)
	north := Tile2Lat(t.y, t.z)
	south := Tile2Lat(t.y+1, t.z)

	return [4]float64{east, north, west, south}
}

/*
	// Keep this comment as it is a guide for how we can take bounds and a srid and convert it to Extents and Buffereded Extents.
	// This is how we convert from the Bounds, and TileSize to Extent for Webmercator.
	bounds := t.Bounds()
	east,north,west, south := bounds[0],bounds[1],bounds[2],bounds[3]

	TileSize := 4096.0
	// Convert bounds to coordinates in webmercator.
	c, err := webmercator.PToXY(east, north, west, south)
	log.Println("c", c, "err", err)

	// Turn the Coordinates into an Extent (minx, miny, maxx, maxy)
	// Here is where the origin flip happens if there is one.
	extent := geom.NewBBox(
		[2]float64{c[0], c[1]},
		[2]float64{c[2], c[3]},
	)

	// A Span is just MaxX - MinX
	xspan := extent.XSpan()
	yspan := extent.YSpan()

	log.Println("Extent", extent, "MinX", extent.MinX(), "MinY", extent.MinY(), "xspan", xspan, "yspan", yspan)

	// To get the Buffered Extent, we just need the extent and the Buffer size.
	// Convert to tile coordinates. Convert the meters (WebMercator) into pixels of the tile..
	nx := float64(int64((c[0] - extent.MinX()) * TileSize / xspan))
	ny := float64(int64((c[1] - extent.MinY()) * TileSize / yspan))
	mx := float64(int64((c[2] - extent.MinX()) * TileSize / xspan))
	my := float64(int64((c[3] - extent.MinY()) * TileSize / yspan))

	// Expend by the that number of pixels. We could also do the Expand on the Extent instead, of the Bounding Box on the Pixel.
	mextent := geom.NewBBox([2]float64{nx, ny}, [2]float64{mx, my}).ExpandBy(64)
	log.Println("mxy[", nx, ny, mx, my, "]", "err", err, "mext", mextent)

	// Convert Pixel back to meters.
	bext := geom.NewBBox(
		[2]float64{
			(mextent.MinX() * xspan / TileSize) + extent.MinX(),
			(mextent.MinY() * yspan / TileSize) + extent.MinY(),
		},
		[2]float64{
			(mextent.MaxX() * xspan / TileSize) + extent.MinX(),
			(mextent.MaxY() * yspan / TileSize) + extent.MinY(),
		},
	)
	log.Println("bext", bext)
*/

// TODO(arolek): support alternative SRIDs. Currently this assumes 3857
// Extent will return the tile extent excluding the tile's buffer and the Extent's SRID
func (t *Tile) Extent() (extent *geom.Extent, srid uint64) {
	max := 20037508.34

	// resolution
	res := (max * 2) / math.Exp2(float64(t.z))

	// unbuffered extent
	return geom.NewExtent(
		[2]float64{
			-max + (float64(t.x) * res), // MinX
			max - (float64(t.y) * res),  // Miny
		},
		[2]float64{
			-max + (float64(t.x) * res) + res, // MaxX
			max - (float64(t.y) * res) - res,  // MaxY
		},
	), t.SRID
}

// BufferedExtent will return the tile extent including the tile's buffer and the Extent's SRID
func (t *Tile) BufferedExtent() (bufferedExtent *geom.Extent, srid uint64) {
	extent, _ := t.Extent()

	// TODO(arolek): the following value is hard coded for MVT, but this concept needs to be abstracted to support different projections
	mvtTileWidthHeight := 4096.0
	// the bounds / extent
	mvtTileExtent := [4]float64{
		0 - t.Buffer, 0 - t.Buffer,
		mvtTileWidthHeight + t.Buffer, mvtTileWidthHeight + t.Buffer,
	}

	xspan := extent.MaxX() - extent.MinX()
	yspan := extent.MaxY() - extent.MinY()

	bufferedExtent = geom.NewExtent(
		[2]float64{
			(mvtTileExtent[0] * xspan / mvtTileWidthHeight) + extent.MinX(),
			(mvtTileExtent[1] * yspan / mvtTileWidthHeight) + extent.MinY(),
		},
		[2]float64{
			(mvtTileExtent[2] * xspan / mvtTileWidthHeight) + extent.MinX(),
			(mvtTileExtent[3] * yspan / mvtTileWidthHeight) + extent.MinY(),
		},
	)
	return bufferedExtent, t.SRID
}
