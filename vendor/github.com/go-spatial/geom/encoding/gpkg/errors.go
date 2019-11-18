// +build cgo

package gpkg

import "github.com/gdey/errors"

const (
	ErrEnvelopeEnvelopeTypeMismatch = errors.String("envelope and envelope type mismatch")
	ErrInvalidEnvelopeType          = errors.String("invalid envelope type")
	ErrInsufficentBytes             = errors.String("not enough bytes")
	ErrInvalidMagicNumber           = errors.String("invalid magic number")
	ErrNilStandardBinary            = errors.String("standard binary is nil")
	ErrNilHandler                   = errors.String("gpkg handler is nil")
)
