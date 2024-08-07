package slippy

import (
	"math"

	"github.com/go-spatial/geom"
)

/*
 *
 * This file should contain the basic math function for converting
 * between coordinates that are internal to the system.
 *
 * Much of the math here is derived from two sources:
 *  ref: https://maplibre.org/maplibre-native/docs/book/design/coordinate-system.html#11
 *  ref: https://wiki.openstreetmap.org/wiki/Slippy_map_tilenames#ECMAScript_(JavaScript/ActionScript,_etc.)
 */

const (
	// DefaultTileSize is the tile size used if the given tile size is 0.
	DefaultTileSize = 256
	// Lat4326Max is the maximum degree for latitude on an SRID 4326 map
	Lat4326Max = 85.05112
	// Lon4326Max is the maximum degree for longitude on an SRID 4326 map
	Lon4326Max = 180

	// floatVariance is used to compare floating point numbers, and to deal with float drift
	//
	//  This is mainly used to nudge the calculated tile values into place.
	//  If the calculated number is extremely close to the border — basically on it — bumping it so that it falls
	//    into the right most or bottom most tiles, where it should be. Since the variance from the floating point
	//    calculation could be either positive or negative. If it's negative it would end up in the tile that is to the
	//    left (leading possibility to a negative tile number) or top tile.
	//  Take for example a calculated value of 6.99999999 v.s.7.000001 as a float these are practically the same number,
	//    but we need them to be 7+, as we will truncate the float in the next step
	//    (the fractional part is the percentage into the tile where the pixel is), thus it is better to bump toward 7
	//    by the way of a very small step. This is the floatVariance value we use.
	floatVariance = 0.000001
)

// Degree2Radians converts degrees to radians
func Degree2Radians(degree float64) float64 {
	return degree * math.Pi / 180
}

// Radians2Degree converts radians to degrees
func Radians2Degree(radians float64) float64 {
	return radians * 180 / math.Pi
}

// lat2Num will return the Y coordinate for the tile at the given Z.
//
//	Lat is assumed to be in degrees in SRID 3857 coordinates
//	If tileSize == 0 then we will use a tileSize of DefaultTileSize
func lat2Num(tileSize uint32, z Zoom, lat float64) (y int) {
	if tileSize == 0 {
		tileSize = DefaultTileSize
	}
	// bound it because we have a top of the world problem
	if lat < -Lat4326Max {
		return int(z.N() - 1)
	}

	if lat > Lat4326Max {
		return 0
	}
	tileY := lat2Px(tileSize, z, lat)
	tileY = tileY / float64(tileSize)
	// Truncate to get the tile
	return int(tileY)
}

// lat2Px will return the pixel coordinate for the lat. This can return
// a pixel that is outside the extents of the map, this just means
// the drawing is happening in the buffered area usually done for stitching
// purposes.
func lat2Px(tileSize uint32, z Zoom, lat float64) (yPx float64) {
	if tileSize == 0 {
		tileSize = DefaultTileSize
	}
	worldSize := float64(tileSize) * z.N()

	// Convert the Degree to radians as most of the math functions work in radians
	radLat := Degree2Radians(45 + lat/2)
	// normalize lat
	latTan := math.Tan(radLat)
	latNormalized := math.Log(latTan)

	// compute the pixel value for y:
	yPxRaw := (180 - Radians2Degree(latNormalized)) / 360
	yPx = yPxRaw * worldSize
	// instead of getting 7.0 we can end up with 6.9999999999, etc... use floatVariance to correct for such cases
	return yPx + floatVariance
}

// lon2Num will return the Y coordinate for the tile at the given Z.
//
//	Lat is assumed to be in degrees in SRID 3857 coordinates
//	If tileSize == 0 then we will use a tileSize of DefaultTileSize
func lon2Num(tileSize uint32, z Zoom, lon float64) (x int) {
	if tileSize == 0 {
		tileSize = DefaultTileSize
	}

	if lon <= -Lon4326Max {
		return 0
	}

	if lon >= Lon4326Max {
		return int(z.N() - 1)
	}

	tileX := lon2Px(tileSize, z, lon)
	tileX = tileX / float64(tileSize)
	// Truncate to get the tile
	return int(tileX)

}

// lonPx will return the pixel coordinate for the lon. This can return
// a pixel that is outside the extents of the map, this just means
// the drawing is happening in the buffered area usually done for stitching
// purposes.
func lon2Px(tileSize uint32, z Zoom, lon float64) (xPx float64) {
	if tileSize == 0 {
		tileSize = DefaultTileSize
	}
	worldSize := float64(tileSize) * z.N()
	lonNormalized := 180 + lon
	// compute the pixel value for x:
	xPxRaw := lonNormalized / 360
	xPx = xPxRaw * worldSize
	// instead of getting 7.0 we can end up with 6.9999999999, etc... use floatVariance to correct for such cases
	return xPx + floatVariance
}

func PtFromLatLon(lat, lon float64) geom.Point {
	return geom.Point{lon, lat}
}

func x2deg(z Zoom, x int) float64 {
	n := z.N()
	long := float64(x) / n
	long = long * 360.0
	long = long - 180.0
	return long
}

func y2deg(z Zoom, y int) float64 {
	n := math.Pi - 2.0*math.Pi*float64(y)/z.N()
	lat := 180.0 / math.Pi * math.Atan(0.5*(math.Exp(n)-math.Exp(-n)))
	return lat
}
