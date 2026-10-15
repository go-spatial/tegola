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

// Adjlon reduces argument to range +/- PI
func Adjlon(lon float64) float64 {
	/* Let lon slightly overshoot, to avoid spurious sign switching at the date line */
	if math.Abs(lon) < Pi+1e-12 {
		return lon
	}

	/* adjust to 0..2pi range */
	lon += Pi

	/* remove integral # of 'revolutions'*/
	lon -= TwoPi * math.Floor(lon/TwoPi)

	/* adjust back to -pi..pi range */
	lon -= Pi

	return lon
}
