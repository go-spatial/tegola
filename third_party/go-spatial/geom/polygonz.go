package geom

import (
	"errors"
)

// ErrNilPolygonZ is thrown when a polygonz is nil but shouldn't be
var ErrNilPolygonZ = errors.New("geom: nil PolygonZ")

// ErrInvalidLinearRingZ is thrown when a LinearRingZ is malformed
var ErrInvalidLinearRingZ = errors.New("geom: invalid LinearRingZ")

// ErrInvalidPolygonZ is thrown when a Polygon is malformed
var ErrInvalidPolygonZ = errors.New("geom: invalid PolygonZ")

// PolygonZ is a geometry consisting of multiple closed LineStringZs.
// There must be only one exterior LineStringZ with a clockwise winding order.
// There may be one or more interior LineStringZs with a counterclockwise winding orders.
// The last point in the linear ring will not match the first point.
type PolygonZ [][][3]float64

// LinearRings returns the coordinates of the linear rings
func (p PolygonZ) LinearRings() [][][3]float64 {
	return p
}

// SetLinearRingZs modifies the array of 3D coordinates
func (p *PolygonZ) SetLinearRings(input [][][3]float64) (err error) {
	if p == nil {
		return ErrNilPolygonZ
	}

	*p = append((*p)[:0], input...)
	return
}

// AsSegments returns the polygon as a slice of lines. This will make no attempt to only add unique segments.
func (p PolygonZ) AsSegments() (segs [][]LineZ, err error) {

	if len(p) == 0 {
		return nil, nil
	}

	segs = make([][]LineZ, 0, len(p))
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
			subr := make([]LineZ, pilen)
			pj := pilen - 1
			for j := 0; j < pilen; j++ {
				subr[j] = LineZ{p[i][pj], p[i][j]}
				pj = j
			}
			segs = append(segs, subr)
		}
	}
	return segs, nil
}
