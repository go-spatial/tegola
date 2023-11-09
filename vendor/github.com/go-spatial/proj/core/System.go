// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package core

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/go-spatial/proj/merror"
	"github.com/go-spatial/proj/support"
)

// DatumType is the enum for the types of datums we support
type DatumType int

// All the DatumType constants (taken directly from the C)
const (
	DatumTypeUnknown   DatumType = 0
	DatumType3Param              = 1
	DatumType7Param              = 2
	DatumTypeGridShift           = 3
	DatumTypeWGS84               = 4 /* WGS84 (or anything considered equivalent) */
)

// IOUnitsType is the enum for the types of input/output units we support
type IOUnitsType int

// All the IOUnitsType constants
const (
	IOUnitsWhatever  IOUnitsType = 0 /* Doesn't matter (or depends on pipeline neighbours) */
	IOUnitsClassic               = 1 /* Scaled meters (right), projected system */
	IOUnitsProjected             = 2 /* Meters, projected system */
	IOUnitsCartesian             = 3 /* Meters, 3D cartesian system */
	IOUnitsAngular               = 4 /* Radians */
)

// DirectionType is the enum for the operation's direction
type DirectionType int

// All the DirectionType constants
const (
	DirectionForward  DirectionType = 1  /* Forward    */
	DirectionIdentity               = 0  /* Do nothing */
	DirectionInverse                = -1 /* Inverse    */
)

const epsLat = 1.0e-12

// System contains all the info needed to describe a coordinate system and
// related info.
//
// This type needs to be improved, as it is still quite close to the
// original C. Types like Ellipsoid and Datum represent a first step
// towards a refactoring.
type System struct {
	ProjString *support.ProjString
	OpDescr    *OperationDescription

	//
	// COORDINATE HANDLING
	//
	Over         bool /* Over-range flag */
	Geoc         bool /* Geocentric latitude flag */
	IsLatLong    bool /* proj=latlong ... not really a projection at all */
	IsGeocentric bool /* proj=geocent ... not really a projection at all */
	NeedEllps    bool /* 0 for operations that are purely cartesian */

	Left  IOUnitsType /* Flags for input/output coordinate types */
	Right IOUnitsType

	//
	// ELLIPSOID
	//
	Ellipsoid *Ellipsoid

	//
	// CARTOGRAPHIC OFFSETS
	//
	Lam0, Phi0     float64 /* central meridian, parallel */
	X0, Y0, Z0, T0 float64 /* false easting and northing (and height and time) */

	//
	// SCALING
	//
	K0                   float64 /* General scaling factor - e.g. the 0.9996 of UTM */
	ToMeter, FromMeter   float64 /* Plane coordinate scaling. Internal unit [m] */
	VToMeter, VFromMeter float64 /* Vertical scaling. Internal unit [m] */

	//
	// DATUMS AND HEIGHT SYSTEMS
	//
	DatumType   DatumType  /* PJD_UNKNOWN/3PARAM/7PARAM/GRIDSHIFT/WGS84 */
	DatumParams [7]float64 /* Parameters for 3PARAM and 7PARAM */

	//struct _pj_gi **gridlist;
	//int     gridlist_count;

	HasGeoidVgrids bool
	//struct _pj_gi **vgridlist_geoid;
	//int     vgridlist_geoid_count;

	FromGreenwich  float64 /* prime meridian offset (in radians) */
	LongWrapCenter float64 /* 0.0 for -180 to 180, actually in radians*/
	IsLongWrapSet  bool
	Axis           string /* Axis order, pj_transform/pj_adjust_axis */

	/* New Datum Shift Grid Catalogs */
	CatalogName string
	//struct _PJ_GridCatalog *catalog;
	DatumDate float64

	//struct _pj_gi *last_before_grid;    /* TODO: Description needed */
	//PJ_Region     last_before_region;   /* TODO: Description needed */
	//double        last_before_date;     /* TODO: Description needed */

	//struct _pj_gi *last_after_grid;     /* TODO: Description needed */
	//PJ_Region     last_after_region;    /* TODO: Description needed */
	//double        last_after_date;      /* TODO: Description needed */
}

