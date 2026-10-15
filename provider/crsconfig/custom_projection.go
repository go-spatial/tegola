package crsconfig

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/alexeydott/proj/core"
	"github.com/alexeydott/proj/support"
)

// Custom admission is deliberately projection-specific: accepting a parameter
// that the selected operation ignores would silently change source geometry.
func newCustomFeatureProjection(definition string) (*HeightProjection, error) {
	values := make(map[string]string)
	for _, word := range strings.Fields(definition) {
		key, value, _ := strings.Cut(word[1:], "=")
		values[key] = value
	}
	if values["proj"] != "etmerc" {
		return nil, fmt.Errorf("custom feature projection requires etmerc")
	}
	for key, value := range values {
		switch key {
		case "proj", "ellps", "units", "pm", "no_defs":
		case "datum":
			if value != "WGS84" || values["ellps"] != "" || values["towgs84"] != "" {
				return nil, fmt.Errorf("custom projection datum must be WGS84 or explicit ellps and towgs84")
			}
		case "lat_0", "lon_0", "k", "k_0", "x_0", "y_0", "to_meter":
			number, err := strconv.ParseFloat(value, 64)
			if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
				return nil, fmt.Errorf("custom projection parameter %s must be finite", key)
			}
			if key == "lat_0" && math.Abs(number) > 90 {
				return nil, fmt.Errorf("custom projection latitude outside valid domain")
			}
		case "towgs84":
			parts := strings.Split(value, ",")
			if len(parts) != 3 && len(parts) != 7 {
				return nil, fmt.Errorf("custom projection requires 3 or 7 datum parameters")
			}
			for _, part := range parts {
				number, err := strconv.ParseFloat(part, 64)
				if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
					return nil, fmt.Errorf("custom projection datum parameters must be finite")
				}
			}
		default:
			return nil, fmt.Errorf("custom projection parameter %s unsupported", key)
		}
	}
	if values["datum"] == "" && (values["ellps"] == "" || values["towgs84"] == "") {
		return nil, fmt.Errorf("custom projection requires explicit ellipsoid and datum transformation")
	}
	if values["k"] != "" && values["k_0"] != "" || values["units"] != "" && values["to_meter"] != "" {
		return nil, fmt.Errorf("custom projection contains conflicting scale or unit parameters")
	}
	parsed, err := support.NewProjString(definition)
	if err != nil {
		return nil, fmt.Errorf("custom projection: %w", err)
	}
	system, operation, err := core.NewSystem(parsed)
	if err != nil {
		return nil, fmt.Errorf("custom projection: %w", err)
	}
	converter, ok := operation.(core.IConvertLPToXY)
	if !ok {
		return nil, fmt.Errorf("custom projection has no horizontal converter")
	}
	if system.DatumType != core.DatumTypeWGS84 && system.DatumType != core.DatumType3Param && system.DatumType != core.DatumType7Param {
		return nil, fmt.Errorf("custom projection datum transformation unsupported")
	}
	if system.Ellipsoid == nil || system.Ellipsoid.A <= 0 || system.Ellipsoid.Es < 0 || system.Ellipsoid.Es >= 1 || !finite(system.FromGreenwich) || !finite(system.ToMeter) {
		return nil, fmt.Errorf("custom projection has invalid ellipsoid or units")
	}
	if system.DatumType == core.DatumType7Param && system.DatumParams[6] <= 0 {
		return nil, fmt.Errorf("custom projection datum scale must be positive")
	}
	return &HeightProjection{definition: definition, converter: converter, datum: system}, nil
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

// transformDatum applies a position-vector Helmert transformation through ECEF.
// Input/output are horizontal coordinates at zero ellipsoidal height, matching
// the 2D PROJ contract. This does not establish vertical-datum equivalence.
func transformDatum(system *core.System, lp *core.CoordLP, fromWGS84 bool) {
	if system == nil || system.DatumType == core.DatumTypeWGS84 {
		return
	}
	const wgsA = 6378137.0
	const wgsE2 = 0.0066943799901413165
	sourceA, sourceE2 := system.Ellipsoid.A, system.Ellipsoid.Es
	a, e2 := sourceA, sourceE2
	if fromWGS84 {
		a, e2 = wgsA, wgsE2
	}
	sinLat, cosLat := math.Sincos(lp.Phi)
	sinLon, cosLon := math.Sincos(lp.Lam)
	radius := a / math.Sqrt(1-e2*sinLat*sinLat)
	x, y, z := radius*cosLat*cosLon, radius*cosLat*sinLon, radius*(1-e2)*sinLat
	params := system.DatumParams
	scale := 1.0
	if system.DatumType == core.DatumType7Param {
		scale = params[6]
	}
	rx, ry, rz := params[3], params[4], params[5]
	if fromWGS84 {
		x, y, z = (x-params[0])/scale, (y-params[1])/scale, (z-params[2])/scale
		x, y, z = x+rz*y-ry*z, -rz*x+y+rx*z, ry*x-rx*y+z
		a, e2 = sourceA, sourceE2
	} else {
		x, y, z = params[0]+scale*(x-rz*y+ry*z), params[1]+scale*(rz*x+y-rx*z), params[2]+scale*(-ry*x+rx*y+z)
		a, e2 = wgsA, wgsE2
	}
	lp.Lam = math.Atan2(y, x)
	horizontal := math.Hypot(x, y)
	// Iterating latitude via N avoids division by cos(latitude) at the poles.
	latitude := math.Atan2(z, horizontal*(1-e2))
	for range 15 {
		sine := math.Sin(latitude)
		n := a / math.Sqrt(1-e2*sine*sine)
		next := math.Atan2(z+e2*n*sine, horizontal)
		if math.Abs(next-latitude) < 1e-14 {
			latitude = next
			break
		}
		latitude = next
	}
	lp.Phi = latitude
}
