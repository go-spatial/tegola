// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package operations

import (
	"math"

	"github.com/go-spatial/proj/core"
	"github.com/go-spatial/proj/merror"
	"github.com/go-spatial/proj/support"
)

func init() {
	core.RegisterConvertLPToXY("aea",
		"Albers Equal Area",
		"\n\tConic Sph&Ell\n\tlat_1= lat_2=",
		NewAea,
	)
	core.RegisterConvertLPToXY("leac",
		"Lambert Equal Area Conic",
		"\n\tConic, Sph&Ell\n\tlat_1= south",
		NewLeac)
}

// Aea implements core.IOperation and core.ConvertLPToXY
type Aea struct {
	core.Operation
	isLambert bool

	// the "opaque" parts

	ec     float64
	n      float64
	c      float64
	dd     float64
	n2     float64
	rho0   float64
	rho    float64
	phi1   float64
	phi2   float64
	en     []float64
	ellips bool
}

// NewAea is from PJ_aea.c
func NewAea(system *core.System, desc *core.OperationDescription) (core.IConvertLPToXY, error) {
	op := &Aea{
		isLambert: false,
	}
	op.System = system

	err := op.aeaSetup(system)
	if err != nil {
		return nil, err
	}
	return op, nil
}

// NewLeac is too
func NewLeac(system *core.System, desc *core.OperationDescription) (core.IConvertLPToXY, error) {
	op := &Aea{
		isLambert: true,
	}
	op.System = system

	err := op.leacSetup(system)
	if err != nil {
		return nil, err
	}
	return op, nil
}

//---------------------------------------------------------------------

/* determine latitude angle phi-1 */
const nIter = 15

func phi1(qs, Te, tOneEs float64) float64 {
	var i int
	var Phi, sinpi, cospi, con, com, dphi float64

	Phi = math.Asin(.5 * qs)
	if Te < eps7 {
		return (Phi)
	}
	i = nIter
	for {
		sinpi = math.Sin(Phi)
		cospi = math.Cos(Phi)
		con = Te * sinpi
		com = 1. - con*con
		dphi = .5 * com * com / cospi * (qs/tOneEs -
			sinpi/com + .5/Te*math.Log((1.-con)/
			(1.+con)))
		Phi += dphi
		i--
		if !(math.Abs(dphi) > tol10 && i != 0) {
			break
		}
	}
	if i != 0 {
		return Phi
	}
	return math.MaxFloat64
}

func (op *Aea) setup(sys *core.System) error {
	var cosphi, sinphi float64
	var secant bool

	Q := op
	P := op.System
	PE := P.Ellipsoid

	if math.Abs(Q.phi1+Q.phi2) < eps10 {
		return merror.New(merror.ConicLatEqual)
	}
	sinphi = math.Sin(Q.phi1)
	Q.n = sinphi
	cosphi = math.Cos(Q.phi1)
	secant = math.Abs(Q.phi1-Q.phi2) >= eps10
	Q.ellips = (P.Ellipsoid.Es > 0.0)
	if Q.ellips {
		var ml1, m1 float64

		Q.en = support.Enfn(PE.Es)
		m1 = support.Msfn(sinphi, cosphi, PE.Es)
		ml1 = support.Qsfn(sinphi, PE.E, PE.OneEs)
		if secant { // secant cone
			var ml2, m2 float64

			sinphi = math.Sin(Q.phi2)
			cosphi = math.Cos(Q.phi2)
			m2 = support.Msfn(sinphi, cosphi, PE.Es)
			ml2 = support.Qsfn(sinphi, PE.E, PE.OneEs)
			if ml2 == ml1 {
				return merror.New(merror.AeaSetupFailed)
			}

			Q.n = (m1*m1 - m2*m2) / (ml2 - ml1)
		}
		Q.ec = 1. - .5*PE.OneEs*math.Log((1.-PE.E)/
			(1.+PE.E))/PE.E
		Q.c = m1*m1 + Q.n*ml1
		Q.dd = 1. / Q.n
		Q.rho0 = Q.dd * math.Sqrt(Q.c-Q.n*support.Qsfn(math.Sin(P.Phi0),
			PE.E, PE.OneEs))
	} else {
		if secant {
			Q.n = .5 * (Q.n + math.Sin(Q.phi2))
		}
		Q.n2 = Q.n + Q.n
		Q.c = cosphi*cosphi + Q.n2*sinphi
		Q.dd = 1. / Q.n
		Q.rho0 = Q.dd * math.Sqrt(Q.c-Q.n2*math.Sin(P.Phi0))
	}

	return nil
}

