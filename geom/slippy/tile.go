package slippy

import "math"

func NewTile(z, x, y uint64) *Tile {
	return &Tile{z, x, y}
}

type Tile struct {
	//	zoom
	Z uint64
	//	column
	X uint64
	//	row
	Y uint64
	//	buffer will add a buffer to the tile bounds
	Buffer uint64
	//	spatial reference id
	SRID uint64
}

func (t *Tile) Zoom() uint64 {
	return t.Z
}

//	TODO: implement tile buffer
//	TODO: support alternative SRIDs. Currently this assumes 3857
//	Extent will return the tile extent and the SRID of the extent
//	if the tile has a buffer, the extent will include the buffer
func (t *Tile) Extent() (extent [2][2]float64, srid uint64) {
	max := 20037508.34

	//	resolution
	res := (max * 2) / math.Exp2(float64(t.Z))

	return [2][2]float64{
		{
			-max + (float64(t.X) * res), // MinX
			max - (float64(t.Y) * res),  // Miny
		},
		{
			-max + (float64(t.X) * res) + res, // MaxX
			max - (float64(t.Y) * res) - res,  // MaxY

		},
	}
}
