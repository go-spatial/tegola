package geom

import (
	"errors"
)

// ErrNilLineStringZ is thrown when a LineStringZ is nil but shouldn't be
var ErrNilLineStringZ = errors.New("geom: nil LineStringZ")

// ErrInvalidLineStringZ is thrown when a LineStringZ is malformed
var ErrInvalidLineStringZ = errors.New("geom: invalid LineStringZ")

// LineString is a basic line type which is made up of two or more points that don't interacted.
type LineStringZ [][3]float64

// Vertices returns a slice of XYM values
func (lsz LineStringZ) Vertices() [][3]float64 { return lsz }

// SetVertices modifies the array of 3D coordinates
func (lsz *LineStringZ) SetVertices(input [][3]float64) (err error) {
	if lsz == nil {
		return ErrNilLineStringZ
	}

	*lsz = append((*lsz)[:0], input...)
	return
}

// Get the simple 2D linestring
func (lsz LineStringZ) LineString() LineString {
	var lsv [][2]float64
	var ls LineString

	verts := lsz.Vertices()
	for i := 0; i < len(verts); i++ {
		lsv = append(lsv, [2]float64{verts[i][0], verts[i][1]})
	}

	ls.SetVertices(lsv)
	return ls
}
