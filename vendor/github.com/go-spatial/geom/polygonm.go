package geom

import (
	"errors"
)

// ErrNilPolygonM is thrown when a polygonz is nil but shouldn't be
var ErrNilPolygonM = errors.New("geom: nil PolygonM")

// ErrInvalidLinearRingM is thrown when a LinearRingM is malformed
var ErrInvalidLinearRingM = errors.New("geom: invalid LinearRingM")

// ErrInvalidPolygonM is thrown when a Polygon is malformed
var ErrInvalidPolygonM = errors.New("geom: invalid PolygonM")

// PolygonM is a geometry consisting of multiple closed LineStringMs.
// There must be only one exterior LineStringM with a clockwise winding order.
// There may be one or more interior LineStringMs with a counterclockwise winding orders.
// The last point in the linear ring will not match the first point.
type PolygonM [][][3]float64

// LinearRings returns the coordinates of the linear rings
func (p PolygonM) LinearRings() [][][3]float64 {
	return p
}

// SetLinearRingZs modifies the array of 2D+1 coordinates
func (p *PolygonM) SetLinearRings(input [][][3]float64) (err error) {
	if p == nil {
		return ErrNilPolygonM
	}

	*p = append((*p)[:0], input...)
	return
}

// AsSegments returns the polygon as a slice of lines. This will make no attempt to only add unique segments.
func (p PolygonM) AsSegments() (segs [][]LineM, err error) {

	if len(p) == 0 {
		return nil, nil
	}

	segs = make([][]LineM, 0, len(p))
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
			subr := make([]LineM, pilen)
			pj := pilen - 1
			for j := 0; j < pilen; j++ {
				subr[j] = LineM{p[i][pj], p[i][j]}
				pj = j
			}
			segs = append(segs, subr)
		}
	}
	return segs, nil
}