// Forward goes frontwords
func (op *Aea) Forward(lp *core.CoordLP) (*core.CoordXY, error) {
	xy := &core.CoordXY{X: 0.0, Y: 0.0}
	Q := op
	PE := op.System.Ellipsoid

	var t float64
	if Q.ellips {
		t = Q.n * support.Qsfn(math.Sin(lp.Phi), PE.E, PE.OneEs)
	} else {
		t = Q.n2 * math.Sin(lp.Phi)
	}
	Q.rho = Q.c - t
	if Q.rho < 0. {
		return xy, merror.New(merror.ToleranceCondition)
	}
	Q.rho = Q.dd * math.Sqrt(Q.rho)
	lp.Lam *= Q.n
	xy.X = Q.rho * math.Sin(lp.Lam)
	xy.Y = Q.rho0 - Q.rho*math.Cos(lp.Lam)
	return xy, nil
}

// Inverse goes backwards
func (op *Aea) Inverse(xy *core.CoordXY) (*core.CoordLP, error) {

	lp := &core.CoordLP{Lam: 0.0, Phi: 0.0}
	Q := op
	PE := op.System.Ellipsoid

	xy.Y = Q.rho0 - xy.Y
	Q.rho = math.Hypot(xy.X, xy.Y)
	if Q.rho != 0.0 {
		if Q.n < 0. {
			Q.rho = -Q.rho
			xy.X = -xy.X
			xy.Y = -xy.Y
		}
		lp.Phi = Q.rho / Q.dd
		if Q.ellips {
			lp.Phi = (Q.c - lp.Phi*lp.Phi) / Q.n
			if math.Abs(Q.ec-math.Abs(lp.Phi)) > tol7 {
				lp.Phi = phi1(lp.Phi, PE.E, PE.OneEs)
				if lp.Phi == math.MaxFloat64 {
					return lp, merror.New(merror.ToleranceCondition)
				}
			} else {
				if lp.Phi < 0. {
					lp.Phi = -support.PiOverTwo
				} else {
					lp.Phi = support.PiOverTwo
				}
			}
		} else {
			lp.Phi = (Q.c - lp.Phi*lp.Phi) / Q.n2
			if math.Abs(lp.Phi) <= 1. {
				lp.Phi = math.Asin(lp.Phi)
			} else {
				if lp.Phi < 0. {
					lp.Phi = -support.PiOverTwo
				} else {
					lp.Phi = support.PiOverTwo
				}
			}
		}
		lp.Lam = math.Atan2(xy.X, xy.Y) / Q.n
	} else {
		lp.Lam = 0.
		if Q.n > 0. {
			lp.Phi = support.PiOverTwo
		} else {
			lp.Phi = -support.PiOverTwo
		}
	}
	return lp, nil
}

func (op *Aea) aeaSetup(sys *core.System) error {

	lat1, ok := op.System.ProjString.GetAsFloat("lat_1")
	if !ok {
		lat1 = 0.0
	}
	lat2, ok := op.System.ProjString.GetAsFloat("lat_2")
	if !ok {
		lat2 = 0.0
	}

	op.phi1 = support.DDToR(lat1)
	op.phi2 = support.DDToR(lat2)

	return op.setup(op.System)
}

func (op *Aea) leacSetup(sys *core.System) error {

	lat1, ok := op.System.ProjString.GetAsFloat("lat_1")
	if !ok {
		lat1 = 0.0
	}

	south := -support.PiOverTwo
	_, ok = op.System.ProjString.GetAsInt("south")
	if !ok {
		south = support.PiOverTwo
	}

	op.phi2 = support.DDToR(lat1)
	op.phi1 = south

	return op.setup(op.System)
}
