package geom

import (
	"errors"
)

// ErrNilPolygon is thrown when a polygon is nil but shouldn't be
var ErrNilPolygon = errors.New("geom: nil Polygon")

// ErrInvalidLinearRing is thrown when a LinearRing is malformed
var ErrInvalidLinearRing = errors.New("geom: invalid LinearRing")

// ErrInvalidPolygon is thrown when a Polygon is malformed
var ErrInvalidPolygon = errors.New("geom: invalid Polygon")

// Polygon is a geometry consisting of multiple closed LineStrings.
// There must be only one exterior LineString with a clockwise winding order.
// There may be one or more interior LineStrings with a counterclockwise winding orders.
// The last point in the polygon will not match the first point.
type Polygon [][][2]float64

// LinearRings returns the coordinates of the linear rings
func (p Polygon) LinearRings() [][][2]float64 {
	return p
}

// SetLinearRings modifies the array of 2D coordinates
func (p *Polygon) SetLinearRings(input [][][2]float64) (err error) {
	if p == nil {
		return ErrNilPolygon
	}

	*p = append((*p)[:0], input...)
	return
}

// AsSegments returns the polygon as a slice of lines. This will make no attempt to only add unique segments.
func (p Polygon) AsSegments() (segs [][]Line, err error) {

	if len(p) == 0 {
		return nil, nil
	}

	segs = make([][]Line, 0, len(p))
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
			subr := make([]Line, pilen)
			pj := pilen - 1
			for j := 0; j < pilen; j++ {
				subr[j] = Line{p[i][pj], p[i][j]}
				pj = j
			}
			segs = append(segs, subr)
		}
	}
	return segs, nil
}
