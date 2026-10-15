package geom

import (
	"errors"
)

// ErrNilLineStringM is thrown when a LineStringM is nil but shouldn't be
var ErrNilLineStringM = errors.New("geom: nil LineStringM")

// ErrInvalidLineStringM is thrown when a LineStringM is malformed
var ErrInvalidLineStringM = errors.New("geom: invalid LineStringM")

// LineString is a basic line type which is made up of two or more points that don't interacted.
type LineStringM [][3]float64

// Vertices returns a slice of XYM values
func (lsm LineStringM) Vertices() [][3]float64 { return lsm }

// SetVertices modifies the array of 2D+1 coordinates
func (lsm *LineStringM) SetVertices(input [][3]float64) (err error) {
	if lsm == nil {
		return ErrNilLineStringM
	}

	*lsm = append((*lsm)[:0], input...)
	return
}

// Get the simple 2D linestring
func (lsm LineStringM) LineString() LineString {
	var lsv [][2]float64
	var ls LineString

	verts := lsm.Vertices()
	for i := 0; i < len(verts); i++ {
		lsv = append(lsv, [2]float64{verts[i][0], verts[i][1]})
	}

	ls.SetVertices(lsv)
	return ls
}
