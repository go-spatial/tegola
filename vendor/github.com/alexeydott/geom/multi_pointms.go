package geom

import "errors"

// ErrNilMultiPointMS is thrown when a MultiPointMS is nil but shouldn't be
var ErrNilMultiPointMS = errors.New("geom: nil MultiPointMS")

// MultiPointMS is a geometry with multiple, referenced 2+1D points.
type MultiPointMS struct {
	Srid uint32
	Mpm  MultiPointM
}

// Points returns the coordinates for the 3D points
func (mpms MultiPointMS) Points() struct {
	Srid uint32
	Mpm  MultiPointM
} {
	return mpms
}

// SetSRID modifies the struct containing the SRID int and the array of 3D coordinates
func (mpms *MultiPointMS) SetSRID(srid uint32, mpm MultiPointM) (err error) {
	if mpms == nil {
		return ErrNilMultiPointMS
	}

	mpms.Srid = srid
	mpms.Mpm = mpm
	return
}

// Get the simple 3D multipoint
func (mpms MultiPointMS) MultiPointM() MultiPointM {
	return mpms.Mpm
}
