package slippy

import "math"

// ==== lat lon (aka WGS 84) ====

func Lat2Tile(zoom uint, lat float64) (y uint) {
	lat_rad := lat * math.Pi / 180

	return uint(math.Exp2(float64(zoom))*
		(1.0-math.Log(
			math.Tan(lat_rad)+
				(1/math.Cos(lat_rad)))/math.Pi)) /
		2.0

}

func Lon2Tile(zoom uint, lon float64) (x uint) {
	return uint(math.Exp2(float64(zoom)) * (lon + 180.0) / 360.0)
}

// Tile2Lon will return the west most longitude
func Tile2Lon(zoom, x uint) float64 { return float64(x)/math.Exp2(float64(zoom))*360.0 - 180.0 }

// Tile2Lat will return the north most latitude
func Tile2Lat(zoom, y uint) float64 {
	var n float64 = math.Pi
	if y != 0 {
		n = math.Pi - 2.0*math.Pi*float64(y)/math.Exp2(float64(zoom))
	}

	return 180.0 / math.Pi * math.Atan(0.5*(math.Exp(n)-math.Exp(-n)))
}

// ==== Web Mercator ====

const WebMercatorMax = 20037508.34

// Returns the side of the tile in the -x side
func Tile2WebX(zoom uint, n uint) float64 {
	res := (WebMercatorMax * 2) / math.Exp2(float64(zoom))

	return -WebMercatorMax + float64(n)*res
}

// Returns the side of the tile in the +y side
func Tile2WebY(zoom uint, n uint) float64 {
	res := (WebMercatorMax * 2) / math.Exp2(float64(zoom))

	return WebMercatorMax - float64(n)*res
}

// returns the column of the tile given the web mercator x value
func WebX2Tile(zoom uint, x float64) uint {
	res := (WebMercatorMax * 2) / math.Exp2(float64(zoom))

	return uint((x + WebMercatorMax) / res)
}

// returns the row of the tile given the web mercator y value
func WebY2Tile(zoom uint, y float64) uint {
	res := (WebMercatorMax * 2) / math.Exp2(float64(zoom))

	return uint(-(y - WebMercatorMax) / res)
}

// ==== pixels ====

const MvtTileDim = 4096.0

// Scalar conversion of pixels into web mercator units
// TODO (@ear7h): perhaps rethink this
func Pixels2Webs(zoom uint, pixels uint) float64 {
	return WebMercatorMax * 2 / math.Exp2(float64(zoom)) * float64(pixels) / MvtTileDim
}
