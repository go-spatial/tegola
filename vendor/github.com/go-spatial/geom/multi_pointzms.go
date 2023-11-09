package geom

import "errors"

// ErrNilMultiPointZMS is thrown when a MultiPointZMS is nil but shouldn't be
var ErrNilMultiPointZMS = errors.New("geom: nil MultiPointZMS")

// MultiPointZMS is a geometry with multiple, referenced 3+1D points.
type MultiPointZMS struct {
	Srid uint32
	Mpzm MultiPointZM
}

// Points returns the coordinates for the 3+1D points
func (mpzms MultiPointZMS) Points() struct {
	Srid uint32
	Mpzm MultiPointZM
} {
	return mpzms
}

// SetSRID modifies the struct containing the SRID int and the array of 3+1D coordinates
func (mpzms *MultiPointZMS) SetSRID(srid uint32, mpzm MultiPointZM) (err error) {
	if mpzms == nil {
		return ErrNilMultiPointZMS
	}

	mpzms.Srid = srid
	mpzms.Mpzm = mpzm
	return
}

// Get the simple 3D multipoint
func (mpzms MultiPointZMS) MultiPointZM() MultiPointZM {
	return mpzms.Mpzm
}
