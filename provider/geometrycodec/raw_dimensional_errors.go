package geometrycodec

import "errors"

// ErrUnsupportedRawGeometry identifies a raw geometry profile outside the supported feature codec.
var ErrUnsupportedRawGeometry = errors.New("unsupported raw geometry profile")
