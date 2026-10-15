package operations

import (
	"math"

	"github.com/go-spatial/proj/core"
	"github.com/go-spatial/proj/merror"
)

func init() {
	core.RegisterConvertLPToXY("eqc",
		"Equidistant Cylindrical (Plate Carree)",
		"\n\tCyl, Sph\n\tlat_ts=[, lat_0=0]",
		NewEqc,
	)
}

// Eqc implements core.IOperation and core.ConvertLPToXY
type Eqc struct {
	core.Operation
	rc float64
}

// NewEqc creates a new Plate Carree system
func NewEqc(system *core.System, desc *core.OperationDescription) (core.IConvertLPToXY, error) {
	op := &Eqc{}
	op.System = system

	err := op.eqcSetup(system)
	if err != nil {
		return nil, err
	}
	return op, nil
}

// Forward goes forewards
func (op *Eqc) Forward(lp *core.CoordLP) (*core.CoordXY, error) {
	return op.spheroidalForward(lp)
}

// Inverse goes backwards
func (op *Eqc) Inverse(xy *core.CoordXY) (*core.CoordLP, error) {
	return op.spheroidalReverse(xy)
}

//---------------------------------------------------------------------

func (op *Eqc) spheroidalForward(lp *core.CoordLP) (*core.CoordXY, error) { /* Ellipsoidal, forward */
	xy := &core.CoordXY{X: 0.0, Y: 0.0}

	P := op.System

	xy.X = op.rc * lp.Lam
	xy.Y = lp.Phi - P.Phi0
	return xy, nil
}

func (op *Eqc) spheroidalReverse(xy *core.CoordXY) (*core.CoordLP, error) { /* Ellipsoidal, inverse */
	lp := &core.CoordLP{Lam: 0.0, Phi: 0.0}

	P := op.System

	lp.Lam = xy.X / op.rc
	lp.Phi = xy.Y + P.Phi0
	return lp, nil
}

func (op *Eqc) eqcSetup(sys *core.System) error {
	ps := op.System.ProjString

	latts, _ := ps.GetAsFloat("lat_ts")
	if math.Cos(latts) <= 0 {
		return merror.New(merror.LatTSLargerThan90)
	}
	op.rc = math.Cos(latts)

	return nil
}
