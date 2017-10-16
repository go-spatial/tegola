package cache

import (
	"fmt"
	"io"
)

//	Cacher defines a cache back end
type Cacher interface {
	Get(key string) (io.Reader, error)
	Set(key string, value io.Reader) error
	Purge(key string) error
	GetWriter(key string) (io.Writer, error)
}

// InitFunc initilize a cache given a config map.
// The InitFunc should validate the config map, and report any errors.
// This is called by the For function.
type InitFunc func(map[string]interface{}) (Cacher, error)

var cache map[string]InitFunc

// Register is called by the init functions of the cache.
func Register(cacheType string, init InitFunc) error {
	if cache == nil {
		cache = make(map[string]InitFunc)
	}

	if _, ok := cache[cacheType]; ok {
		return fmt.Errorf("Cache (%v) already exists", cacheType)

	}
	cache[cacheType] = init

	return nil
}

// For function returns a configed cache of the given type, provided the correct config map.
func For(cacheType string, config map[string]interface{}) (Cacher, error) {
	if cache == nil {
		return nil, fmt.Errorf("No cache backends registered.")
	}

	c, ok := cache[cacheType]
	if !ok {
		return nil, fmt.Errorf("No cache backends registered by the cache type: (%v)", cacheType)
	}

	return c(config)
}
