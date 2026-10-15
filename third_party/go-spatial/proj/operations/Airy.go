// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package operations

import (
	"math"

	"github.com/go-spatial/proj/merror"

	"github.com/go-spatial/proj/core"
	"github.com/go-spatial/proj/support"
)

func init() {
	core.RegisterConvertLPToXY("airy",
		"Airy",
		"\n\tMisc Sph, no inv.\n\tno_cut lat_b=",
		NewAiry,
	)
}

// Airy implements core.IOperation and core.ConvertLPToXY
type Airy struct {
	core.Operation
	phalfpi float64
	sinph0  float64
	cosph0  float64
	Cb      float64
	mode    mode
	nocut   bool /* do not cut at hemisphere limit */
}

// NewAiry returns a new Airy
func NewAiry(system *core.System, desc *core.OperationDescription) (core.IConvertLPToXY, error) {
	op := &Airy{}
	op.System = system

	err := op.setup(system)
	if err != nil {
		return nil, err
	}
	return op, nil
}

// Forward goes forewards
func (op *Airy) Forward(lp *core.CoordLP) (*core.CoordXY, error) {
	xy := &core.CoordXY{X: 0.0, Y: 0.0}

	Q := op

	var sinlam, coslam, cosphi, sinphi, t, Krho, cosz float64

	sinlam = math.Sin(lp.Lam)
	coslam = math.Cos(lp.Lam)

	switch Q.mode {

	case modeEquit, modeObliq:
		sinphi = math.Sin(lp.Phi)
		cosphi = math.Cos(lp.Phi)
		cosz = cosphi * coslam
		if Q.mode == modeObliq {
			cosz = Q.sinph0*sinphi + Q.cosph0*cosz
		}
		if !Q.nocut && cosz < -eps10 {
			return nil, merror.New(merror.ToleranceCondition)
		}
		s := 1. - cosz
		if math.Abs(s) > eps10 {
			t = 0.5 * (1. + cosz)
			Krho = -math.Log(t)/s - Q.Cb/t
		} else {
			Krho = 0.5 - Q.Cb
		}
		xy.X = Krho * cosphi * sinlam
		if Q.mode == modeObliq {
			xy.Y = Krho * (Q.cosph0*sinphi - Q.sinph0*cosphi*coslam)
		} else {
			xy.Y = Krho * sinphi
		}

	case modeSPole, modeNPole:
		lp.Phi = math.Abs(Q.phalfpi - lp.Phi)
		if !Q.nocut && (lp.Phi-eps10) > support.PiOverTwo {
			return nil, merror.New(merror.ToleranceCondition)
		}
		lp.Phi *= 0.5
		if lp.Phi > eps10 {
			t = math.Tan(lp.Phi)
			Krho = -2. * (math.Log(math.Cos(lp.Phi))/t + t*Q.Cb)
			xy.X = Krho * sinlam
			xy.Y = Krho * coslam
			if Q.mode == modeNPole {
				xy.Y = -xy.Y
			}
		} else {
			xy.X = 0.
			xy.Y = 0.
		}
	}

	return xy, nil
}

// Inverse is not allowed
func (*Airy) Inverse(*core.CoordXY) (*core.CoordLP, error) {
	panic("no such conversion")
}

func (op *Airy) setup(sys *core.System) error {
	var beta float64

	Q := op
	P := op.System
	PE := op.System.Ellipsoid

	Q.nocut = P.ProjString.ContainsKey("no_cut")
	latb, ok := P.ProjString.GetAsFloat("lat_b")
	if !ok {
		latb = 0.0
	}
	latb = support.DDToR(latb)

	beta = 0.5 * (support.PiOverTwo - latb)
	if math.Abs(beta) < eps10 {
		Q.Cb = -0.5
	} else {
		Q.Cb = 1. / math.Tan(beta)
		Q.Cb *= Q.Cb * math.Log(math.Cos(beta))
	}

	if math.Abs(math.Abs(P.Phi0)-support.PiOverTwo) < eps10 {
		if P.Phi0 < 0. {
			Q.phalfpi = -support.PiOverTwo
			Q.mode = modeSPole
		} else {
			Q.phalfpi = support.PiOverTwo
			Q.mode = modeNPole
		}
	} else {
		if math.Abs(P.Phi0) < eps10 {
			Q.mode = modeEquit
		} else {
			Q.mode = modeObliq
			Q.sinph0 = math.Sin(P.Phi0)
			Q.cosph0 = math.Cos(P.Phi0)
		}
	}

	PE.Es = 0.

	return nil
}