// NewSystem returns a new System object
func NewSystem(ps *support.ProjString) (*System, IOperation, error) {

	err := ValidateProjStringContents(ps)
	if err != nil {
		return nil, nil, err
	}

	sys := &System{
		ProjString: ps,
		NeedEllps:  true,
		Left:       IOUnitsAngular,
		Right:      IOUnitsClassic,
		Axis:       "enu",
	}

	err = sys.initialize()
	if err != nil {
		return nil, nil, err
	}

	op, err := sys.OpDescr.CreateOperation(sys)
	if err != nil {
		return nil, nil, err
	}

	return sys, op, nil
}

// ValidateProjStringContents checks to mke sure the contents are semantically valid
func ValidateProjStringContents(pl *support.ProjString) error {

	// TODO: we don't support +init
	if pl.CountKey("init") > 0 {
		return merror.New(merror.UnsupportedProjectionString, "init")
	}

	// TODO: we don't support +pipeline
	if pl.CountKey("pipeline") > 0 {
		return merror.New(merror.UnsupportedProjectionString, "pipeline")
	}

	// you have to say +proj=...
	if pl.CountKey("proj") != 1 {
		return merror.New(merror.InvalidProjectionSyntax, "proj must appear exactly once")
	}
	projName, ok := pl.GetAsString("proj")
	if !ok || projName == "" {
		return merror.New(merror.InvalidProjectionSyntax, "proj=?")
	}

	// explicitly call out stuff we don't support yet
	if pl.ContainsKey("axis") {
		return merror.New(merror.UnsupportedProjectionString, "axis")
	}
	if pl.ContainsKey("geoidgrids") {
		return merror.New(merror.UnsupportedProjectionString, "geoidgrids")
	}
	if pl.ContainsKey("to_meter") {
		return merror.New(merror.UnsupportedProjectionString, "to_meter")
	}

	return nil
}

func (sys *System) String() string {
	b, err := json.MarshalIndent(sys, "", " ")
	if err != nil {
		panic(err)
	}

	return string(b)
}

func (sys *System) initialize() error {

	projName, _ := sys.ProjString.GetAsString("proj")
	opDescr, ok := OperationDescriptionTable[projName]
	if !ok {
		return merror.New(merror.UnknownProjection, projName)
	}

	sys.OpDescr = opDescr

	err := sys.processDatum()
	if err != nil {
		return err
	}

	err = sys.processEllipsoid()
	if err != nil {
		return err
	}

	/* Now that we have ellipse information check for WGS84 datum */
	if sys.DatumType == DatumType3Param &&
		sys.DatumParams[0] == 0.0 &&
		sys.DatumParams[1] == 0.0 &&
		sys.DatumParams[2] == 0.0 &&
		sys.Ellipsoid.A == 6378137.0 &&
		math.Abs(sys.Ellipsoid.Es-0.006694379990) < 0.000000000050 {
		/*WGS84/GRS80*/
		sys.DatumType = DatumTypeWGS84
	}

	return sys.processMisc()
}

func (sys *System) processDatum() error {

	sys.DatumType = DatumTypeUnknown

	datumName, ok := sys.ProjString.GetAsString("datum")
	if ok {

		datum, ok := support.DatumsTable[datumName]
		if !ok {
			return merror.New(merror.NoSuchDatum)
		}

		// add the ellipse to the end of the list
		// TODO: move this into the ProjString processor?

		if datum.EllipseID != "" {
			sys.ProjString.Add(support.Pair{Key: "ellps", Value: datum.EllipseID})
		}
		if datum.DefinitionString != "" {
			sys.ProjString.AddList(datum.Definition)
		}
	}

	if sys.ProjString.ContainsKey("nadgrids") {
		sys.DatumType = DatumTypeGridShift

	} else if sys.ProjString.ContainsKey("catalog") {
		sys.DatumType = DatumTypeGridShift
		catalogName, ok := sys.ProjString.GetAsString("catalog")
		if !ok {
			return merror.New(merror.UnsupportedProjectionString, catalogName)
		}
		sys.CatalogName = catalogName
		datumDate, ok := sys.ProjString.GetAsString("date")
		if !ok {
			return merror.New(merror.UnsupportedProjectionString, datumDate)
		}
		if datumDate != "" {
			sys.DatumDate = support.ParseDate(datumDate)
		}

	} else if sys.ProjString.ContainsKey("towgs84") {

		values, ok := sys.ProjString.GetAsFloats("towgs84")
		if !ok {
			return merror.New(merror.InvalidProjectionSyntax, "towgs84")
		}

		if len(values) == 3 {
			sys.DatumType = DatumType3Param

			sys.DatumParams[0] = values[0]
			sys.DatumParams[1] = values[1]
			sys.DatumParams[2] = values[2]

		} else if len(values) == 7 {
			sys.DatumType = DatumType7Param

			sys.DatumParams[0] = values[0]
			sys.DatumParams[1] = values[1]
			sys.DatumParams[2] = values[2]
			sys.DatumParams[3] = values[3]
			sys.DatumParams[4] = values[4]
			sys.DatumParams[5] = values[5]
			sys.DatumParams[6] = values[6]

			// transform from arc seconds to radians
			sys.DatumParams[3] = support.ConvertArcsecondsToRadians(sys.DatumParams[3])
			sys.DatumParams[4] = support.ConvertArcsecondsToRadians(sys.DatumParams[4])
			sys.DatumParams[5] = support.ConvertArcsecondsToRadians(sys.DatumParams[5])

			// transform from parts per million to scaling factor
			sys.DatumParams[6] = (sys.DatumParams[6] / 1000000.0) + 1

			/* Note that pj_init() will later switch datum_type to
			   PJD_WGS84 if shifts are all zero, and ellipsoid is WGS84 or GRS80 */
		} else {
			return merror.New(merror.InvalidProjectionSyntax)
		}
	}

	return nil
}

