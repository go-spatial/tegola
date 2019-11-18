package pseudopolygon

import "github.com/gdey/errors"

const (
	// ErrInvalidPseudoPolygonSize is returned when an invalid polygon
	ErrInvalidPseudoPolygonSize = errors.String("invalid polygon, not enough points")
	// ErrAllPointsColinear is returned when more the 2 points are given and they all line on a line
	ErrAllPointsColinear = errors.String("all points are colinear")
)

