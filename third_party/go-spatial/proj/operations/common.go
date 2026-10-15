// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package operations

type mode int

const (
	modeNPole mode = 0
	modeSPole      = 1
	modeEquit      = 2
	modeObliq      = 3
)

const tol7 = 1.e-7
const tol10 = 1.0e-10

const eps7 = 1.0e-7
const eps10 = 1.e-10
