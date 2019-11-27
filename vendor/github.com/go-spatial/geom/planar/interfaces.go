package planar

import (
	"context"

	"github.com/go-spatial/geom"
)

// Label is the the label for the triangle. Is in "inside" or "outside".
// TODO: gdey — would be make more sense to just have a bool here? IsInside or somthing like that?
type Label uint8

func (l Label) String() string {
	switch l {
	case Outside:
		return "outside"
	case Inside:
		return "inside"
	default:
		return "unknown"
	}
}

const (
	// Unknown is the default if it cannot be determined in/outside
	Unknown Label = iota
	Outside
	Inside
)

type HitMapper interface {
	LabelFor(pt [2]float64) Label
	Extent() [4]float64
	Area() float64
}

type MakeValider interface {
	// Makevalid will take a possibility invalid geometry and an optional clipbox, returning a valid geometry, weather it clipped the geometry, or an error if one occured.
	Makevalid(ctx context.Context, geo geom.Geometry, clipbox *geom.Extent) (geometry geom.Geometry, didClip bool, err error)
}

type Clipper interface {
	// Clip will take a valid geometry and a clipbox, returning a clipped version of the geometry to the clipbox, or an error
	Clip(ctx context.Context, geo geom.Geometry, clipbox *geom.Extent) (geometry geom.Geometry, err error)
}

// Simplifer is an interface for Simplifying geometries.
type Simplifer interface {
	Simplify(ctx context.Context, linestring [][2]float64, isClosed bool) ([][2]float64, error)
}
