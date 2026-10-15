// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package support

import "math"

// Msfn is to "determine constant small m"
func Msfn(sinphi, cosphi, es float64) float64 {
	return (cosphi / math.Sqrt(1.-es*sinphi*sinphi))
}
