package geom

import "errors"

// ErrNilMultiLineStringMS is thrown when MultiLineStringMS is nil but shouldn't be
var ErrNilMultiLineStringMS = errors.New("geom: nil MultiLineStringMS")

// MultiLineStringMS is a geometry with multiple LineStringSs.
type MultiLineStringMS struct {
	Srid uint32
	Mlsm MultiLineStringM
}

// LineStrings returns the coordinates for the linestrings
func (mlsms MultiLineStringMS) MultiLineStringMs() struct {
	Srid uint32
	Mlsm MultiLineStringM
} {
	return mlsms
}

// SetSRID modifies the struct containing the SRID int and the array of 2D+1 coordinates
func (mlsms *MultiLineStringMS) SetSRID(srid uint32, mlsm MultiLineStringM) (err error) {
	if mlsms == nil {
		return ErrNilMultiLineStringMS
	}

	mlsms.Srid = srid
	mlsms.Mlsm = mlsm
	return
}

// Get the simple 2D+1 multiline string
func (mlsms MultiLineStringMS) MultiLineStringM() MultiLineStringM {
	return mlsms.Mlsm
}
