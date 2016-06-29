package tegola

import (
	"math"
)

//	slippy map tilenames
//	http://wiki.openstreetmap.org/wiki/Slippy_map_tilenames
type Tile struct {
	Z    int
	X    int
	Y    int
	Lat  float64
	Long float64
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

//	returns the bound box coordinates for upper left (ulx, uly) and lower right (lrx, lry)
//	in web mercator projection
func (t *Tile) BBox() (ulx, uly, llx, lly float64) {
	max := 20037508.34

	//	resolution
	res := (max * 2) / math.Exp2(float64(t.Z))

	//	upper left point
	ulx = -max + (float64(t.X) * res)
	uly = max - (float64(t.Y) * res)
	//	lower left point
	llx = -max + (float64(t.X) * res) + res
	lly = max - (float64(t.Y) * res) - res

	return
}

func (t *Tile) ZRes() float64 {
	return 40075016.6855785 / (256 * math.Exp2(float64(t.Z)))
}
