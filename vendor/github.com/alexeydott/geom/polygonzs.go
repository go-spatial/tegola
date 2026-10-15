package geom

import (
	"errors"
)

// ErrNilPolygonZS is thrown when a polygonz is nil but shouldn't be
var ErrNilPolygonZS = errors.New("geom: nil PolygonZS")

// ErrInvalidLinearRingZS is thrown when a LinearRingZS is malformed
var ErrInvalidLinearRingZS = errors.New("geom: invalid LinearRingZS")

// ErrInvalidPolygonZS is thrown when a Polygon is malformed
var ErrInvalidPolygonZS = errors.New("geom: invalid PolygonZS")

// PolygonZS is a geometry consisting of multiple closed LineStringZSs.
// There must be only one exterior LineStringZS with a clockwise winding order.
// There may be one or more interior LineStringZSs with a counterclockwise winding orders.
// The last point in the linear ring will not match the first point.
type PolygonZS struct {
	Srid uint32
	Polz PolygonZ
}

// LinearRings returns the coordinates of the linear rings
func (p PolygonZS) LinearRings() struct {
	Srid uint32
	Polz PolygonZ
} {
	return p
}

// SetLinearRingZs modifies the array of 3D coordinates
func (p *PolygonZS) SetLinearRings(srid uint32, polz PolygonZ) (err error) {
	if p == nil {
		return ErrNilPolygonZS
	}

	p.Srid = srid
	p.Polz = polz
	return
}

// AsSegments returns the polygon as a slice of lines. This will make no attempt to only add unique segments.
func (p PolygonZS) AsSegments() (segs [][]LineZ, srid uint32, err error) {

	if len(p.Polz) == 0 {
		return nil, 0, nil
	}

	segs = make([][]LineZ, 0, len(p.Polz))
	for i := range p.Polz {
		switch len(p.Polz[i]) {
		case 0, 1, 2:
			continue
			// TODO(gdey) : why are we getting invalid points.
			/*
				case 1, 2:
					return nil, ErrInvalidLinearRing
			*/

		default:
			pilen := len(p.Polz[i])
			subr := make([]LineZ, pilen)
			pj := pilen - 1
			for j := 0; j < pilen; j++ {
				subr[j] = LineZ{p.Polz[i][pj], p.Polz[i][j]}
				pj = j
			}
			segs = append(segs, subr)
		}
	}
	return segs, p.Srid, nil
}
