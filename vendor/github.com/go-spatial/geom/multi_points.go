package geom

import "errors"

// ErrNilMultiPointS is thrown when a MultiPointS is nil but shouldn't be
var ErrNilMultiPointS = errors.New("geom: nil MultiPointS")

// MultiPointS is a geometry with multiple, referenced 2D points.
type MultiPointS struct {
	Srid uint32
	Mp   MultiPoint
}

// Points returns the coordinates for the 2D points
func (mps MultiPointS) Points() struct {
	Srid uint32
	Mp   MultiPoint
} {
	return mps
}

// SetSRID modifies the struct containing the SRID int and the array of 2D coordinates
func (mps *MultiPointS) SetSRID(srid uint32, mp MultiPoint) (err error) {
	if mps == nil {
		return ErrNilMultiPointS
	}

	mps.Srid = srid
	mps.Mp = mp
	return
}

// Get the simple 2D multipoint
func (mps MultiPointS) MultiPoint() MultiPoint {
	return mps.Mp
}
