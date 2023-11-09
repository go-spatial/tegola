package geom

import "errors"

// ErrNilMultiLineStringS is thrown when MultiLineStringS is nil but shouldn't be
var ErrNilMultiLineStringS = errors.New("geom: nil MultiLineStringS")

// MultiLineStringS is a geometry with multiple LineStringSs.
type MultiLineStringS struct {
	Srid uint32
	Mls  MultiLineString
}

// LineStrings returns the coordinates for the linestrings
func (mlss MultiLineStringS) MultiLineStrings() struct {
	Srid uint32
	Mls  MultiLineString
} {
	return mlss
}

// SetSRID modifies the struct containing the SRID int and the array of 2D coordinates
func (mlss *MultiLineStringS) SetSRID(srid uint32, mls MultiLineString) (err error) {
	if mlss == nil {
		return ErrNilMultiLineStringS
	}

	mlss.Srid = srid
	mlss.Mls = mls
	return
}

// Get the simple 2D multiline string
func (mlss MultiLineStringS) MultiLineString() MultiLineString {
	return mlss.Mls
}
