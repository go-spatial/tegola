package provider

import (
	"fmt"
	"strings"

	"github.com/terranodo/tegola/mvt"
)

// InitFunc initilize a provider given a config map. The init function should validate the config map, and report any errors. This is called by the For function.
type InitFunc func(map[string]interface{}) (mvt.Provider, error)

var providers map[string]InitFunc

// Register is called by the init functions of the provider.
func Register(name string, init InitFunc) error {
	if providers == nil {
		providers = make(map[string]InitFunc)
	}
	if _, ok := providers[name]; ok {
		return fmt.Errorf("Provider %v already exists", name)
	}
	providers[name] = init
	return nil
}

// Drivers returns a list of drivers that have registered.
func Drivers() (l []string) {
	if providers == nil {
		return l
	}
	for k, _ := range providers {
		l = append(l, k)
	}
	return l
}

// For function returns a configured provider of the given type, provided the correct config map.
func For(name string, config map[string]interface{}) (mvt.Provider, error) {
	if providers == nil {
		return nil, fmt.Errorf("No providers registered.")
	}
	p, ok := providers[name]
	if !ok {
		return nil, fmt.Errorf("No providers registered by the name: %v, known providers(%v)", name, strings.Join(Drivers(), ","))
	}
	return p(config)
}
