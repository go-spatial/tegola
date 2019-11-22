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
)

func init() {
	core.RegisterConvertLPToXY("august",
		"August Epicycloidal",
		"\n\tMisc Sph, no inv.",
		NewAugust,
	)
}

// August implements core.IOperation and core.ConvertLPToXY
type August struct {
	core.Operation
}

const m = 1.333333333333333

// NewAugust returns a new August
func NewAugust(system *core.System, desc *core.OperationDescription) (core.IConvertLPToXY, error) {
	op := &August{}
	op.System = system

	PE := op.System.Ellipsoid
	PE.Es = 0.0

	return op, nil
}

// Forward goes forewards
func (op *August) Forward(lp *core.CoordLP) (*core.CoordXY, error) {
	xy := &core.CoordXY{X: 0.0, Y: 0.0}

	var t, c1, c, x1, x12, y1, y12 float64

	t = math.Tan(.5 * lp.Phi)
	c1 = math.Sqrt(1. - t*t)
	lp.Lam *= .5
	c = 1. + c1*math.Cos(lp.Lam)
	x1 = math.Sin(lp.Lam) * c1 / c
	y1 = t / c
	x12 = x1 * x1
	y12 = y1 * y1
	xy.X = m * x1 * (3. + x12 - 3.*y12)
	xy.Y = m * y1 * (3. + 3.*x12 - y12)

	return xy, nil
}

// Inverse is not allowed
func (*August) Inverse(*core.CoordXY) (*core.CoordLP, error) {
	panic("no such conversion")
}
