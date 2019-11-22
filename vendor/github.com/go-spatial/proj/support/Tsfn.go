// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package support

import (
	"math"
)

// Tsfn is to "determine small t"
func Tsfn(phi, sinphi, e float64) float64 {
	sinphi *= e

	/* avoid zero division, fail gracefully */
	denominator := 1.0 + sinphi
	if denominator == 0.0 {
		return math.MaxFloat64
	}

	return (math.Tan(.5*(PiOverTwo-phi)) /
		math.Pow((1.-sinphi)/(denominator), .5*e))
}
