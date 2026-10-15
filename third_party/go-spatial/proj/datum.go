package proj

import (
	"math"

	"github.com/go-spatial/proj/core"
)

var wgs84Ellipsoid = ellipsoid{a: 6378137, e2: 0.0066943799901413165}

type ellipsoid struct {
	a  float64
	e2 float64
}

type geodetic struct {
	lon float64
	lat float64
	h   float64
}

func datumToWGS84(system *core.System, lp *core.CoordLP) {
	if system == nil || lp == nil || system.DatumType != core.DatumType3Param && system.DatumType != core.DatumType7Param {
		return
	}

	source := ellipsoid{a: system.Ellipsoid.A, e2: system.Ellipsoid.Es}
	xyz := geodeticToECEF(source, geodetic{lon: lp.Lam, lat: lp.Phi})
	xyz = helmert(xyz, system.DatumParams, false)
	out := ecefToGeodetic(wgs84Ellipsoid, xyz)
	lp.Lam, lp.Phi = out.lon, out.lat
}

func datumFromWGS84(system *core.System, lp *core.CoordLP) {
	if system == nil || lp == nil || system.DatumType != core.DatumType3Param && system.DatumType != core.DatumType7Param {
		return
	}

	xyz := geodeticToECEF(wgs84Ellipsoid, geodetic{lon: lp.Lam, lat: lp.Phi})
	xyz = helmert(xyz, system.DatumParams, true)
	source := ellipsoid{a: system.Ellipsoid.A, e2: system.Ellipsoid.Es}
	out := ecefToGeodetic(source, xyz)
	lp.Lam, lp.Phi = out.lon, out.lat
}

func geodeticToECEF(e ellipsoid, p geodetic) [3]float64 {
	sinLat, cosLat := math.Sin(p.lat), math.Cos(p.lat)
	sinLon, cosLon := math.Sin(p.lon), math.Cos(p.lon)
	n := e.a / math.Sqrt(1-e.e2*sinLat*sinLat)
	return [3]float64{
		(n + p.h) * cosLat * cosLon,
		(n + p.h) * cosLat * sinLon,
		(n*(1-e.e2) + p.h) * sinLat,
	}
}

func ecefToGeodetic(e ellipsoid, xyz [3]float64) geodetic {
	lon := math.Atan2(xyz[1], xyz[0])
	p := math.Hypot(xyz[0], xyz[1])
	lat := math.Atan2(xyz[2], p*(1-e.e2))

	for i := 0; i < 10; i++ {
		sinLat := math.Sin(lat)
		n := e.a / math.Sqrt(1-e.e2*sinLat*sinLat)
		h := p/math.Cos(lat) - n
		next := math.Atan2(xyz[2], p*(1-e.e2*n/(n+h)))
		if math.Abs(next-lat) < 1e-13 {
			lat = next
			break
		}
		lat = next
	}

	sinLat := math.Sin(lat)
	n := e.a / math.Sqrt(1-e.e2*sinLat*sinLat)
	h := p/math.Cos(lat) - n
	return geodetic{lon: lon, lat: lat, h: h}
}

func helmert(xyz [3]float64, params [7]float64, inverse bool) [3]float64 {
	dx, dy, dz := params[0], params[1], params[2]
	rx, ry, rz := params[3], params[4], params[5]
	scale := params[6]
	if scale == 0 {
		scale = 1
	}

	if inverse {
		x := (xyz[0] - dx) / scale
		y := (xyz[1] - dy) / scale
		z := (xyz[2] - dz) / scale
		return [3]float64{x + rz*y - ry*z, -rz*x + y + rx*z, ry*x - rx*y + z}
	}

	x, y, z := xyz[0], xyz[1], xyz[2]
	return [3]float64{
		dx + scale*(x-rz*y+ry*z),
		dy + scale*(rz*x+y-rx*z),
		dz + scale*(-ry*x+rx*y+z),
	}
}
