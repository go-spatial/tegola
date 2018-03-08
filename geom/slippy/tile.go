package slippy

import (
	"math"

	"github.com/go-spatial/tegola/geom"
)

func NewTile(z, x, y uint64, buffer float64, srid uint64) *Tile {
	return &Tile{
		z:      z,
		x:      x,
		y:      y,
		Buffer: buffer,
		SRID:   srid,
	}
}

type Tile struct {
	// zoom
	z uint64
	// column
	x uint64
	// row
	y uint64
	// buffer will add a buffer to the tile bounds. this buffer is expected to use the same units as the SRID
	// of the projected tile (i.e. WebMercator = pixels, 3395 = meters)
	Buffer float64
	// spatial reference id
	SRID uint64
}

func (t *Tile) ZXY() (uint64, uint64, uint64) { return t.z, t.x, t.y }

func Tile2Lon(x, z uint64) float64 { return float64(x)/math.Exp2(float64(z))*360.0 - 180.0 }

func Tile2Lat(y, z uint64) float64 {
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
	// This is how we convert from the Bounds, and TileSize to Extent for Webmercator.
	bounds := t.Bounds()
	east,north,west, south := bounds[0],bounds[1],bounds[2],bounds[3]

	TileSize := 4096.0
	c, err := webmercator.PToXY(east, north, west, south)
	log.Println("c", c, "err", err)

	extent := geom.NewBBox(
		[2]float64{c[0], c[1]},
		[2]float64{c[2], c[3]},
	)

	xspan := extent.XSpan()
	yspan := extent.YSpan()

	log.Println("Extent", extent, "MinX", extent.MinX(), "MinY", extent.MinY(), "xspan", xspan, "yspan", yspan)

	// To get the Buffered Extent, we just need the extent and the Buffer size.
	// Convert to tile coordinates.
	nx := float64(int64((c[0] - extent.MinX()) * TileSize / xspan))
	ny := float64(int64((c[1] - extent.MinY()) * TileSize / yspan))
	mx := float64(int64((c[2] - extent.MinX()) * TileSize / xspan))
	my := float64(int64((c[3] - extent.MinY()) * TileSize / yspan))

	mextent := geom.NewBBox([2]float64{nx, ny}, [2]float64{mx, my}).ExpandBy(64)
	log.Println("mxy[", nx, ny, mx, my, "]", "err", err, "mext", mextent)
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
func (t *Tile) Extent() (extent *geom.BoundingBox, srid uint64) {
	max := 20037508.34

	//	resolution
	res := (max * 2) / math.Exp2(float64(t.z))

	//	unbuffered extent
	return geom.NewBBox(
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
func (t *Tile) BufferedExtent() (bufferedExtent *geom.BoundingBox, srid uint64) {
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

	bufferedExtent = geom.NewBBox(
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
