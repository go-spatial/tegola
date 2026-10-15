// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package operations

/*** WE NEED TO PORT THE geodesic.h FUNCTIONS BEFORE WE CAN UNCOMMENT THIS ***/

/***

import (
	"math"

	"github.com/go-spatial/proj/merror"

	"github.com/go-spatial/proj/core"
	"github.com/go-spatial/proj/support"
)

func init() {
	core.RegisterConvertLPToXY("aeqd",
		"Azimuthal Equidistant",
		"\n\tAzi, Sph&Ell\n\tlat_0 guam",
		NewAeqd,
	)
}

type fwdfunc func(aea *Aeqd, lp *core.CoordLP) (*core.CoordXY, error)
type invfunc func(aea *Aeqd, lp *core.CoordXY) (*core.CoordLP, error)

// Aeqd implements core.IOperation and core.ConvertLPToXY
type Aeqd struct {
	core.Operation

	sinph0 float64
	cosph0 float64
	en     []float64
	M1     float64
	N1     float64
	Mp     float64
	He     float64
	G      float64
	mode   mode
	g      geodesic

	fwd fwdfunc
	inv invfunc
}

// NewAeqd is
func NewAeqd(system *core.System, desc *core.OperationDescription) (core.IConvertLPToXY, error) {
	xxx := &Aeqd{}
	xxx.System = system

	err := xxx.setup(system)
	if err != nil {
		return nil, err
	}
	return xxx, nil
}

// Forward goes frontwords
func (aeqd *Aeqd) Forward(lp *core.CoordLP) (*core.CoordXY, error) {
	return aeqd.fwd(aeqd, lp)
}

// Inverse goes backwards
func (aeqd *Aeqd) Inverse(xy *core.CoordXY) (*core.CoordLP, error) {
	return aeqd.inv(aeqd, xy)
}

//---------------------------------------------------------------------

// Guam elliptical
func aeqdEGuamFwd(aeqd *Aeqd, lp *core.CoordLP) (*core.CoordXY, error) {
	xy := &core.CoordXY{0.0, 0.0}

	Q := aeqd
	P := aeqd.System
	PE := aeqd.System.Ellipsoid

	var cosphi, sinphi, t float64

	cosphi = math.Cos(lp.Phi)
	sinphi = math.Sin(lp.Phi)
	t = 1. / math.Sqrt(1.-PE.Es*sinphi*sinphi)
	xy.X = lp.Lam * cosphi * t
	xy.Y = support.Mlfn(lp.Phi, sinphi, cosphi, Q.en) - Q.M1 +
		.5*lp.Lam*lp.Lam*cosphi*sinphi*t

	return xy, nil
}

// Guam elliptical
func aeqdEGuamInv(aeqd *Aeqd, xy *core.CoordXY) (*core.CoordLP, error) {
	lp := &core.CoordLP{0.0, 0.0}

	Q := aeqd
	P := aeqd.System
	PE := aeqd.System.Ellipsoid

	var x2 float64
	t := 0.0
	var i int
	var err error

	x2 = 0.5 * xy.X * xy.X
	lp.Phi = P.Phi0
	for i = 0; i < 3; i++ {
		t = PE.E * math.Sin(lp.Phi)
		t = math.Sqrt(1. - t*t)
		lp.Phi, err = support.InvMlfn(Q.M1+xy.Y-x2*math.Tan(lp.Phi)*t, PE.Es, Q.en)
		if err != nil {
			return nil, err
		}
	}
	lp.Lam = xy.X * t / math.Cos(lp.Phi)
	return lp, nil
}

// Ellipsoidal, forward
func aeqdEForward(aeqd *Aeqd, lp *core.CoordLP) (*core.CoordXY, error) {
	xy := &core.CoordXY{X: 0.0, Y: 0.0}

	Q := aeqd
	P := aeqd.System
	PE := aeqd.System.Ellipsoid

	var coslam, cosphi, sinphi, rho float64
	var azi1, azi2, s12 float64
	var lam1, phi1, lam2, phi2 float64

	coslam = math.Cos(lp.Lam)
	cosphi = math.Cos(lp.Phi)
	sinphi = math.Sin(lp.Phi)
	switch Q.mode {
	case modeNPole:
		coslam = -coslam
		fallthrough
	case modeSPole:
		rho = math.Abs(Q.Mp - support.Mlfn(lp.Phi, sinphi, cosphi, Q.en))
		xy.X = rho * math.Sin(lp.Lam)
		xy.Y = rho * coslam
		break
	case modeEquit, modeObliq:
		if math.Abs(lp.Lam) < eps10 && math.Abs(lp.Phi-P.Phi0) < eps10 {
			xy.X = 0.
			xy.Y = 0.
			break
		}

		phi1 = P.Phi0 / support.DegToRad
		lam1 = P.Lam0 / support.DegToRad
		phi2 = lp.Phi / support.DegToRad
		lam2 = (lp.Lam + P.Lam0) / support.DegToRad

		geod_inverse(&Q.g, phi1, lam1, phi2, lam2, &s12, &azi1, &azi2)
		azi1 *= support.DegToRad
		xy.X = s12 * math.Sin(azi1) / PE.A
		xy.Y = s12 * math.Cos(azi1) / PE.A
		break
	}
	return xy, nil
}

// Spheroidal, forward
func aeqdSForward(aeqd *Aeqd, lp *core.CoordLP) (*core.CoordXY, error) {

	xy := &core.CoordXY{0.0, 0.0}

	Q := aeqd
	P := aeqd.System
	PE := aeqd.System.Ellipsoid

	var coslam, cosphi, sinphi float64

	sinphi = math.Sin(lp.Phi)
	cosphi = math.Cos(lp.Phi)
	coslam = math.Cos(lp.Lam)

	switch Q.mode {
	case modeEquit, modeObliq:
		if Q.mode == modeEquit {
			xy.Y = cosphi * coslam
		} else {
			xy.Y = Q.sinph0*sinphi + Q.cosph0*cosphi*coslam
		}

		if math.Abs(math.Abs(xy.Y)-1.) < tol14 {
			if xy.Y < 0. {
				return nil, merror.New(merror.ToleranceCondition)
			}
			xy.X = 0.
			xy.Y = 0.
		} else {
			xy.Y = math.Acos(xy.Y)
			xy.Y /= math.Sin(xy.Y)
			xy.X = xy.Y * cosphi * math.Sin(lp.Lam)
			if Q.mode == modeEquit {
				xy.Y *= sinphi
			} else {
				xy.Y *= Q.cosph0*sinphi - Q.sinph0*cosphi*coslam
			}
		}
		break
	case modeNPole:
		lp.Phi = -lp.Phi
		coslam = -coslam
		fallthrough
	case modeSPole:
		if math.Abs(lp.Phi-support.PiOverTwo) < eps10 {
			return nil, merror.New(merror.ToleranceCondition)
		}
		xy.Y = support.PiOverTwo + lp.Phi
		xy.X = xy.Y * math.Sin(lp.Lam)
		xy.Y *= coslam
		break
	}
	return xy, nil
}

// Ellipsoidal, inverse
func aeqdEInverse(aeqd *Aeqd, xy *core.CoordXY) (*core.CoordLP, error) {

	lp := &core.CoordLP{0.0, 0.0}

	Q := aeqd
	P := aeqd.System
	PE := aeqd.System.Ellipsoid
	var err error
	var c, azi1, azi2, s12, x2, y2, lat1, lon1, lat2, lon2 float64

	c = math.Hypot(xy.X, xy.Y)
	if c < eps10 {
		lp.Phi = P.Phi0
		lp.Lam = 0.
		return lp, nil
	}
	if Q.mode == modeObliq || Q.mode == modeEquit {

		x2 = xy.X * PE.A
		y2 = xy.Y * PE.A
		lat1 = P.Phi0 / support.DegToRad
		lon1 = P.Lam0 / support.DegToRad
		azi1 = math.Atan2(x2, y2) / support.DegToRad
		s12 = math.Sqrt(x2*x2 + y2*y2)
		geod_direct(&Q.g, lat1, lon1, azi1, s12, &lat2, &lon2, &azi2)
		lp.Phi = lat2 * support.DegToRad
		lp.Lam = lon2 * support.DegToRad
		lp.Lam -= P.Lam0
	} else { // Polar
		t := 0.0
		if Q.mode == modeNPole {
			t = Q.Mp - c
		} else {
			t = Q.Mp + c
		}
		lp.Phi, err = support.InvMlfn(t, PE.Es, Q.en)
		if err != nil {
			return nil, err
		}
		if Q.mode == modeNPole {
			t = -xy.Y
		} else {
			t = xy.Y
		}
		lp.Lam = math.Atan2(xy.X, t)
	}
	return lp, nil
}

// Spheroidal, inverse
func aeqdSInverse(aeqd *Aeqd, xy *core.CoordXY) (*core.CoordLP, error) {

	lp := &core.CoordLP{0.0, 0.0}

	Q := aeqd
	P := aeqd.System
	PE := aeqd.System.Ellipsoid

	var cosc, cRh, sinc float64

	cRh = math.Hypot(xy.X, xy.Y)
	if cRh > math.Pi {
		if cRh-eps10 > math.Pi {
			return nil, merror.New(merror.ToleranceCondition)
		}
		cRh = math.Pi
	} else if cRh < eps10 {
		lp.Phi = P.Phi0
		lp.Lam = 0.
		return lp, nil
	}
	if Q.mode == modeObliq || Q.mode == modeEquit {
		sinc = math.Sin(cRh)
		cosc = math.Cos(cRh)
		if Q.mode == modeEquit {
			lp.Phi = support.Aasin(xy.Y * sinc / cRh)
			xy.X *= sinc
			xy.Y = cosc * cRh
		} else {
			lp.Phi = support.Aasin(cosc*Q.sinph0 + xy.Y*sinc*Q.cosph0/cRh)
			xy.Y = (cosc - Q.sinph0*math.Sin(lp.Phi)) * cRh
			xy.X *= sinc * Q.cosph0
		}
		if xy.Y == 0. {
			lp.Lam = 0.
		} else {
			lp.Lam = math.Atan2(xy.X, xy.Y)
		}
	} else if Q.mode == modeNPole {
		lp.Phi = support.PiOverTwo - cRh
		lp.Lam = math.Atan2(xy.X, -xy.Y)
	} else {
		lp.Phi = cRh - support.PiOverTwo
		lp.Lam = math.Atan2(xy.X, xy.Y)
	}
	return lp, nil
}

func (aeqd *Aeqd) setup(sys *core.System) error {

	Q := aeqd
	P := aeqd.System
	PE := aeqd.System.Ellipsoid

	geod_init(&Q.g, PE.A, PE.Es/(1+math.Sqrt(PE.OneEs)))

	if math.Abs(math.Abs(P.Phi0)-support.PiOverTwo) < eps10 {
		if P.Phi0 < 0. {
			Q.mode = modeSPole
			Q.sinph0 = -1.
		} else {
			Q.mode = modeNPole
			Q.sinph0 = 1.
		}
		Q.cosph0 = 0.
	} else if math.Abs(P.Phi0) < eps10 {
		Q.mode = modeEquit
		Q.sinph0 = 0.
		Q.cosph0 = 1.
	} else {
		Q.mode = modeObliq
		Q.sinph0 = math.Sin(P.Phi0)
		Q.cosph0 = math.Cos(P.Phi0)
	}
	if PE.Es == 0.0 {
		Q.inv = aeqdSInverse
		Q.fwd = aeqdSForward
	} else {
		Q.en = support.Enfn(PE.Es)
		if Q.en == nil {
			return nil
		}
		if P.ProjString.ContainsKey("guam") {
			Q.M1 = support.Mlfn(P.Phi0, Q.sinph0, Q.cosph0, Q.en)
			Q.inv = aeqdEGuamInv
			Q.fwd = aeqdEGuamFwd
		} else {
			switch Q.mode {
			case modeNPole:
				Q.Mp = support.Mlfn(support.PiOverTwo, 1., 0., Q.en)
				break
			case modeSPole:
				Q.Mp = support.Mlfn(-support.PiOverTwo, -1., 0., Q.en)
				break
			case modeEquit, modeObliq:
				Q.inv = aeqdEInverse
				Q.fwd = aeqdEForward
				Q.N1 = 1. / math.Sqrt(1.-PE.Es*Q.sinph0*Q.sinph0)
				Q.He = PE.E / math.Sqrt(PE.OneEs)
				Q.G = Q.sinph0 * Q.He
				Q.He *= Q.cosph0
				break
			}
			Q.inv = aeqdEInverse
			Q.fwd = aeqdEForward
		}
	}

	return nil
}

***/
