package register

import (
	"errors"
	"fmt"

	"github.com/go-spatial/tegola/provider"
)

var (
	ErrProviderNameMissing = errors.New("register: provider 'name' parameter missing")
	ErrProviderNameInvalid = errors.New("register: provider 'name' value must be a string")
)

type ErrProviderAlreadyRegistered struct {
	ProviderName string
}

func (e ErrProviderAlreadyRegistered) Error() string {
	return fmt.Sprintf("register: provider (%v) already registered", e.ProviderName)
}

type ErrProviderTypeMissing struct {
	ProviderName string
}

func (e ErrProviderTypeMissing) Error() string {
	return fmt.Sprintf("register: provider 'type' parameter missing for provider (%v)", e.ProviderName)
}

type ErrProviderTypeInvalid struct {
	ProviderName string
}

func (e ErrProviderTypeInvalid) Error() string {
	return fmt.Sprintf("register: provider 'type' must be a string for provider (%v)", e.ProviderName)
}

// Providers registers data provider backends
func Providers(providers []map[string]interface{}) (map[string]provider.Tiler, error) {
	var err error

	// holder for registered providers
	registeredProviders := map[string]provider.Tiler{}

	// iterate providers
	for _, p := range providers {
		// lookup our proivder name
		n, ok := p["name"]
		if !ok {
			return registeredProviders, ErrProviderNameMissing
		}

		pname, found := n.(string)
		if !found {
			return registeredProviders, ErrProviderNameInvalid
		}

		// check if a proivder with this name is alrady registered
		_, ok = registeredProviders[pname]
		if ok {
			return registeredProviders, ErrProviderAlreadyRegistered{pname}
		}

		// lookup our provider type
		t, ok := p["type"]
		if !ok {
			return registeredProviders, ErrProviderTypeMissing{pname}
		}

		ptype, found := t.(string)
		if !found {
			return registeredProviders, ErrProviderTypeInvalid{pname}
		}

		// register the provider
		prov, err := provider.For(ptype, p)
		if err != nil {
			return registeredProviders, err
		}

		// add the provider to our map of registered providers
		registeredProviders[pname] = prov
	}

	return registeredProviders, err
}
