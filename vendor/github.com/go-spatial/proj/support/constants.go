// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package support

import "math"

// some more useful math constants and aliases */
const (
	Pi          = math.Pi
	PiOverTwo   = Pi / 2
	PiOverFour  = Pi / 4
	TwoOverPi   = 2 / Pi
	PiHalfPi    = 1.5 * Pi
	TwoPi       = 2 * Pi
	TwoPiHalfPi = 2.5 * Pi
)

// DegToRad is the multiplication factor to convert degrees to radians
const DegToRad = 0.017453292519943296
