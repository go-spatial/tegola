package geom

import (
	"errors"
)

// ErrNilPolygonMS is thrown when a polygonz is nil but shouldn't be
var ErrNilPolygonMS = errors.New("geom: nil PolygonMS")

// ErrInvalidLinearRingMS is thrown when a LinearRingMS is malformed
var ErrInvalidLinearRingMS = errors.New("geom: invalid LinearRingMS")

// ErrInvalidPolygonMS is thrown when a Polygon is malformed
var ErrInvalidPolygonMS = errors.New("geom: invalid PolygonMS")

// PolygonMS is a geometry consisting of multiple closed LineStringMSs.
// There must be only one exterior LineStringMS with a clockwise winding order.
// There may be one or more interior LineStringMSs with a counterclockwise winding orders.
// The last point in the linear ring will not match the first point.
type PolygonMS struct {
	Srid uint32
	Polm PolygonM
}

// LinearRings returns the coordinates of the linear rings
func (p PolygonMS) LinearRings() struct {
	Srid uint32
	Polm PolygonM
} {
	return p
}

// SetLinearRingMs modifies the array of 3D coordinates
func (p *PolygonMS) SetLinearRings(srid uint32, polm PolygonM) (err error) {
	if p == nil {
		return ErrNilPolygonMS
	}

	p.Srid = srid
	p.Polm = polm
	return
}

// AsSegments returns the polygon as a slice of lines. This will make no attempt to only add unique segments.
func (p PolygonMS) AsSegments() (segs [][]LineM, srid uint32, err error) {

	if len(p.Polm) == 0 {
		return nil, 0, nil
	}

	segs = make([][]LineM, 0, len(p.Polm))
	for i := range p.Polm {
		switch len(p.Polm[i]) {
		case 0, 1, 2:
			continue
			// TODO(gdey) : why are we getting invalid points.
			/*
				case 1, 2:
					return nil, ErrInvalidLinearRing
			*/

		default:
			pilen := len(p.Polm[i])
			subr := make([]LineM, pilen)
			pj := pilen - 1
			for j := 0; j < pilen; j++ {
				subr[j] = LineM{p.Polm[i][pj], p.Polm[i][j]}
				pj = j
			}
			segs = append(segs, subr)
		}
	}
	return segs, p.Srid, nil
}
