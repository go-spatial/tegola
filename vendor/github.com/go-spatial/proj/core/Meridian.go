// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package core

// PrimeMeridian contains information about a prime meridian
//
// Someday, this will be a rich type with lots of methods and stuff.
//
// Today, it is not.
type PrimeMeridian struct {
	ID         string
	Definition string
}
