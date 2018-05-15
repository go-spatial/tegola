package register

import (
	"errors"

	"github.com/go-spatial/tegola/cache"
)

var (
	ErrCacheTypeMissing = errors.New("register: cache 'type' parameter missing")
	ErrCacheTypeInvalid = errors.New("register: cache 'type' value must be a string")
)

// Cache registers cache backends
func Cache(config map[string]interface{}) (cache.Interface, error) {
	// lookup our cache type
	t, ok := config["type"]
	if !ok {
		return nil, ErrCacheTypeMissing
	}

	cType, ok := t.(string)
	if !ok {
		return nil, ErrCacheTypeInvalid
	}

	// register the provider
	return cache.For(cType, config)
}
