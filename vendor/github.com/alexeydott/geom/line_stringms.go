package geom

import (
	"errors"
)

// ErrNilLineStringMS is thrown when a LineStringS is nil but shouldn't be
var ErrNilLineStringMS = errors.New("geom: nil LineStringMS")

// ErrInvalidLineStringMS is thrown when a LineStringMS is malformed
var ErrInvalidLineStringMS = errors.New("geom: invalid LineStringMS")

// LineStringMS is a basic line type which is made up of two or more points that don't interacted.
type LineStringMS struct {
	Srid uint32
	Lsm  LineStringM
}

// Vertices returns a slice of referenced XYM values
func (lsms LineStringMS) Vertices() struct {
	Srid uint32
	Lsm  LineStringM
} {
	return lsms
}

// SetVertices modifies the struct containing the SRID int and the array of 2D + 1 coordinates
func (lsms *LineStringMS) SetSRID(srid uint32, lsm LineStringM) (err error) {
	if lsms == nil {
		return ErrNilLineStringMS
	}

	lsms.Srid = srid
	lsms.Lsm = lsm
	return
}

// Get the simple 2D + 1 linestring
func (lsms LineStringMS) LineStringM() LineStringM {
	return lsms.Lsm
}
