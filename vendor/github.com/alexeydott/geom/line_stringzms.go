package geom

import (
	"errors"
)

// ErrNilLineStringZMS is thrown when a LineStringZMS is nil but shouldn't be
var ErrNilLineStringZMS = errors.New("geom: nil LineStringZMS")

// ErrInvalidLineStringZMS is thrown when a LineStringZMS is malformed
var ErrInvalidLineStringZMS = errors.New("geom: invalid LineStringZMS")

// LineStringZMS is a basic line type which is made up of two or more points that don't interacted.
type LineStringZMS struct {
	Srid uint32
	Lszm LineStringZM
}

// Vertices returns a slice of referenced XYZM values
func (lszms LineStringZMS) Vertices() struct {
	Srid uint32
	Lszm LineStringZM
} {
	return lszms
}

// SetVertices modifies the struct containing the SRID int and the array of 3D + 1 coordinates
func (lszms *LineStringZMS) SetSRID(srid uint32, lszm LineStringZM) (err error) {
	if lszms == nil {
		return ErrNilLineStringZMS
	}

	lszms.Srid = srid
	lszms.Lszm = lszm
	return
}

// Get the simple 3D + 1 linestring
func (lszms LineStringZMS) LineStringZM() LineStringZM {
	return lszms.Lszm
}
