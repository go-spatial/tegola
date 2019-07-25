package geom

import "errors"

// ErrNilMultiLineString is thrown when MultiLineString is nil but shouldn't be
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

// AsSegments returns the multi lines string as a set of lines.
func (mls MultiLineString) AsSegments() (segs [][]Line, err error) {
	if len(mls) == 0 {
		return nil, nil
	}
	for i := range mls {
		ls := LineString(mls[i])
		ss, err := ls.AsSegments()
		if err != nil {
			return nil, err
		}
		segs = append(segs, ss)
	}
	return segs, nil
}
