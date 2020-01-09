package provider

import (
	"errors"
	"fmt"
	"strings"
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

// ErrProviderAlreadyExists is returned when the Provided being registered
// already exists in the registration system
type ErrProviderAlreadyExists struct {
	Name string
}

func (err ErrProviderAlreadyExists) Error() string {
	return fmt.Sprintf("provider %s already exists", err.Name)
}

// ErrUnknownProvider is returned when no providers are registered or a requested
// provider is not registered
type ErrUnknownProvider struct {
	Name               string
	KnownProvidersFunc func() []string
}

func (err ErrUnknownProvider) Error() string {
	var errStr strings.Builder
	errStr.WriteString("no providers registered")
	if err.Name != "" {
		errStr.Reset()
		fmt.Fprintf(&errStr, "no providers registered by the name %s", err.Name)
	}
	if err.KnownProvidersFunc != nil {
		errStr.WriteString(", known providers:")
		errStr.WriteString(strings.Join(err.KnownProvidersFunc(), ","))
	}

	return errStr.String()
}
