package simplify

import (
	"math"

	"github.com/go-spatial/tegola"
)

var (
	DefaultTolerence = 10.0
)

// This is from Leaflet
func ZEpislon(z uint, tileExtent float64) float64 {

	if z == tegola.MaxZ {
		return 0
	}
	epi := DefaultTolerence
	if epi <= 0 {
		return 0
	}

	denom := (math.Exp2(float64(z)) * tileExtent)

	return epi / denom
}
