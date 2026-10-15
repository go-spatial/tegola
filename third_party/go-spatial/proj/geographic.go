package proj

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/go-spatial/proj/core"
	"github.com/go-spatial/proj/support"
)

// IsGeographicDefinition reports whether defn is a supported geographic CRS:
// longitude/latitude in degrees with a supported datum transformation,
// Greenwich meridian and east/north axes. Grid shifts remain unsupported.
func IsGeographicDefinition(defn string) bool {
	ps, err := support.NewProjString(defn)
	if err != nil {
		return false
	}
	geographic, err := geographicDefinition(ps)
	if !geographic || err != nil {
		return false
	}
	_, err = geographicSystem(ps)
	return err == nil
}

func geographicDefinition(ps *support.ProjString) (bool, error) {
	geographic := false
	for _, p := range ps.Pairs {
		if p.Key == "proj" {
			switch p.Value {
			case "longlat", "latlong", "lonlat", "latlon":
				geographic = true
			}
		}
	}
	if !geographic {
		return false, nil
	}
	seen := map[string]bool{}
	for _, p := range ps.Pairs {
		valid := false
		if seen[p.Key] {
			return true, fmt.Errorf("geographic CRS has duplicate parameter %q", p.Key)
		}
		seen[p.Key] = true
		switch p.Key {
		case "proj":
			valid = p.Value == "longlat" || p.Value == "latlong" || p.Value == "lonlat" || p.Value == "latlon"
		case "datum", "ellps":
			valid = p.Value != ""
		case "a", "b", "rf", "f", "es", "e":
			valid = finiteGeographicNumber(p.Value)
		case "no_defs", "wktext":
			valid = p.Value == ""
		case "type":
			valid = p.Value == "crs"
		case "axis":
			valid = p.Value == "enu"
		case "units":
			valid = p.Value == "degrees"
		case "pm":
			valid = p.Value == "greenwich" || zeroGeographicNumber(p.Value)
		case "towgs84":
			values := strings.Split(p.Value, ",")
			valid = len(values) == 3 || len(values) == 7
			for _, value := range values {
				valid = valid && finiteGeographicNumber(value)
			}
		}
		if !valid {
			return true, fmt.Errorf("unsupported geographic CRS parameter %q", p.Key)
		}
	}
	if name, ok := ps.GetAsString("datum"); ok {
		if datum, known := support.DatumsTable[name]; known {
			if ellps, explicit := ps.GetAsString("ellps"); explicit && ellps != datum.EllipseID {
				return true, fmt.Errorf("geographic datum and ellipsoid conflict")
			}
			for _, key := range []string{"a", "b", "rf", "f", "es", "e"} {
				if ps.ContainsKey(key) {
					return true, fmt.Errorf("geographic datum cannot be combined with ellipsoid override %q", key)
				}
			}
		}
	}
	return true, nil
}

func zeroGeographicNumber(value string) bool {
	v, err := strconv.ParseFloat(value, 64)
	return err == nil && v == 0
}

func copyGeographicCoordinates(input []float64) ([]float64, error) {
	if len(input)%2 != 0 {
		return nil, fmt.Errorf("input array of lon/lat values must be an even number")
	}
	for i, v := range input {
		if math.IsNaN(v) || math.IsInf(v, 0) || (i%2 == 1 && math.Abs(v) > 90) {
			return nil, fmt.Errorf("invalid geographic coordinate at index %d", i)
		}
	}
	output := make([]float64, len(input))
	copy(output, input)
	return output, nil
}

// geographicSystem uses the same datum parser and ellipsoid as projected CRSs.
func geographicSystem(ps *support.ProjString) (*core.System, error) {
	sys, err := core.NewGeographicSystem(ps)
	if err != nil {
		return nil, err
	}
	e := sys.Ellipsoid
	if math.IsNaN(e.A) || math.IsInf(e.A, 0) || e.A <= 0 ||
		math.IsNaN(e.Es) || math.IsInf(e.Es, 0) || e.Es < 0 || e.Es >= 1 {
		return nil, fmt.Errorf("invalid geographic ellipsoid")
	}
	switch sys.DatumType {
	case core.DatumType3Param, core.DatumType7Param, core.DatumTypeWGS84:
	case core.DatumTypeUnknown:
		if e.A != wgs84Ellipsoid.a || math.Abs(e.Es-wgs84Ellipsoid.e2) > 1e-14 {
			return nil, fmt.Errorf("geographic CRS requires a datum transformation to WGS84")
		}
	default:
		return nil, fmt.Errorf("unsupported geographic datum transformation")
	}
	for _, v := range sys.DatumParams {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("invalid geographic datum parameter")
		}
	}
	if sys.DatumType == core.DatumType7Param && sys.DatumParams[6] == 0 {
		return nil, fmt.Errorf("geographic datum scale must be nonzero")
	}
	return sys, nil
}

func finiteGeographicNumber(value string) bool {
	v, err := strconv.ParseFloat(value, 64)
	return err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func (conv *conversion) geographicCoordinates(input []float64, inverse bool) ([]float64, error) {
	output, err := copyGeographicCoordinates(input)
	if err != nil {
		return nil, err
	}
	// Preserve exact identity coordinates, including unwrapped longitudes.
	identity := conv.system.DatumType != core.DatumType3Param && conv.system.DatumType != core.DatumType7Param
	if !identity {
		p := conv.system.DatumParams
		identity = conv.system.Ellipsoid.A == wgs84Ellipsoid.a &&
			math.Abs(conv.system.Ellipsoid.Es-wgs84Ellipsoid.e2) < 1e-14 &&
			p[0] == 0 && p[1] == 0 && p[2] == 0 && p[3] == 0 && p[4] == 0 && p[5] == 0 &&
			(conv.system.DatumType == core.DatumType3Param || p[6] == 1)
	}
	if identity {
		return output, nil
	}
	for i := 0; i < len(output); i += 2 {
		lp := &core.CoordLP{Lam: support.DDToR(input[i]), Phi: support.DDToR(input[i+1])}
		if inverse {
			datumToWGS84(conv.system, lp)
		} else {
			datumFromWGS84(conv.system, lp)
		}
		output[i], output[i+1] = support.RToDD(lp.Lam), support.RToDD(lp.Phi)
		if math.IsNaN(output[i]) || math.IsInf(output[i], 0) ||
			math.IsNaN(output[i+1]) || math.IsInf(output[i+1], 0) {
			return nil, fmt.Errorf("non-finite geographic datum result at index %d", i)
		}
	}
	return output, nil
}
