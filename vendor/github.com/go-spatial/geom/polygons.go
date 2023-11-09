package geom

import (
	"errors"
)

// ErrNilPolygonS is thrown when a polygonz is nil but shouldn't be
var ErrNilPolygonS = errors.New("geom: nil PolygonS")

// ErrInvalidLinearRingS is thrown when a LinearRingS is malformed
var ErrInvalidLinearRingS = errors.New("geom: invalid LinearRingS")

// ErrInvalidPolygonS is thrown when a Polygon is malformed
var ErrInvalidPolygonS = errors.New("geom: invalid PolygonS")

// PolygonS is a geometry consisting of multiple closed LineStringSs.
// There must be only one exterior LineStringS with a clockwise winding order.
// There may be one or more interior LineStringSs with a counterclockwise winding orders.
// The last point in the linear ring will not match the first point.
type PolygonS struct {
	Srid uint32
	Pol  Polygon
}

// LinearRings returns the coordinates of the linear rings
func (p PolygonS) LinearRings() struct {
	Srid uint32
	Pol  Polygon
} {
	return p
}

// SetLinearRingZs modifies the array of 3D coordinates
func (p *PolygonS) SetLinearRings(srid uint32, pol Polygon) (err error) {
	if p == nil {
		return ErrNilPolygonS
	}

	p.Srid = srid
	p.Pol = pol
	return
}

// AsSegments returns the polygon as a slice of lines. This will make no attempt to only add unique segments.
func (p PolygonS) AsSegments() (segs [][]Line, srid uint32, err error) {

	if len(p.Pol) == 0 {
		return nil, 0, nil
	}

	segs = make([][]Line, 0, len(p.Pol))
	for i := range p.Pol {
		switch len(p.Pol[i]) {
		case 0, 1, 2:
			continue
			// TODO(gdey) : why are we getting invalid points.
			/*
				case 1, 2:
					return nil, ErrInvalidLinearRing
			*/

		default:
			pilen := len(p.Pol[i])
			subr := make([]Line, pilen)
			pj := pilen - 1
			for j := 0; j < pilen; j++ {
				subr[j] = Line{p.Pol[i][pj], p.Pol[i][j]}
				pj = j
			}
			segs = append(segs, subr)
		}
	}
	return segs, p.Srid, nil
}
