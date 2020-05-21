package register

import (
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/mvtprovider"
)

// MVTProviders registers data provider backends
func MVTProviders(providers []dict.Dicter) (map[string]mvtprovider.Tiler, error) {

	// holder for registered providers
	registeredProviders := map[string]mvtprovider.Tiler{}

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

		pname = mvtprovider.NamePrefix + pname

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
		prov, err := mvtprovider.For(ptype, p)
		if err != nil {
			log.Infof("registering provider %v error: %v", ptype, err)
			return registeredProviders, err
		}

		// add the provider to our map of registered providers
		registeredProviders[pname] = prov
		log.Infof("registering mvt provider(type): %v (%v)", pname, ptype)
	}

	return registeredProviders, nil
}
