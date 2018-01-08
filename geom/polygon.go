package geom

import "errors"

var ErrNilPolygon = errors.New("geom: nil Polygon")

// Polygon is a geometry consisting of multiple closed LineStrings.
// There must be only one exterior LineString with a clockwise winding order.
// There may be one or more interior LineStrings with a counterclockwise winding orders.
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
