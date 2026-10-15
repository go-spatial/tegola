package geom

import (
	"errors"
)

// ErrNilLineStringZS is thrown when a LineStringS is nil but shouldn't be
var ErrNilLineStringZS = errors.New("geom: nil LineStringZS")

// ErrInvalidLineStringZS is thrown when a LineStringZS is malformed
var ErrInvalidLineStringZS = errors.New("geom: invalid LineStringZS")

// LineStringZS is a basic line type which is made up of two or more points that don't interacted.
type LineStringZS struct {
	Srid uint32
	Lsz  LineStringZ
}

// Vertices returns a slice of referenced XYM values
func (lszs LineStringZS) Vertices() struct {
	Srid uint32
	Lsz  LineStringZ
} {
	return lszs
}

// SetVertices modifies the struct containing the SRID int and the array of 3D coordinates
func (lszs *LineStringZS) SetSRID(srid uint32, lsz LineStringZ) (err error) {
	if lszs == nil {
		return ErrNilLineStringZS
	}

	lszs.Srid = srid
	lszs.Lsz = lsz
	return
}

// Get the simple 3D linestring
func (lszs LineStringZS) LineStringZ() LineStringZ {
	return lszs.Lsz
}
