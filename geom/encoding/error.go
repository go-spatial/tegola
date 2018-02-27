package encoding

import (
	"fmt"

	"github.com/go-spatial/tegola/geom"
)

type ErrUnknownGeometry struct {
	Geom geom.Geometry
}

func (e ErrUnknownGeometry) Error() string {
	return fmt.Sprintf("unknown geometry: %T", e.Geom)
}
