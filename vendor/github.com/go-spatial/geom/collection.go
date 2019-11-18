package geom

import "errors"

// ErrNilCollection is thrown when a collection is nil but shouldn't be
var ErrNilCollection = errors.New("geom: nil collection")

// Collection is a collection of one or more geometries.
type Collection []Geometry

// Geometries returns the slice of Geometries
func (c Collection) Geometries() []Geometry {
	return c
}

// SetGeometries modifies the array of 2D coordinates
func (c *Collection) SetGeometries(input []Geometry) (err error) {
	if c == nil {
		return ErrNilCollection
	}

	*c = append((*c)[:0], input...)
	return
}
