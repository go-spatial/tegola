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
	core.RegisterConvertLPToXY("merc",
		"Universal Transverse Mercator (UTM)",
		"\n\tCyl, Sph&Ell\n\tlat_ts=",
		NewMerc,
	)
}

// Merc implements core.IOperation and core.ConvertLPToXY
type Merc struct {
	core.Operation
	isSphere bool
}

// NewMerc returns a new Merc
func NewMerc(system *core.System, desc *core.OperationDescription) (core.IConvertLPToXY, error) {
	op := &Merc{
		isSphere: false,
	}
	op.System = system

	err := op.mercSetup(system)
	if err != nil {
		return nil, err
	}
	return op, nil
}

// Forward goes forewards
func (op *Merc) Forward(lp *core.CoordLP) (*core.CoordXY, error) {

	if op.isSphere {
		return op.sphericalForward(lp)
	}
	return op.ellipsoidalForward(lp)
}

// Inverse goes backwards
func (op *Merc) Inverse(xy *core.CoordXY) (*core.CoordLP, error) {

	if op.isSphere {
		return op.sphericalInverse(xy)
	}
	return op.ellipsoidalInverse(xy)
}

//---------------------------------------------------------------------

func (op *Merc) ellipsoidalForward(lp *core.CoordLP) (*core.CoordXY, error) { /* Ellipsoidal, forward */
	xy := &core.CoordXY{X: 0.0, Y: 0.0}

	P := op.System
	PE := op.System.Ellipsoid

	if math.Abs(math.Abs(lp.Phi)-support.PiOverTwo) <= eps10 {
		return xy, merror.New(merror.ToleranceCondition)
	}
	xy.X = P.K0 * lp.Lam
	xy.Y = -P.K0 * math.Log(support.Tsfn(lp.Phi, math.Sin(lp.Phi), PE.E))
	return xy, nil
}

func (op *Merc) sphericalForward(lp *core.CoordLP) (*core.CoordXY, error) { /* Spheroidal, forward */
	xy := &core.CoordXY{X: 0.0, Y: 0.0}

	P := op.System

	if math.Abs(math.Abs(lp.Phi)-support.PiOverTwo) <= eps10 {
		return xy, merror.New(merror.ToleranceCondition)
	}
	xy.X = P.K0 * lp.Lam
	xy.Y = P.K0 * math.Log(math.Tan(support.PiOverFour+.5*lp.Phi))
	return xy, nil
}

func (op *Merc) ellipsoidalInverse(xy *core.CoordXY) (*core.CoordLP, error) { /* Ellipsoidal, inverse */
	lp := &core.CoordLP{Lam: 0.0, Phi: 0.0}

	P := op.System
	PE := op.System.Ellipsoid
	var err error

	lp.Phi, err = support.Phi2(math.Exp(-xy.Y/P.K0), PE.E)
	if err != nil {
		return nil, err
	}
	if lp.Phi == math.MaxFloat64 {
		return lp, merror.New(merror.ToleranceCondition)
	}
	lp.Lam = xy.X / P.K0
	return lp, nil
}

func (op *Merc) sphericalInverse(xy *core.CoordXY) (*core.CoordLP, error) { /* Spheroidal, inverse */
	lp := &core.CoordLP{Lam: 0.0, Phi: 0.0}

	P := op.System

	lp.Phi = support.PiOverTwo - 2.*math.Atan(math.Exp(-xy.Y/P.K0))
	lp.Lam = xy.X / P.K0
	return lp, nil
}

func (op *Merc) mercSetup(sys *core.System) error {
	var phits float64

	ps := op.System.ProjString

	isPhits := ps.ContainsKey("lat_ts")
	if isPhits {
		phits, _ = ps.GetAsFloat("lat_ts")
		phits = support.DDToR(phits)
		phits = math.Abs(phits)
		if phits >= support.PiOverTwo {
			return merror.New(merror.LatTSLargerThan90)
		}
	}

	P := op.System
	PE := op.System.Ellipsoid

	if PE.Es != 0.0 { /* ellipsoid */
		op.isSphere = false
		if isPhits {
			P.K0 = support.Msfn(math.Sin(phits), math.Cos(phits), PE.Es)
		}
	} else { /* sphere */
		op.isSphere = true
		if isPhits {
			P.K0 = math.Cos(phits)
		}
	}

	return nil
}
