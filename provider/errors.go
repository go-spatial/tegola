package provider

import (
	"errors"
	"fmt"
)

var (
	ErrCanceled    = errors.New("provider: canceled")
	ErrUnsupported = errors.New("provider: unsupported")
)

type ErrUnableToConvertFeatureID struct {
	val interface{}
}

func (e ErrUnableToConvertFeatureID) Error() string {
	return fmt.Sprintf("unable to convert feature id %+v to uint64", e.val)
}
