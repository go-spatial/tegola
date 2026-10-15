package webmercator

import (
	"math"

	"github.com/alexeydott/tegola/internal/log"
)

// PLonToX projects a WGS84 longitude to a Web Mercator x coordinate.
//
// NaN input propagates to NaN output (audit P5-14): an invalid longitude
// yields an invalid x and must never be silently replaced with a finite
// value such as x = 0.
func PLonToX(lon float64) float64 {
	return DegToRad(lon) * EarthRadius
}

// MaxLatitude is the largest absolute latitude Web Mercator can represent; the
// projection runs to infinity at the poles and the standard web-mercator tiling
// scheme stops at ±85.05112878°.
const MaxLatitude = 85.05112878

// PLatToY projects a WGS84 latitude to a Web Mercator y coordinate.
//
// Web Mercator is defined only for |lat| <= MaxLatitude (±85.05112878). Latitudes
// beyond that range — including corrupt values such as |lat| > 90 — are clamped
// to the nearest valid edge. Clamping was chosen over returning an error to keep
// the function signature used throughout the render pipeline unchanged
// (minimal-invasive fix, documented here per the audit).
//
// The previous behavior silently returned y = 0 (the equator) whenever the
// projection math produced NaN (any |lat| > 90 makes log(tan(π/4 + rad/2))
// NaN), drawing invalid coordinates on the equator. With clamping they land on
// the map edge instead. A NaN input has no edge to clamp to and propagates as a
// NaN y — visible downstream rather than collapsing to the equator. Valid
// in-range latitudes are projected exactly as before.
func PLatToY(lat float64) float64 {
	if math.IsNaN(lat) {
		return math.NaN()
	}
	if lat > MaxLatitude {
		lat = MaxLatitude
	} else if lat < -MaxLatitude {
		lat = -MaxLatitude
	}
	rad := DegToRad(lat)
	raddiv2 := rad / 2
	radiv2p4 := PiDiv4 + raddiv2
	tan := math.Tan(radiv2p4)
	logTan := math.Log(tan)
	return EarthRadius * logTan
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
		log.Warnf("Whe have an issue with y %v ydivr %v ydivexp %v atanexp %v atanexp2x %v atanexp2x-π/2 %v", y,
			ydivr,
			ydivexp,
			atanexp,
			atanexp2x,
			atanexp2x-PiDiv2,
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
//
// NaN contract (audit P5-14): if either the longitude or the latitude is NaN,
// the whole point is invalid and BOTH x and y are returned as NaN. Invalid
// input yields invalid output; it is never silently clamped to a valid
// location such as the equator.
func PToXY(c ...float64) ([]float64, error) {
	if len(c) < 2 {
		return c, ErrCoordsRequire2Values
	}
	// log.Println("Lon/Lat", c)
	//x, y := PLonToX(c[0]), PLatToY(c[1])

	if math.IsNaN(c[0]) || math.IsNaN(c[1]) {
		crds := []float64{math.NaN(), math.NaN()}
		crds = append(crds, c[2:]...)
		return crds, nil
	}

	crds := []float64{PLonToX(c[0]), PLatToY(c[1])}
	crds = append(crds, c[2:]...)
	return crds, nil
}
