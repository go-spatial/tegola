package geom

import (
	"errors"
)

// ErrNilLineString is thrown when a LineString is nil but shouldn't be
var ErrNilLineString = errors.New("geom: nil LineString")

// ErrInvalidLineString is thrown when a LineString is malformed
var ErrInvalidLineString = errors.New("geom: invalid LineString")

// LineString is a basic line type which is made up of two or more points that don't interacted.
type LineString [][2]float64

/* We need to change this to a WalkPoints function.
// Points returns a slice of XY values
func (ls LineString) Points() [][2]float64 {
	return ls
}
*/

// IsRing returns true if the first and last vertices are the same
func (ls LineString) IsRing() bool {
	last := len(ls) - 1
	if len(ls) > 1 && ls[0][0] == ls[last][0] && ls[0][1] == ls[last][1] {
		return true
	}
	return false
}

// Vertices returns a slice of XY values
func (ls LineString) Vertices() [][2]float64 { return ls }

// SetVertices modifies the array of 2D coordinates
func (ls *LineString) SetVertices(input [][2]float64) (err error) {
	if ls == nil {
		return ErrNilLineString
	}

	*ls = append((*ls)[:0], input...)
	return
}

// AsSegments returns the line string as a slice of lines.
func (ls LineString) AsSegments() (segs []Line, err error) {
	switch len(ls) {
	case 0:
		return nil, nil
	case 1:
		return nil, ErrInvalidLineString
	case 2:
		return []Line{{ls[0], ls[1]}}, nil
	default:
		segs = make([]Line, len(ls)-1)
		for i := 0; i < len(ls)-1; i++ {
			segs[i] = Line{ls[i], ls[i+1]}
		}
		return segs, nil
	}
}
