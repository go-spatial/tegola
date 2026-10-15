package geom

import (
	"errors"
)

// ErrNilLineStringZM is thrown when a LineStringZM is nil but shouldn't be
var ErrNilLineStringZM = errors.New("geom: nil LineStringZM")

// ErrInvalidLineStringZM is thrown when a LineStringZM is malformed
var ErrInvalidLineStringZM = errors.New("geom: invalid LineStringZM")

// LineString is a basic line type which is made up of two or more points that don't interacted.
type LineStringZM [][4]float64

// Vertices returns a slice of XYM values
func (lszm LineStringZM) Vertices() [][4]float64 { return lszm }

// SetVertices modifies the array of 3D + 1 coordinates
func (lszm *LineStringZM) SetVertices(input [][4]float64) (err error) {
	if lszm == nil {
		return ErrNilLineStringZM
	}

	*lszm = append((*lszm)[:0], input...)
	return
}

// Get the simple 2D linestring
func (lszm LineStringZM) LineString() LineString {
	var lsv [][2]float64
	var ls LineString

	verts := lszm.Vertices()
	for i := 0; i < len(verts); i++ {
		lsv = append(lsv, [2]float64{verts[i][0], verts[i][1]})
	}

	ls.SetVertices(lsv)
	return ls
}
