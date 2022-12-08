package register

import (
	"errors"
	"fmt"

	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/provider"
)

var (
	ErrProviderNameMissing = errors.New("register: provider 'name' parameter missing")
	ErrProviderNameInvalid = errors.New("register: provider 'name' value must be a string")
)

type ErrProviderAlreadyRegistered string

func (e ErrProviderAlreadyRegistered) Error() string {
	return fmt.Sprintf("register: provider (%v) already registered", string(e))
}

type ErrProviderTypeMissing string

func (e ErrProviderTypeMissing) Error() string {
	return fmt.Sprintf("register: provider 'type' parameter missing for provider (%v)", string(e))
}

type ErrProviderTypeInvalid string

func (e ErrProviderTypeInvalid) Error() string {
	return fmt.Sprintf("register: provider 'type' must be a string for provider (%v)", string(e))
}

// Providers registers data provider backends
func Providers(providers []dict.Dicter, maps []provider.Map) (map[string]provider.TilerUnion, error) {
	// holder for registered providers
	registeredProviders := map[string]provider.TilerUnion{}

	// iterate providers
	for _, p := range providers {
		// lookup our provider name
		pname, err := p.String("name", nil)
		if err != nil {
			switch err.(type) {
			case dict.ErrKeyRequired:
				return registeredProviders, ErrProviderNameMissing
			case dict.ErrKeyType:
				return registeredProviders, ErrProviderNameInvalid
			default:
				return registeredProviders, err
			}
		}

		// check if a provider with this name is already registered
		_, ok := registeredProviders[pname]
		if ok {
			return registeredProviders, ErrProviderAlreadyRegistered(pname)
		}

		// lookup our provider type
		ptype, err := p.String("type", nil)
		if err != nil {
			switch err.(type) {
			case dict.ErrKeyRequired:
				return registeredProviders, ErrProviderTypeMissing(pname)
			case dict.ErrKeyType:
				return registeredProviders, ErrProviderTypeInvalid(pname)
			default:
				return registeredProviders, err
			}
		}

		// register the provider
		prov, err := provider.For(ptype, p, maps)
		if err != nil {
			return registeredProviders, err
		}

		// add the provider to our map of registered providers
		registeredProviders[pname] = prov
		log.Infof("registering provider(type): %v (%v)", pname, ptype)
	}

	return registeredProviders, nil
}
