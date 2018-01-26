package slippy

import "math"

func NewTile(z, x, y, buffer, srid uint64) *Tile {
	return &Tile{
		z:      z,
		x:      x,
		y:      y,
		Buffer: buffer,
		SRID:   srid,
	}
}

type Tile struct {
	//	zoom
	z uint64
	//	column
	x uint64
	//	row
	y uint64
	//	buffer will add a buffer to the tile bounds. this buffer is expected to use the same units as the SRID
	//	of the projected tile (i.e. WebMercator = pixels, 3395 = meters)
	Buffer uint64
	//	spatial reference id
	SRID uint64
}

func (t *Tile) Z() uint64 {
	return t.z
}

func (t *Tile) X() uint64 {
	return t.x
}

func (t *Tile) Y() uint64 {
	return t.y
}

//	TODO: return geom.Extent once it has been refactored
//	TODO: support alternative SRIDs. Currently this assumes 3857
//	Extent will return the tile extent excluding the tile's buffer and the Extent's SRID
func (t *Tile) Extent() (extent [2][2]float64, srid uint64) {
	max := 20037508.34

	//	resolution
	res := (max * 2) / math.Exp2(float64(t.Z()))

	//	unbuffered extent
	return [2][2]float64{
		{
			-max + (float64(t.X()) * res), // MinX
			max - (float64(t.Y()) * res),  // Miny
		},
		{
			-max + (float64(t.X()) * res) + res, // MaxX
			max - (float64(t.Y()) * res) - res,  // MaxY

		},
	}, t.SRID
}

//	BufferedExtent will return the tile extent including the tile's buffer and the Extent's SRID
func (t *Tile) BufferedExtent() (extent [2][2]float64, srid uint64) {
	extent, _ = t.Extent()

	//	MVT defaults
	mvtTileWidthHeight := 4096.0
	mvtBuffer := 64.0
	//	the bounds / extent
	mvtTileExtent := [2][2]float64{{0 - mvtBuffer, 0 - mvtBuffer}, {mvtTileWidthHeight + mvtBuffer, mvtTileWidthHeight + mvtBuffer}}

	xspan := extent[1][0] - extent[0][0]
	yspan := extent[1][1] - extent[0][1]

	extent[0][0] = (mvtTileExtent[0][0] * xspan / mvtTileWidthHeight) + extent[0][0]
	extent[0][1] = (mvtTileExtent[0][1] * yspan / mvtTileWidthHeight) + extent[0][1]
	extent[1][0] = (mvtTileExtent[1][0] * xspan / mvtTileWidthHeight) + extent[0][0]
	extent[1][1] = (mvtTileExtent[1][1] * yspan / mvtTileWidthHeight) + extent[0][1]

	return extent, t.SRID

}
