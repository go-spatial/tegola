package config

import (
	"fmt"
	"strings"

	"github.com/go-spatial/tegola/provider"
)

type ErrMapNotFound struct {
	MapName string
}

func (e ErrMapNotFound) Error() string {
	return fmt.Sprintf("config: map (%v) not found", e.MapName)
}

type ErrParamTokenReserved struct {
	MapName   string
	Parameter provider.QueryParameter
}

func (e ErrParamTokenReserved) Error() string {
	return fmt.Sprintf("config: map %s parameter %s uses a reserved token name %s",
		e.MapName, e.Parameter.Name, e.Parameter.Token)
}

type ErrParamDuplicateName struct {
	MapName   string
	Parameter provider.QueryParameter
}

func (e ErrParamDuplicateName) Error() string {
	return fmt.Sprintf("config: map %s parameter %s has a name already used by another parameter",
		e.MapName, e.Parameter.Name)
}

type ErrParamDuplicateToken struct {
	MapName   string
	Parameter provider.QueryParameter
}

func (e ErrParamDuplicateToken) Error() string {
	return fmt.Sprintf("config: map %s parameter %s has a token name %s already used by another parameter",
		e.MapName, e.Parameter.Token, e.Parameter.Name)
}

type ErrParamUnknownType struct {
	MapName   string
	Parameter provider.QueryParameter
}

func (e ErrParamUnknownType) Error() string {
	validTypes := make([]string, 0, len(provider.ParamTypeDecoders))
	for k := range provider.ParamTypeDecoders {
		validTypes = append(validTypes, k)
	}

	return fmt.Sprintf("config: map %s parameter %s has an unknown type %s, must be one of: %s",
		e.MapName, e.Parameter.Name, e.Parameter.Type, strings.Join(validTypes, ","))
}

type ErrParamTwoDefaults struct {
	MapName   string
	Parameter provider.QueryParameter
}

func (e ErrParamTwoDefaults) Error() string {
	return fmt.Sprintf("config: map %s parameter %s has both default_value and default_sql",
		e.MapName, e.Parameter.Name)
}

type ErrParamInvalidDefault struct {
	MapName   string
	Parameter provider.QueryParameter
}

func (e ErrParamInvalidDefault) Error() string {
	return fmt.Sprintf("config: map %s parameter %s has a default value that is invalid for type %s",
		e.MapName, e.Parameter.Name, e.Parameter.Type)
}

type ErrParamBadTokenName struct {
	MapName   string
	Parameter provider.QueryParameter
}

func (e ErrParamBadTokenName) Error() string {
	return fmt.Sprintf("config: map %s parameter %s has an invalid token name %s",
		e.MapName, e.Parameter.Name, e.Parameter.Token)
}

type ErrInvalidProviderForMap struct {
	MapName      string
	ProviderName string
}

func (e ErrInvalidProviderForMap) Error() string {
	return fmt.Sprintf("config: map %s references unknown provider %s", e.MapName, e.ProviderName)
}

type ErrInvalidProviderLayerName struct {
	ProviderLayerName string
}

func (e ErrInvalidProviderLayerName) Error() string {
	return fmt.Sprintf("config: invalid provider layer name (%v)", e.ProviderLayerName)
}

type ErrOverlappingLayerZooms struct {
	ProviderLayer1 string
	ProviderLayer2 string
}

func (e ErrOverlappingLayerZooms) Error() string {
	return fmt.Sprintf("config: overlapping zooms for layer (%v) and layer (%v)", e.ProviderLayer1, e.ProviderLayer2)
}

type ErrInvalidLayerZoom struct {
	ProviderLayer string
	MinZoom       bool
	Zoom          int
	ZoomLimit     int
}

func (e ErrInvalidLayerZoom) Error() string {
	n, d := "MaxZoom", "above"
	if e.MinZoom {
		n, d = "MinZoom", "below"
	}
	return fmt.Sprintf(
		"config: for provider layer %v %v(%v) is %v allowed level of %v",
		e.ProviderLayer, n, e.Zoom, d, e.ZoomLimit,
	)
}

// ErrMVTDifferentProviders represents when there are two different MVT providers in a map
// definition. MVT providers have to be unique per map definition
type ErrMVTDifferentProviders struct {
	Original string
	Current  string
}

func (e ErrMVTDifferentProviders) Error() string {
	return fmt.Sprintf(
		"config: all layer providers need to be the same, first provider is %s second provider is %s",
		e.Original,
		e.Current,
	)
}

// ErrMixedProviders represents the user configuration issue of using an MVT provider with another provider
type ErrMixedProviders struct {
	Map string
}

func (e ErrMixedProviders) Error() string {
	return fmt.Sprintf("config: can not mix MVT providers with normal providers for map %v", e.Map)
}

// ErrMissingEnvVar represents an environmental variable the system was unable to find in the environment
type ErrMissingEnvVar struct {
	EnvVar string
}

func (e ErrMissingEnvVar) Error() string {
	return fmt.Sprintf("config: config file is referencing an environment variable that is not set (%v)", e.EnvVar)
}

type ErrInvalidHeader struct {
	Header string
}

func (e ErrInvalidHeader) Error() string {
	return fmt.Sprintf("config: header (%v) blacklisted", e.Header)
}

type ErrInvalidURIPrefix string

func (e ErrInvalidURIPrefix) Error() string {
	return fmt.Sprintf("config: invalid uri_prefix (%v). uri_prefix must start with a forward slash '/' ", string(e))
}

// ErrUnknownProviderType is returned when the config contains a provider type that has not been registered
type ErrUnknownProviderType struct {
	Name           string // Name is the name of the entry in the config
	Type           string // Type is the name of the data provider
	KnownProviders []string
}

func (e ErrUnknownProviderType) Error() string {
	return fmt.Sprintf("config: invalid type (%s) for provider %s; known providers are: %v", e.Name, e.Type, strings.Join(e.KnownProviders, ","))
}

// Is returns whether the error is of type ErrUnknownProviderType, only checking the Type value.
func (e ErrUnknownProviderType) Is(err error) bool {
	err1, ok := err.(ErrUnknownProviderType)
	if !ok {
		return false
	}
	return err1.Type == e.Type
}

// ErrProviderNameRequired is returned when the name of a provider is missing from the provider list
type ErrProviderNameRequired struct {
	Pos int
}

func (e ErrProviderNameRequired) Error() string {
	return fmt.Sprintf("config: name field required for provider at position %v", e.Pos)
}

// ErrProviderNameDuplicate is returned when the name of a provider is duplicated in the provider list
type ErrProviderNameDuplicate struct {
	Pos int
}

func (e ErrProviderNameDuplicate) Error() string {
	return fmt.Sprintf("config: name for provider at position %v is a duplicate", e.Pos)
}

// ErrProviderTypeRequired is returned when the type of a provider is missing from the provider list
type ErrProviderTypeRequired struct {
	Pos int
}

func (e ErrProviderTypeRequired) Error() string {
	return fmt.Sprintf("config: type field required for provider at position %v", e.Pos)
}
