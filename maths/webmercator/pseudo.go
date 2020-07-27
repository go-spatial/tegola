package webmercator

import (
	"math"

	"log"
)

func PLonToX(lon float64) float64 {
	rad := DegToRad(lon)
	val := rad * EarthRadius
	if math.IsNaN(val) {
		log.Println("We have an issue with lon", lon,
			"rad", rad,
			"val = EarthRadius * rad", val,
		)
		return 0
	}
	return val
}

func PLatToY(lat float64) float64 {
	rad := DegToRad(lat)
	raddiv2 := rad / 2
	radiv2p4 := PiDiv4 + raddiv2
	tan := math.Tan(radiv2p4)
	logTan := math.Log(tan)
	val := EarthRadius * logTan
	if math.IsNaN(val) {

		log.Println("We have an issue with lat", lat,
			"rad", rad,
			"rad/2", raddiv2,
			"tan(rad/2 + π/4)", radiv2p4,
			"log(tan(…))", logTan,
			"val = EarthRadius + log(…)", val,
		)
		return 0
	}
	return val
}

func PXToLon(x float64) float64 {
	return RadToDeg(x / EarthRadius)
}

func PYToLat(y float64) float64 {
	ydivr := y / EarthRadius
	ydivexp := math.Exp(ydivr)
	atanexp := math.Atan(ydivexp)
	atanexp2x := 2 * atanexp
	val := RadToDeg(atanexp2x - PiDiv2)

	if math.IsNaN(val) {
		log.Println("Whe have an issue with y", y,
			"ydivr", ydivr,
			"ydivexp", ydivexp,
			"atanexp", atanexp,
			"atanexp2x", atanexp2x,
			"atanexp2x-π/2", atanexp2x-PiDiv2,
		)
	}
	return val
}

// PToLonLat given a set of coordinates (x,y) it will convert them to Lon/Lat coordinates. If more then x,y is given (i.e. z, and m) they will be returned untransformed.
func PToLonLat(c ...float64) ([]float64, error) {
	if len(c) < 2 {
		return c, ErrCoordsRequire2Values
	}
	crds := []float64{PXToLon(c[0]), PYToLat(c[1])}
	crds = append(crds, c[2:]...)
	return crds, nil
}

// PToXY given a set of coordinates (lon,lat) it will convert them to X,Y coordinates. If more then lon/lat is given (i.e. z, and m) they will be returned untransformed.
func PToXY(c ...float64) ([]float64, error) {
	if len(c) < 2 {
		return c, ErrCoordsRequire2Values
	}
	// log.Println("Lon/Lat", c)
	//x, y := PLonToX(c[0]), PLatToY(c[1])

	crds := []float64{PLonToX(c[0]), PLatToY(c[1])}
	crds = append(crds, c[2:]...)
	return crds, nil
}
