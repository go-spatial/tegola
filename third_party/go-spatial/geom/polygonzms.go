package geom

import (
	"errors"
)

// ErrNilPolygonZMS is thrown when a polygonz is nil but shouldn't be
var ErrNilPolygonZMS = errors.New("geom: nil PolygonZMS")

// ErrInvalidLinearRingZMS is thrown when a LinearRingZMS is malformed
var ErrInvalidLinearRingZMS = errors.New("geom: invalid LinearRingZMS")

// ErrInvalidPolygonZMS is thrown when a Polygon is malformed
var ErrInvalidPolygonZMS = errors.New("geom: invalid PolygonZMS")

// PolygonZMS is a geometry consisting of multiple closed LineStringZMSs.
// There must be only one exterior LineStringZMS with a clockwise winding order.
// There may be one or more interior LineStringZMSs with a counterclockwise winding orders.
// The last point in the linear ring will not match the first point.
type PolygonZMS struct {
	Srid  uint32
	Polzm PolygonZM
}

// LinearRings returns the coordinates of the linear rings
func (p PolygonZMS) LinearRings() struct {
	Srid  uint32
	Polzm PolygonZM
} {
	return p
}

// SetLinearRingMs modifies the array of 3D coordinates
func (p *PolygonZMS) SetLinearRings(srid uint32, polzm PolygonZM) (err error) {
	if p == nil {
		return ErrNilPolygonZMS
	}

	p.Srid = srid
	p.Polzm = polzm
	return
}

// AsSegments returns the polygon as a slice of lines. This will make no attempt to only add unique segments.
func (p PolygonZMS) AsSegments() (segs [][]LineZM, srid uint32, err error) {

	if len(p.Polzm) == 0 {
		return nil, 0, nil
	}

	segs = make([][]LineZM, 0, len(p.Polzm))
	for i := range p.Polzm {
		switch len(p.Polzm[i]) {
		case 0, 1, 2:
			continue
			// TODO(gdey) : why are we getting invalid points.
			/*
				case 1, 2:
					return nil, ErrInvalidLinearRing
			*/

		default:
			pilen := len(p.Polzm[i])
			subr := make([]LineZM, pilen)
			pj := pilen - 1
			for j := 0; j < pilen; j++ {
				subr[j] = LineZM{p.Polzm[i][pj], p.Polzm[i][j]}
				pj = j
			}
			segs = append(segs, subr)
		}
	}
	return segs, p.Srid, nil
}
