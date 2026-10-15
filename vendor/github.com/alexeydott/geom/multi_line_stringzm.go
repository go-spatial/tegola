package geom

import "errors"

// ErrNilMultiLineStringZM is thrown when MultiLineStringZM is nil but shouldn't be
var ErrNilMultiLineStringZM = errors.New("geom: nil MultiLineStringZM")

// MultiLineStringZM is a geometry with multiple LineStringZMs.
type MultiLineStringZM [][][4]float64

// LineStringZMs returns the coordinates for the linestrings
func (mlszm MultiLineStringZM) LineStringZMs() [][][4]float64 {
	return mlszm
}

// SetLineStringZMs modifies the array of 3D+1D coordinates
func (mlszm *MultiLineStringZM) SetLineStringZMs(input [][][4]float64) (err error) {
	if mlszm == nil {
		return ErrNilMultiLineStringZM
	}

	*mlszm = append((*mlszm)[:0], input...)
	return
}

// AsSegments returns the multi lines string as a set of lineZMs.
func (mlszm MultiLineStringZM) AsLineStringZMs() (segs []LineStringZM, err error) {
	if len(mlszm) == 0 {
		return nil, nil
	}
	for i := range mlszm {
		lszm := LineStringZM(mlszm[i])
		if lszm == nil {
			return nil, errors.New("geom: error in splitting MultiLineStringZM")
		}
		segs = append(segs, lszm)
	}
	return segs, nil
}