func (sys *System) processEllipsoid() error {

	ellipsoid, err := NewEllipsoid(sys)
	if err != nil {
		return err
	}

	if ellipsoid == nil {
		/* Didn't get an ellps, but doesn't need one: Get a free WGS84 */
		if sys.NeedEllps {
			return merror.New(merror.ProjectionStringRequiresEllipse)
		}

		ellipsoid = &Ellipsoid{}
		ellipsoid.F = 1.0 / 298.257223563
		ellipsoid.AOrig = 6378137.0
		ellipsoid.A = 6378137.0
		ellipsoid.EsOrig = ellipsoid.F * (2 - ellipsoid.F)
		ellipsoid.Es = ellipsoid.F * (2 - ellipsoid.F)
	}

	ellipsoid.AOrig = ellipsoid.A
	ellipsoid.EsOrig = ellipsoid.Es

	err = ellipsoid.doCalcParams(ellipsoid.A, ellipsoid.Es)
	if err != nil {
		return err
	}

	sys.Ellipsoid = ellipsoid

	return nil
}

func (sys *System) readUnits(vertical bool) (float64, float64, error) {

	units := "units"
	toMeter := "toMeter"

	var to, from float64

	if vertical {
		units = "v" + units
		toMeter = "v" + toMeter
	}

	name, ok := sys.ProjString.GetAsString(units)
	var s string
	if ok {
		u, ok := support.UnitsTable[name]
		if !ok {
			return 0.0, 0.0, merror.New(merror.UnknownUnit)
		}
		s = u.ToMetersS
	}

	if sys.ProjString.ContainsKey(toMeter) {
		s, _ = sys.ProjString.GetAsString(toMeter)
	}

	if s != "" {
		var factor float64
		var ratio = false

		/* ratio number? */
		if len(s) > 1 && s[0:1] == "1" && s[1:2] == "/" {
			ratio = true
			s = s[2:]
		}

		factor, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0.0, 0.0, merror.New(merror.InvalidProjectionSyntax, s)
		}
		if (factor <= 0.0) || (1.0/factor == 0.0) {
			return 0.0, 0.0, merror.New(merror.UnitFactorLessThanZero)
		}

		if ratio {
			to = 1.0 / factor
		} else {
			to = factor
		}

		from = 1.0 / sys.FromMeter
	} else {
		to = 1.0
		from = 1.0
	}

	return to, from, nil
}

