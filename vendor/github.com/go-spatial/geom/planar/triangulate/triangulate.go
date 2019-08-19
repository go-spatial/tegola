package triangulate

import (
	"context"

	"github.com/go-spatial/geom"
)

type nodata struct{}

// EmptyMetadata can be used to indicate that this point or line does
// not have any metadata associated with it.
var EmptyMetadata = nodata{}

// Triangulator describes an object that can take a set of points and produce
// a triangulation.
type Triangulator interface {

	// SetPoints sets the nodes to be used in the triangulations
	// the number of data elements should be equal to or less then
	// the number of the points, where the index of the data maps
	// to the point. The EmptyMetadata value can be used to indicate no metadata
	// for that point.
	SetPoints(ctx context.Context, pts []geom.Point, data []interface{})

	// Triangles returns the triangles that were produced by the triangulation
	// If the triangulation uses a frame the includeFrame should be used to
	// determine if the triangles touching the frame should be included or not.
	Triangles(ctx context.Context, includeFrame bool) []geom.Triangle
}

// Constrainer is a Triangulator that can take set of points and ensure that the
// given set of edges (the constraints) exist in the triangulation.
type Constrainer interface {
	Triangulator

	// AddConstraints adds constraint lines to the triangulation, this may require
	// the triangulation to be recalculated.
	// Data is any matadata to be attached to each constraint. The number of data
	// elements must be less then the number of lines. The EmptyMetadata value can be used to
	// indicate no metadata for that constraint.
	AddConstraint(ctx context.Context, constraints []geom.Line, data []interface{}) error
}
