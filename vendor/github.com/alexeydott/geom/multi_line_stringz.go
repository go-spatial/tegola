package geom

import "errors"

// ErrNilMultiLineStringZ is thrown when MultiLineStringZ is nil but shouldn't be
var ErrNilMultiLineStringZ = errors.New("geom: nil MultiLineStringZ")

// MultiLineStringZ is a geometry with multiple LineStringZs.
type MultiLineStringZ [][][3]float64

// LineStringZs returns the coordinates for the linestrings
func (mlsz MultiLineStringZ) LineStringZs() [][][3]float64 {
	return mlsz
}

// SetLineStringZs modifies the array of 3D coordinates
func (mlsz *MultiLineStringZ) SetLineStringZs(input [][][3]float64) (err error) {
	if mlsz == nil {
		return ErrNilMultiLineStringZ
	}

	*mlsz = append((*mlsz)[:0], input...)
	return
}

// AsSegments returns the multi lines string as a set of lineZs.
func (mlsz MultiLineStringZ) AsLineStringZs() (segs []LineStringZ, err error) {
	if len(mlsz) == 0 {
		return nil, nil
	}
	for i := range mlsz {
		lsz := LineStringZ(mlsz[i])
		if lsz == nil {
			return nil, errors.New("geom: error in splitting MultiLineStringZ")
		}
		segs = append(segs, lsz)
	}
	return segs, nil
}
