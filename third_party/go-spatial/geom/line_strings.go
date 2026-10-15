package geom

import (
	"errors"
)

// ErrNilLineStringS is thrown when a LineStringS is nil but shouldn't be
var ErrNilLineStringS = errors.New("geom: nil LineStringS")

// ErrInvalidLineStringS is thrown when a LineStringS is malformed
var ErrInvalidLineStringS = errors.New("geom: invalid LineStringS")

// LineString is a basic line type which is made up of two or more points that don't interacted.
type LineStringS struct {
	Srid uint32
	Ls   LineString
}

// Vertices returns a slice of referenced XY values
func (lss LineStringS) Vertices() struct {
	Srid uint32
	Ls   LineString
} {
	return lss
}

// SetVertices modifies the struct containing the SRID int and the array of 2D coordinates
func (lss *LineStringS) SetSRID(srid uint32, ls LineString) (err error) {
	if lss == nil {
		return ErrNilLineStringS
	}

	lss.Srid = srid
	lss.Ls = ls
	return
}

// Get the simple 2D linestring
func (lss LineStringS) LineString() LineString {
	return lss.Ls
}
