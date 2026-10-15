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
	core.RegisterConvertLPToXY("utm",
		"Universal Transverse Mercator (UTM)",
		"\n\tCyl, Sph\n\tzone= south",
		NewUtm,
	)
	core.RegisterConvertLPToXY("etmerc",
		"Extended Transverse Mercator (UTM)",
		"\n\tCyl, Sph\n\tlat_ts=(0)\nlat_0=(0)",
		NewEtMerc,
	)
}

// EtMerc implements core.IOperation and core.ConvertLPToXY
type EtMerc struct {
	core.Operation
	isUtm bool

	// the "opaque" parts
	Qn  float64    /* Merid. quad., scaled to the projection */
	Zb  float64    /* Radius vector in polar coord. systems  */
	cgb [6]float64 /* Constants for Gauss -> Geo lat */
	cbg [6]float64 /* Constants for Geo lat -> Gauss */
	utg [6]float64 /* Constants for transv. merc. -> geo */
	gtu [6]float64 /* Constants for geo -> transv. merc. */
}

// NewEtMerc returns a new EtMerc
func NewEtMerc(system *core.System, desc *core.OperationDescription) (core.IConvertLPToXY, error) {
	op := &EtMerc{
		isUtm: false,
	}
	op.System = system

	err := op.etmercSetup(system)
	if err != nil {
		return nil, err
	}
	return op, nil
}

// NewUtm returns a new EtMerc
func NewUtm(system *core.System, desc *core.OperationDescription) (core.IConvertLPToXY, error) {
	op := &EtMerc{
		isUtm: true,
	}
	op.System = system

	err := op.utmSetup(system)
	if err != nil {
		return nil, err
	}
	return op, nil
}

//---------------------------------------------------------------------

const etmercOrder = 6

//---------------------------------------------------------------------

func log1py(x float64) float64 { /* Compute log(1+x) accurately */
	y := 1.0 + x
	z := y - 1.0
	/* Here's the explanation for this magic: y = 1 + z, exactly, and z
	 * approx x, thus log(y)/z (which is nearly constant near z = 0) returns
	 * a good approximation to the true log(1 + x)/x.  The multiplication x *
	 * (log(y)/z) introduces little additional error. */
	if z == 0 {
		return x
	}
	return x * math.Log(y) / z
}

func asinhy(x float64) float64 { /* Compute asinh(x) accurately */
	y := math.Abs(x) /* Enforce odd parity */
	y = log1py(y * (1 + y/(math.Hypot(1.0, y)+1)))
	if x < 0 {
		return -y
	}
	return y
}

func gatg(p1 []float64, lenP1 int, B float64) float64 {
	// TODO: review translation
	var pi int
	var h = 0.0
	var h1 float64
	var h2 = 0.0

	cos2B := 2 * math.Cos(2*B)

	pi = lenP1
	pi--
	h1 = p1[pi]
	for pi != 0 {
		pi--
		h = -h2 + cos2B*h1 + p1[pi]
		h2 = h1
		h1 = h
	}

	return (B + h*math.Sin(2*B))
}

/* Complex Clenshaw summation */
func clenS(a []float64, lenA int, argR float64, argI float64, R *float64, I *float64) float64 {
	// TODO: review translation
	var ai int
	var r, i, hr, hr1, hr2, hi, hi1, hi2 float64
	var sinArgR, cosArgR, sinhArgI, coshArgI float64

	/* arguments */
	ai = lenA
	sinArgR, cosArgR = math.Sincos(argR)
	sinhArgI = math.Sinh(argI)
	coshArgI = math.Cosh(argI)
	r = 2 * cosArgR * coshArgI
	i = -2 * sinArgR * sinhArgI

	/* summation loop */
	hi1 = 0.0
	hr1 = 0.0
	hi = 0.0
	ai--
	hr = a[ai]
	for ai != 0 {
		hr2 = hr1
		hi2 = hi1
		hr1 = hr
		hi1 = hi
		ai--
		hr = -hr2 + r*hr1 - i*hi1 + a[ai]
		hi = -hi2 + i*hr1 + r*hi1
	}

	r = sinArgR * coshArgI
	i = cosArgR * sinhArgI
	*R = r*hr - i*hi
	*I = r*hi + i*hr
	return *R
}

/* Real Clenshaw summation */
func clens(a []float64, lenA int, argR float64) float64 {
	var ai int
	var r, hr, hr1, hr2, cosArgR float64

	ai = lenA
	cosArgR = math.Cos(argR)
	r = 2 * cosArgR

	/* summation loop */
	hr1 = 0
	ai--
	hr = a[ai]
	for ai != 0 {
		hr2 = hr1
		hr1 = hr
		ai--
		hr = -hr2 + r*hr1 + a[ai]
	}
	return math.Sin(argR) * hr
}

//---------------------------------------------------------------------------

