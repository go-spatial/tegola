package geom

import "errors"

// ErrNilMultiLineStringM is thrown when MultiLineStringM is nil but shouldn't be
var ErrNilMultiLineStringM = errors.New("geom: nil MultiLineStringM")

// MultiLineStringM is a geometry with multiple LineStringMs.
type MultiLineStringM [][][3]float64

// LineStringMs returns the coordinates for the linestrings
func (mlsm MultiLineStringM) LineStringMs() [][][3]float64 {
	return mlsm
}

// SetLineStringZs modifies the array of 2D+1D coordinates
func (mlsm *MultiLineStringM) SetLineStringMs(input [][][3]float64) (err error) {
	if mlsm == nil {
		return ErrNilMultiLineStringM
	}

	*mlsm = append((*mlsm)[:0], input...)
	return
}

// AsSegments returns the multi lines string as a set of lineMs.
func (mlsm MultiLineStringM) AsLineStringMs() (segs []LineStringM, err error) {
	if len(mlsm) == 0 {
		return nil, nil
	}
	for i := range mlsm {
		lsm := LineStringM(mlsm[i])
		if lsm == nil {
			return nil, errors.New("geom: error in splitting MultiLineStringM")
		}
		segs = append(segs, lsm)
	}
	return segs, nil
}
