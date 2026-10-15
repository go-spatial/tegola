package geom

import "errors"

// ErrNilMultiLineStringZS is thrown when MultiLineStringZS is nil but shouldn't be
var ErrNilMultiLineStringZS = errors.New("geom: nil MultiLineStringZS")

// MultiLineStringZS is a geometry with multiple LineStringSs.
type MultiLineStringZS struct {
	Srid uint32
	Mlsz MultiLineStringZ
}

// LineStrings returns the coordinates for the linestrings
func (mlszs MultiLineStringZS) MultiLineStringZs() struct {
	Srid uint32
	Mlsz MultiLineStringZ
} {
	return mlszs
}

// SetSRID modifies the struct containing the SRID int and the array of 3D coordinates
func (mlszs *MultiLineStringZS) SetSRID(srid uint32, mlsz MultiLineStringZ) (err error) {
	if mlszs == nil {
		return ErrNilMultiLineStringZS
	}

	mlszs.Srid = srid
	mlszs.Mlsz = mlsz
	return
}

// Get the simple 3D multiline string
func (mlszs MultiLineStringZS) MultiLineStringZ() MultiLineStringZ {
	return mlszs.Mlsz
}
