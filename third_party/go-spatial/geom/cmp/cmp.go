package cmp

import (
	"math"

	"github.com/go-spatial/geom"
)

var (
	NilPoint           = (*geom.Point)(nil)
	NilMultiPoint      = (*geom.MultiPoint)(nil)
	NilLineString      = (*geom.LineString)(nil)
	NilMultiLineString = (*geom.MultiLineString)(nil)
	NilPoly            = (*geom.Polygon)(nil)
	NilMultiPoly       = (*geom.MultiPolygon)(nil)
	NilCollection      = (*geom.Collection)(nil)
)

// BitToleranceFor returns the BitToleranceFor the given tolerance
func BitToleranceFor(tol float64) int64 {
	return int64(math.Float64bits(1.0+tol) - math.Float64bits(1.0))
}

// These are here for compability reasons

// Tolerance is only here for compability reasons
var Tolerance = DefaultCompare().Tolerance

// BitTolerance is only here for compability reasons
var BitTolerance = DefaultCompare().BitTolerance

// Float64Slice compares two sets of float64 slices within the given tolerance.
func Float64Slice(f1, f2 []float64, tolerance float64, bitTolerance int64) bool {
	return Compare{
		Tolerance:    tolerance,
		BitTolerance: bitTolerance,
	}.FloatSlice(f1, f2)
}

// Float64 compares two floats to see if they are within the given tolerance.
func Float64(f1, f2, tolerance float64, bitTolerance int64) bool {
	return Compare{
		Tolerance:    tolerance,
		BitTolerance: bitTolerance,
	}.Float(f1, f2)
}
