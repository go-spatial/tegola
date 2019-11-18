package clip

import (
	"context"

	"github.com/go-spatial/geom"
)

type dclipper struct{}

func (_ dclipper) Clip(ctx context.Context, geo geom.Geometry, clipbox *geom.Extent) (geom.Geometry, error) {
	return Geometry(ctx, geo, clipbox)
}

var Default dclipper

// Geometry will return the clipped version of the given geometry.
func Geometry(ctx context.Context, geo geom.Geometry, clipbox *geom.Extent) (geom.Geometry, error) {
	if clipbox.IsUniverse() {
		return geo, nil
	}
	if geo == nil {
		return nil, nil
	}

	switch g := geo.(type) {
	case geom.Pointer:
		xy := g.XY()
		if clipbox.ContainsPoint(xy) {
			return geo, nil
		}
		return nil, nil

	case geom.MultiPointer:
		return MultiPointer(ctx, g, clipbox)
	case geom.LineStringer:
		return LineStringer(ctx, g, clipbox)
	case geom.MultiLineStringer:
		return MultiLineStringer(ctx, g, clipbox)
	default:
		return geo, ErrUnsupportedGeometry
	}
}
