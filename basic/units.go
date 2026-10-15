package basic

import (
	"fmt"
	"math"

	"github.com/alexeydott/proj"
	"github.com/alexeydott/proj/core"
	"github.com/alexeydott/proj/support"
	"github.com/alexeydott/tegola"
)

// IsGeographicSRID reports whether coordinates are longitude/latitude
// in degrees, either EPSG:4326 or a supported registered geographic definition.
func IsGeographicSRID(srid uint64) bool {
	if srid == tegola.WGS84 {
		return true
	}
	// EPSG:3857 always follows the native WebMercator path.
	if srid == tegola.WebMercator {
		return false
	}
	proj4RegisteredMu.Lock()
	defn := proj4Registered[srid]
	proj4RegisteredMu.Unlock()
	return proj.IsGeographicDefinition(defn)
}

// GeographicMetersPerLongitudeDegree returns the local length of one degree
// along the source CRS ellipsoid's parallel at latitude (in source CRS degrees).
// It uses N(phi)*cos(phi)*pi/180, where N=a/sqrt(1-e²*sin²(phi)) is the
// prime-vertical radius of curvature. It is a local horizontal scale, not the
// shortest geodesic between two points or the length of a latitude degree.
// Longitude is singular at the poles; those latitudes return an error.
func GeographicMetersPerLongitudeDegree(srid uint64, latitude float64) (float64, error) {
	if math.IsNaN(latitude) || math.IsInf(latitude, 0) || math.Abs(latitude) >= 90 {
		return 0, fmt.Errorf("invalid geographic scale latitude: %v", latitude)
	}
	a, e2 := 6378137.0, 0.0066943799901413165 // EPSG:4326, WGS84 ellipsoid.
	if srid != tegola.WGS84 {
		proj4RegisteredMu.Lock()
		defn := proj4Registered[srid]
		proj4RegisteredMu.Unlock()
		if srid == tegola.WebMercator || !proj.IsGeographicDefinition(defn) {
			return 0, fmt.Errorf("no geographic ellipsoid definition for SRID %d", srid)
		}
		ps, err := support.NewProjString(defn)
		if err != nil {
			return 0, fmt.Errorf("SRID %d ellipsoid: %w", srid, err)
		}
		// Share the projection engine's datum/ellipsoid resolution and precedence.
		sys, err := core.NewGeographicSystem(ps)
		if err != nil {
			return 0, fmt.Errorf("SRID %d ellipsoid: %w", srid, err)
		}
		a, e2 = sys.Ellipsoid.A, sys.Ellipsoid.Es
	}
	phi := latitude * math.Pi / 180
	sinPhi, cosPhi := math.Sincos(phi)
	meters := a / math.Sqrt(1-e2*sinPhi*sinPhi) * cosPhi * math.Pi / 180
	if meters <= 0 || math.IsNaN(meters) || math.IsInf(meters, 0) {
		return 0, fmt.Errorf("SRID %d has invalid geographic scale at latitude %v", srid, latitude)
	}
	return meters, nil
}

// ProjectedMetersPerUnit returns the linear unit conversion used by the
// registered projection. Geographic and unknown CRSs return an error.
func ProjectedMetersPerUnit(srid uint64) (float64, error) {
	if IsGeographicSRID(srid) {
		return 0, fmt.Errorf("SRID %d has geographic angular units", srid)
	}
	// FromWebMercator handles 3857 directly, regardless of registrations.
	if srid == tegola.WebMercator {
		return 1, nil
	}
	proj4RegisteredMu.Lock()
	defn, ok := proj4Registered[srid]
	proj4RegisteredMu.Unlock()
	if !ok {
		switch srid {
		case 3395, 4087:
			return 1, nil
		default:
			return 0, fmt.Errorf("no projected unit definition for SRID %d", srid)
		}
	}
	ps, err := support.NewProjString(defn)
	if err != nil {
		return 0, fmt.Errorf("SRID %d units: %w", srid, err)
	}
	// Use the projection engine's unit parser, including +units, +to_meter
	// and their precedence, rather than interpreting PROJ.4 independently.
	sys, _, err := core.NewSystem(ps)
	if err != nil {
		return 0, fmt.Errorf("SRID %d units: %w", srid, err)
	}
	if sys.Right != core.IOUnitsClassic && sys.Right != core.IOUnitsProjected {
		return 0, fmt.Errorf("SRID %d has unsupported non-linear units", srid)
	}
	if sys.ToMeter <= 0 || math.IsNaN(sys.ToMeter) || math.IsInf(sys.ToMeter, 0) {
		return 0, fmt.Errorf("SRID %d has invalid meters per unit: %v", srid, sys.ToMeter)
	}
	return sys.ToMeter, nil
}