func (sys *System) processMisc() error {

	/* Set PIN->geoc coordinate system */
	sys.Geoc = (sys.Ellipsoid.Es != 0.0 && sys.ProjString.ContainsKey("geoc"))

	/* Over-ranging flag */
	sys.Over = sys.ProjString.ContainsKey("over")

	/* Vertical datum geoid grids */
	sys.HasGeoidVgrids = sys.ProjString.ContainsKey("geoidgrids")

	/* Longitude center for wrapping */
	sys.IsLongWrapSet = sys.ProjString.ContainsKey("lon_wrap")
	if sys.IsLongWrapSet {
		sys.LongWrapCenter, _ = sys.ProjString.GetAsFloat("lon_wrap")
		/* Don't accept excessive values otherwise we might perform badly */
		/* when correcting longitudes around it */
		/* The test is written this way to error on long_wrap_center "=" NaN */
		if !(math.Abs(sys.LongWrapCenter) < 10.0*support.TwoPi) {
			return merror.New(merror.LatOrLonExceededLimit)
		}
	}

	err := sys.processAxis()
	if err != nil {
		return err
	}

	/* Central meridian */
	f, ok := sys.ProjString.GetAsFloat("lon_0")
	if ok {
		sys.Lam0 = f
	}

	/* Central latitude */
	f, ok = sys.ProjString.GetAsFloat("lat_0")
	if ok {
		sys.Phi0 = f
	}

	/* False easting and northing */
	f, ok = sys.ProjString.GetAsFloat("x_0")
	if ok {
		sys.X0 = f
	}
	f, ok = sys.ProjString.GetAsFloat("y_0")
	if ok {
		sys.Y0 = f
	}
	f, ok = sys.ProjString.GetAsFloat("z_0")
	if ok {
		sys.Z0 = f
	}
	f, ok = sys.ProjString.GetAsFloat("t_0")
	if ok {
		sys.T0 = f
	}

	err = sys.processScaling()
	if err != nil {
		return err
	}

	err = sys.processUnits()
	if err != nil {
		return err
	}

	return sys.processMeridian()
}

func (sys *System) processScaling() error {

	/* General scaling factor */
	if sys.ProjString.ContainsKey("k_0") {
		sys.K0, _ = sys.ProjString.GetAsFloat("k_0")
	} else if sys.ProjString.ContainsKey("k") {
		sys.K0, _ = sys.ProjString.GetAsFloat("k")
	} else {
		sys.K0 = 1.0
	}
	if sys.K0 <= 0.0 {
		return merror.New(merror.KLessThanZero)
	}

	return nil
}

func (sys *System) processMeridian() error {

	/* Prime meridian */
	name, ok := sys.ProjString.GetAsString("pm")
	if ok {
		var value string
		pm, ok := support.MeridiansTable[name]
		if ok {
			value = pm.Definition
		} else {
			value = name
		}
		f, err := support.DMSToR(value)
		if err != nil {
			return err
		}
		sys.FromGreenwich = f
	} else {
		sys.FromGreenwich = 0.0
	}

	return nil
}
func (sys *System) processUnits() error {

	to, from, err := sys.readUnits(false)
	if err != nil {
		return err
	}
	sys.ToMeter = to
	sys.FromMeter = from

	to, from, err = sys.readUnits(true)
	if err != nil {
		return err
	}
	sys.VToMeter = to
	sys.VFromMeter = from

	return nil
}

func (sys *System) processAxis() error {
	/* Axis orientation */
	if sys.ProjString.ContainsKey("axis") {
		axisLegal := "ewnsud"
		axisArg, _ := sys.ProjString.GetAsString("axis")
		if len(axisArg) != 3 {
			return merror.New(merror.Axis)
		}

		if !strings.ContainsAny(axisArg[0:1], axisLegal) ||
			!strings.ContainsAny(axisArg[1:2], axisLegal) ||
			!strings.ContainsAny(axisArg[2:3], axisLegal) {
			return merror.New(merror.Axis)
		}

		/* TODO: it would be nice to validate we don't have on axis repeated */
		sys.Axis = axisArg
	}

	return nil
}

// GeocentricLatitude converts geographical latitude to geocentric
// or the other way round if direction = PJ_INV
func GeocentricLatitude(op *System, direction DirectionType, lp *CoordLP) *CoordLP {
	/**************************************************************************************

		The conversion involves a call to the tangent function, which goes through the
		roof at the poles, so very close (the last centimeter) to the poles no
		conversion takes place and the input latitude is copied directly to the output.

		Fortunately, the geocentric latitude converges to the geographical at the
		poles, so the difference is negligible.

		For the spherical case, the geographical latitude equals the geocentric, and
		consequently, the input is copied directly to the output.
	**************************************************************************************/
	const limit = support.PiOverTwo - 1e-9

	res := lp

	if (lp.Phi > limit) || (lp.Phi < -limit) || (op.Ellipsoid.Es == 0) {
		return res
	}
	if direction == DirectionForward {
		res.Phi = math.Atan(op.Ellipsoid.OneEs * math.Tan(lp.Phi))
	} else {
		res.Phi = math.Atan(op.Ellipsoid.ROneEs * math.Tan(lp.Phi))
	}

	return res
}
