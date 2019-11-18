package subdivision

import (
	"github.com/gdey/errors"
)

const (

	// ErrInvalidStartingVertex is returned when the starting vertex is invalid
	ErrInvalidStartingVertex = errors.String("invalid starting vertex")

	// ErrInvalidEndVertex is returned when the ending vertex is invalid
	ErrInvalidEndVertex = errors.String("invalid ending vertex")

	// ErrCancelled is returned when the activity is cancelled
	ErrCancelled = errors.String("cancelled")

	// ErrCoincidentalEdges is returned when two edges are conincidental and not expected to be
	ErrCoincidentalEdges = errors.String("coincident edges")

	// ErrDidNotFindToFrom is returned when one of the endpoint of an edge is not in the graph,
	// and is expected to be
	ErrDidNotFindToFrom = errors.String("did not find to and from edge")
)
