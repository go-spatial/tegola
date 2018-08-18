package geom

import "fmt"

// ErrUnknownGeometry represents an objects that is not a known geom geometry.
type ErrUnknownGeometry struct {
	Geom Geometry
}

func (e ErrUnknownGeometry) Error() string {
	return fmt.Sprintf("unknown geometry: %T", e.Geom)
}
