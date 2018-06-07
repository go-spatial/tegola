package register

import (
	"errors"

	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/internal/dict"
)

var (
	ErrCacheTypeMissing = errors.New("register: cache 'type' parameter missing")
	ErrCacheTypeInvalid = errors.New("register: cache 'type' value must be a string")
)

// Cache registers cache backends
func Cache(config dict.Dicter) (cache.Interface, error) {
	cType, err := config.String("type", nil)
	if err != nil {
		switch err.(type) {
		case dict.ErrKeyRequired:
			return nil, ErrCacheTypeMissing
		case dict.ErrKeyType:
			return nil, ErrCacheTypeInvalid
		default:
			return nil, err
		}
	}

	// register the provider
	return cache.For(cType, config)
}
