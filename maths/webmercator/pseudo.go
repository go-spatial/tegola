package webmercator

import (
	"fmt"
	"math"

	"github.com/terranodo/tegola/maths"
)

const (
	EarthRadius = RMajor
)

func PLonToX(lon float64) float64 {
	return maths.DegToRad(lon) * EarthRadius
}

func PLatToY(lat float64) float64 {
	return EarthRadius * math.Log(math.Tan(maths.PiDiv4+maths.DegToRad(lat)/2))
}

func PXToLon(x float64) float64 {
	return maths.RadToDeg(x / EarthRadius)
}

func PYToLat(y float64) float64 {
	return maths.RadToDeg(2*math.Atan(math.Exp((y/EarthRadius))) - maths.PiDiv2)
}

// PToLonLat given a set of coordinates (x,y) it will convert them to Lon/Lat coordinates. If more then x,y is given (i.e. z, and m) they will be returned untransformed.
func PToLonLat(c ...float64) ([]float64, error) {
	if len(c) < 2 {
		return c, fmt.Errorf("Coords should have at least 2 coords")
	}
	crds := []float64{PXToLon(c[0]), PYToLat(c[1])}
	crds = append(crds, c[2:]...)
	return crds, nil
}

// PToXY given a set of coordinates (lon,lat) it will convert them to X,Y coordinates. If more then lon/lat is given (i.e. z, and m) they will be returned untransformed.
func PToXY(c ...float64) ([]float64, error) {
	if len(c) < 2 {
		return c, fmt.Errorf("Coords should have at least 2 coords")
	}
	crds := []float64{PLonToX(c[0]), PLatToY(c[1])}
	crds = append(crds, c[2:]...)
	return crds, nil
}