// Forward operation -- Ellipsoidal, forward
func (op *EtMerc) Forward(lp *core.CoordLP) (*core.CoordXY, error) {

	xy := &core.CoordXY{X: 0.0, Y: 0.0}

	var Q = op
	var sinCn, cosCn, cosCe, sinCe, dCn, dCe float64
	Cn := lp.Phi
	Ce := lp.Lam

	/* ell. LAT, LNG -> Gaussian LAT, LNG */
	Cn = gatg(Q.cbg[:], etmercOrder, Cn)
	/* Gaussian LAT, LNG -> compl. sph. LAT */
	sinCn, cosCn = math.Sincos(Cn)
	sinCe, cosCe = math.Sincos(Ce)

	Cn = math.Atan2(sinCn, cosCe*cosCn)
	Ce = math.Atan2(sinCe*cosCn, math.Hypot(sinCn, cosCn*cosCe))

	/* compl. sph. N, E -> ell. norm. N, E */
	Ce = asinhy(math.Tan(Ce)) /* Replaces: Ce  = log(tan(FORTPI + Ce*0.5)); */
	Cn += clenS(Q.gtu[:], etmercOrder, 2*Cn, 2*Ce, &dCn, &dCe)
	Ce += dCe
	if math.Abs(Ce) <= 2.623395162778 {
		xy.Y = Q.Qn*Cn + Q.Zb /* Northing */
		xy.X = Q.Qn * Ce      /* Easting  */
	} else {
		xy.X = math.MaxFloat64
		xy.Y = math.MaxFloat64
	}
	return xy, nil
}

// Inverse operation (Ellipsoidal, inverse)
func (op *EtMerc) Inverse(xy *core.CoordXY) (*core.CoordLP, error) {

	lp := &core.CoordLP{Lam: 0.0, Phi: 0.0}

	Q := op
	var sinCn, cosCn, cosCe, sinCe, dCn, dCe float64
	Cn := xy.Y
	Ce := xy.X

	/* normalize N, E */
	Cn = (Cn - Q.Zb) / Q.Qn
	Ce = Ce / Q.Qn

	if math.Abs(Ce) <= 2.623395162778 { /* 150 degrees */
		/* norm. N, E -> compl. sph. LAT, LNG */
		Cn += clenS(Q.utg[:], etmercOrder, 2*Cn, 2*Ce, &dCn, &dCe)
		Ce += dCe
		Ce = math.Atan(math.Sinh(Ce)) /* Replaces: Ce = 2*(atan(exp(Ce)) - FORTPI); */
		/* compl. sph. LAT -> Gaussian LAT, LNG */
		sinCn, cosCn = math.Sincos(Cn)
		sinCe, cosCe = math.Sincos(Ce)
		Ce = math.Atan2(sinCe, cosCe*cosCn)
		Cn = math.Atan2(sinCn*cosCe, math.Hypot(sinCe, cosCe*cosCn))
		/* Gaussian LAT, LNG -> ell. LAT, LNG */
		lp.Phi = gatg(Q.cgb[:], etmercOrder, Cn)
		lp.Lam = Ce
	} else {
		lp.Phi = math.MaxFloat64
		lp.Lam = math.MaxFloat64
	}
	return lp, nil
}

