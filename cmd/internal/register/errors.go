package register

import "fmt"

// ErrProviderLayerInvalid should be returned when an invalid Provider layer for a map is given
type ErrProviderLayerInvalid struct {
	ProviderLayer string
	Map           string
}

func (e ErrProviderLayerInvalid) Error() string {
	return fmt.Sprintf("invalid provider layer (%v) for map (%v)", e.ProviderLayer, e.Map)
}

// ErrProviderNotFound when the requested provider is not a known provider
type ErrProviderNotFound struct {
	Provider string
}

func (e ErrProviderNotFound) Error() string {
	return fmt.Sprintf("provider (%v) not defined", e.Provider)
}

// ErrProviderLayerNotRegistered should be returned when the requested provider was not registered into the system.
type ErrProviderLayerNotRegistered struct {
	MapName       string
	ProviderLayer string
	Provider      string
}

func (e ErrProviderLayerNotRegistered) Error() string {
	return fmt.Sprintf("map (%v) 'provider_layer' (%v) is not registered with provider (%v)", e.MapName, e.ProviderLayer, e.Provider)
}

// ErrFetchingLayerInfo wraps an error when attempting to obtain layer information for a provider
type ErrFetchingLayerInfo struct {
	Provider string
	Err      error
}

func (e ErrFetchingLayerInfo) Unwrap() error { return e.Err }
func (e ErrFetchingLayerInfo) Error() string {
	return fmt.Sprintf("error fetching layer info from provider (%v): %v", e.Provider, e.Err)
}

// ErrDefaultTagsInvalid should be returned when the type of defaultTags is not an acceptable type.
type ErrDefaultTagsInvalid struct {
	ProviderLayer string
	Err           error
}

func (e ErrDefaultTagsInvalid) Unwrap() error { return e.Err }
func (e ErrDefaultTagsInvalid) Error() string {
	return fmt.Sprintf("'default_tags' for 'provider_layer' (%v) should be a TOML table", e.ProviderLayer)
}
