// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package merror

// All the errors
var (
	UnknownProjection               = "unknown projection: %s"
	UnknownEllipseParameter         = "unknown ellipse parameter: %s"
	UnsupportedProjectionString     = "unsupported projection string: %s"
	InvalidProjectionSyntax         = "invalid projection syntax: %s"
	ProjectionStringRequiresEllipse = "projection string requires ellipse"
	MajorAxisNotGiven               = "major axis not given"
	ReverseFlatteningIsZero         = "reverse flattening (rf) is zero"
	EccentricityIsOne               = "eccentricity is one"
	ToleranceCondition              = "tolerance condition error"
	ProjValueMissing                = "proj value missing"
	NoSuchDatum                     = "no such datum"
	NotYetSupported                 = "not yet supported" // TODO
	InvalidArg                      = "invalid argument in proj string"
	EsLessThanZero                  = "ES is less than zero"
	RefRadLargerThan90              = "ref rad is greater than 90"
	InvalidDMS                      = "invalid DMS (degrees-minutes-seconds)"
	EllipsoidUseRequired            = "use of ellipsoid required"
	InvalidUTMZone                  = "invalid UTM zone"
	LatOrLonExceededLimit           = "lat or lon limit exceeded"
	UnknownUnit                     = "unknown unit"
	UnitFactorLessThanZero          = "unit factor less than zero"
	Axis                            = "invalid axis configuration"
	KLessThanZero                   = "K is less than zero"
	CoordinateError                 = "invalid coordinates"
	InvalidXOrY                     = "invalid X or Y"
	ConicLatEqual                   = "Conic lat is equal"
	AeaSetupFailed                  = "setup for aea projection failed"
	InvMlfn                         = "invalid mlfn computation"
	AeaProjString                   = "invalid projection string for aea"
	LatTSLargerThan90               = "lat ts is greater than 90"
	Phi2                            = "invalid phi2 computation"
)