/* general initialization */
func (op *EtMerc) setup(P *core.System) error {
	var f, n, np, Z float64

	Q := op

	PE := P.Ellipsoid

	if PE.Es <= 0 {
		return merror.New(merror.EllipsoidUseRequired)
	}

	/* flattening */
	f = PE.Es / (1 + math.Sqrt(1.0-PE.Es)) /* Replaces: f = 1 - sqrt(1-P->es); */

	/* third flattening */
	n = f / (2.0 - f)
	np = n

	/* COEF. OF TRIG SERIES GEO <-> GAUSS */
	/* cgb := Gaussian -> Geodetic, KW p190 - 191 (61) - (62) */
	/* cbg := Geodetic -> Gaussian, KW p186 - 187 (51) - (52) */
	/* PROJ_ETMERC_ORDER = 6th degree : Engsager and Poder: ICC2007 */

	Q.cgb[0] = n * (2 + n*(-2/3.0+n*(-2+n*(116/45.0+n*(26/45.0+
		n*(-2854/675.0))))))
	Q.cbg[0] = n * (-2 + n*(2/3.0+n*(4/3.0+n*(-82/45.0+n*(32/45.0+
		n*(4642/4725.0))))))
	np *= n
	Q.cgb[1] = np * (7/3.0 + n*(-8/5.0+n*(-227/45.0+n*(2704/315.0+
		n*(2323/945.0)))))
	Q.cbg[1] = np * (5/3.0 + n*(-16/15.0+n*(-13/9.0+n*(904/315.0+
		n*(-1522/945.0)))))
	np *= n
	/* n^5 coeff corrected from 1262/105 -> -1262/105 */
	Q.cgb[2] = np * (56/15.0 + n*(-136/35.0+n*(-1262/105.0+
		n*(73814/2835.0))))
	Q.cbg[2] = np * (-26/15.0 + n*(34/21.0+n*(8/5.0+
		n*(-12686/2835.0))))
	np *= n
	/* n^5 coeff corrected from 322/35 -> 332/35 */
	Q.cgb[3] = np * (4279/630.0 + n*(-332/35.0+n*(-399572/14175.0)))
	Q.cbg[3] = np * (1237/630.0 + n*(-12/5.0+n*(-24832/14175.0)))
	np *= n
	Q.cgb[4] = np * (4174/315.0 + n*(-144838/6237.0))
	Q.cbg[4] = np * (-734/315.0 + n*(109598/31185.0))
	np *= n
	Q.cgb[5] = np * (601676 / 22275.0)
	Q.cbg[5] = np * (444337 / 155925.0)

	/* Constants of the projections */
	/* Transverse Mercator (UTM, ITM, etc) */
	np = n * n
	/* Norm. mer. quad, K&W p.50 (96), p.19 (38b), p.5 (2) */
	Q.Qn = P.K0 / (1 + n) * (1 + np*(1/4.0+np*(1/64.0+np/256.0)))
	/* coef of trig series */
	/* utg := ell. N, E -> sph. N, E,  KW p194 (65) */
	/* gtu := sph. N, E -> ell. N, E,  KW p196 (69) */
	Q.utg[0] = n * (-0.5 + n*(2/3.0+n*(-37/96.0+n*(1/360.0+
		n*(81/512.0+n*(-96199/604800.0))))))
	Q.gtu[0] = n * (0.5 + n*(-2/3.0+n*(5/16.0+n*(41/180.0+
		n*(-127/288.0+n*(7891/37800.0))))))
	Q.utg[1] = np * (-1/48.0 + n*(-1/15.0+n*(437/1440.0+n*(-46/105.0+
		n*(1118711/3870720.0)))))
	Q.gtu[1] = np * (13/48.0 + n*(-3/5.0+n*(557/1440.0+n*(281/630.0+
		n*(-1983433/1935360.0)))))
	np *= n
	Q.utg[2] = np * (-17/480.0 + n*(37/840.0+n*(209/4480.0+
		n*(-5569/90720.0))))
	Q.gtu[2] = np * (61/240.0 + n*(-103/140.0+n*(15061/26880.0+
		n*(167603/181440.0))))
	np *= n
	Q.utg[3] = np * (-4397/161280.0 + n*(11/504.0+n*(830251/7257600.0)))
	Q.gtu[3] = np * (49561/161280.0 + n*(-179/168.0+n*(6601661/7257600.0)))
	np *= n
	Q.utg[4] = np * (-4583/161280.0 + n*(108847/3991680.0))
	Q.gtu[4] = np * (34729/80640.0 + n*(-3418889/1995840.0))
	np *= n
	Q.utg[5] = np * (-20648693 / 638668800.0)
	Q.gtu[5] = np * (212378941 / 319334400.0)

	/* Gaussian latitude value of the origin latitude */
	Z = gatg(Q.cbg[:], etmercOrder, P.Phi0)

	/* Origin northing minus true northing at the origin latitude */
	/* i.e. true northing = N - P->Zb                         */
	Q.Zb = -Q.Qn * (Z + clens(Q.gtu[:], etmercOrder, 2*Z))

	return nil
}

func (op *EtMerc) etmercSetup(sys *core.System) error {

	return op.setup(sys)
}

/* utm uses etmerc for the underlying projection */

func (op *EtMerc) utmSetup(sys *core.System) error {

	if sys.Ellipsoid.Es == 0.0 {
		return merror.New(merror.EllipsoidUseRequired)
	}
	if sys.Lam0 < -1000.0 || sys.Lam0 > 1000.0 {
		return merror.New(merror.InvalidUTMZone)
	}

	sys.Y0 = 0.0
	if sys.ProjString.ContainsKey("south") {
		sys.Y0 = 10000000.0
	}
	sys.X0 = 500000.0

	zone, ok := sys.ProjString.GetAsInt("zone") /* zone input ? */
	if ok {
		if zone > 0 && zone <= 60 {
			zone--
		} else {
			return merror.New(merror.InvalidUTMZone)
		}
	} else { /* nearest central meridian input */
		zone = (int)(math.Floor((support.Adjlon(sys.Lam0) + support.Pi) * 30. / support.Pi))
		if zone < 0 {
			zone = 0
		} else if zone >= 60 {
			zone = 59
		}
	}
	sys.Lam0 = (float64(zone)+0.5)*support.Pi/30.0 - support.Pi
	sys.K0 = 0.9996
	sys.Phi0 = 0.0

	return op.setup(sys)
}
