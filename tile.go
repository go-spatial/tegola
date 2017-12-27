package tegola

import (
	"fmt"
	"log"
	"math"
)

const (
	DefaultEpislon = 10.0
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

func (t *Tile) Deg2Num() (x, y int, err error) {
	// Converts from WGS84 Lat/Long postion to tile position (x,y)

	// Check that input lat/long values are within WGS84 bounds
	if t.Lat < -85.0511 || t.Lat > 85.0511 || t.Long < -180.0 || t.Long > 180.0 {
		err := fmt.Errorf("one or both outside valid range (Long, Lat): (%v, %v)", t.Long, t.Lat)
		log.Print(err)
		return -1, -1, err
	}

	x = int(math.Floor((t.Long + 180.0) / 360.0 * (math.Exp2(float64(t.Z)))))
	y = int(math.Floor((1.0 - math.Log(math.Tan(t.Lat*math.Pi/180.0)+1.0/math.Cos(t.Lat*math.Pi/180.0))/math.Pi) / 2.0 * (math.Exp2(float64(t.Z)))))

	return
}

func (t *Tile) Num2Deg() (lat, lng float64, err error) {
	//	WGS84Bounds       = [4]float64{-180.0, -85.0511, 180.0, 85.0511}
	//	WebMercatorBounds = [4]float64{-20026376.39, -20048966.10, 20026376.39, 20048966.10}
	n := math.Pi - 2.0*math.Pi*float64(t.Y)/math.Exp2(float64(t.Z))

	lat = 180.0 / math.Pi * math.Atan(0.5*(math.Exp(n)-math.Exp(-n)))
	lng = float64(t.X)/math.Exp2(float64(t.Z))*360.0 - 180.0

	if lat < -85.0511 || lat > 85.0511 || lng < -180.0 || lng > 180.0 {
		err := fmt.Errorf("one or both outside valid range (x, y): (%v, %v)", t.X, t.Y)
		log.Print(err)
		return -400.0, -400.0, err
	}

	return lat, lng, nil
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

	e := epi / denom
	return e

}
