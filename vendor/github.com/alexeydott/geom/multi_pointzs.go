package geom

import "errors"

// ErrNilMultiPointZS is thrown when a MultiPointZS is nil but shouldn't be
var ErrNilMultiPointZS = errors.New("geom: nil MultiPointZS")

// MultiPointZS is a geometry with multiple, referenced 3D points.
type MultiPointZS struct {
	Srid uint32
	Mpz  MultiPointZ
}

// Points returns the coordinates for the 3D points
func (mpzs MultiPointZS) Points() struct {
	Srid uint32
	Mpz  MultiPointZ
} {
	return mpzs
}

// SetSRID modifies the struct containing the SRID int and the array of 3D coordinates
func (mpzs *MultiPointZS) SetSRID(srid uint32, mpz MultiPointZ) (err error) {
	if mpzs == nil {
		return ErrNilMultiPointZS
	}

	mpzs.Srid = srid
	mpzs.Mpz = mpz
	return
}

// Get the simple 3D multipoint
func (mps MultiPointZS) MultiPointZ() MultiPointZ {
	return mps.Mpz
}
