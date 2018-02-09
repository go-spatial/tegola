package geom

import "errors"

var ErrNilMultiLineString = errors.New("geom: nil MultiLineString")

// MultiLineString is a geometry with multiple LineStrings.
type MultiLineString [][][2]float64

// LineStrings returns the coordinates for the linestrings
func (mls MultiLineString) LineStrings() [][][2]float64 {
	return mls
}

// SetLineStrings modifies the array of 2D coordinates
func (mls *MultiLineString) SetLineStrings(input [][][2]float64) (err error) {
	if mls == nil {
		return ErrNilMultiLineString
	}

	*mls = append((*mls)[:0], input...)
	return
}
