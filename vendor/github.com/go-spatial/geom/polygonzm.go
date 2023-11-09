package geom

import (
	"errors"
)

// ErrNilPolygonZM is thrown when a polygonz is nil but shouldn't be
var ErrNilPolygonZM = errors.New("geom: nil PolygonZM")

// ErrInvalidLinearRingZM is thrown when a LinearRingZM is malformed
var ErrInvalidLinearRingZM = errors.New("geom: invalid LinearRingZM")

// ErrInvalidPolygonZM is thrown when a Polygon is malformed
var ErrInvalidPolygonZM = errors.New("geom: invalid PolygonZM")

// PolygonZM is a geometry consisting of multiple closed LineStringZMs.
// There must be only one exterior LineStringZM with a clockwise winding order.
// There may be one or more interior LineStringZMs with a counterclockwise winding orders.
// The last point in the linear ring will not match the first point.
type PolygonZM [][][4]float64

// LinearRings returns the coordinates of the linear rings
func (p PolygonZM) LinearRings() [][][4]float64 {
	return p
}

// SetLinearRingZs modifies the array of 3D coordinates
func (p *PolygonZM) SetLinearRings(input [][][4]float64) (err error) {
	if p == nil {
		return ErrNilPolygonZM
	}

	*p = append((*p)[:0], input...)
	return
}

// AsSegments returns the polygon as a slice of lines. This will make no attempt to only add unique segments.
func (p PolygonZM) AsSegments() (segs [][]LineZM, err error) {

	if len(p) == 0 {
		return nil, nil
	}

	segs = make([][]LineZM, 0, len(p))
	for i := range p {
		switch len(p[i]) {
		case 0, 1, 2:
			continue
			// TODO(gdey) : why are we getting invalid points.
			/*
				case 1, 2:
					return nil, ErrInvalidLinearRing
			*/

		default:
			pilen := len(p[i])
			subr := make([]LineZM, pilen)
			pj := pilen - 1
			for j := 0; j < pilen; j++ {
				subr[j] = LineZM{p[i][pj], p[i][j]}
				pj = j
			}
			segs = append(segs, subr)
		}
	}
	return segs, nil
}
