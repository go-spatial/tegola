package tegola

import (
	"math"
)

const (
	DefaultEpislon = 5
	DefaultExtent  = 4096
)

//Tile slippy map tilenames
//	http://wiki.openstreetmap.org/wiki/Slippy_map_tilenames
type Tile struct {
	Z         int
	X         int
	Y         int
	Lat       float64
	Long      float64
	Tolerance *float64
	Extent    *float64
}

func (t *Tile) Deg2Num() (x, y int) {
	x = int(math.Floor((t.Long + 180.0) / 360.0 * (math.Exp2(float64(t.Z)))))
	y = int(math.Floor((1.0 - math.Log(math.Tan(t.Lat*math.Pi/180.0)+1.0/math.Cos(t.Lat*math.Pi/180.0))/math.Pi) / 2.0 * (math.Exp2(float64(t.Z)))))

	return
}

func (t *Tile) Num2Deg() (lat, lng float64) {
	n := math.Pi - 2.0*math.Pi*float64(t.Y)/math.Exp2(float64(t.Z))

	lat = 180.0 / math.Pi * math.Atan(0.5*(math.Exp(n)-math.Exp(-n)))
	lng = float64(t.X)/math.Exp2(float64(t.Z))*360.0 - 180.0

	return lat, lng
}

//BoundingBox returns the bound box coordinates for upper left (ulx, uly) and lower right (lrx, lry)
// in web mercator projection
// ported from: https://raw.githubusercontent.com/mapbox/postgis-vt-util/master/postgis-vt-util.sql
func (t *Tile) BoundingBox() BoundingBox {
	max := 20037508.34

	//	resolution
	res := (max * 2) / math.Exp2(float64(t.Z))

	return BoundingBox{
		Minx:    -max + (float64(t.X) * res),
		Miny:    max - (float64(t.Y) * res),
		Maxx:    -max + (float64(t.X) * res) + res,
		Maxy:    max - (float64(t.Y) * res) - res,
		Epsilon: t.ZEpislon(),
		HasXYZ:  true,
		X:       t.X,
		Y:       t.Y,
		Z:       t.Z,
	}
}

//ZRes takes a web mercator zoom level and returns the pixel resolution for that
//	scale, assuming 256x256 pixel tiles. Non-integer zoom levels are accepted.
//	ported from: https://raw.githubusercontent.com/mapbox/postgis-vt-util/master/postgis-vt-util.sql
func (t *Tile) ZRes() float64 {
	return 40075016.6855785 / (256 * math.Exp2(float64(t.Z)))
}

// This is from Leafty
func (t *Tile) ZEpislon() float64 {

	if t.Z == 20 {
		return 0
	}
	// return DefaultEpislon
	epi := float64(DefaultEpislon)
	if t.Tolerance != nil {
		epi = *t.Tolerance
	}
	if epi <= 0 {
		return 0
	}

	ext := float64(DefaultExtent)
	if t.Extent != nil {
		ext = *t.Extent
	}
	denom := (math.Exp2(float64(t.Z)) * ext)

	/*
		exp := t.Z - 1
		if exp < 0 {
			exp = 0
		}
		denom := math.Exp2(float64(exp))
	*/
	e := epi / denom
	return e

}
