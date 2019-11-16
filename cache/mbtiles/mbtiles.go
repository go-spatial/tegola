package file

import (
	"errors"

	"github.com/go-spatial/tegola/cache"
)

var (
	ErrMissingBasepath = errors.New("mbtilescache: missing required param 'basepath'")
)

//TODO attribution form maps definition (if possible)
//TODO set generic description

const CacheType = "mbtiles"

const (
	ConfigKeyBasepath = "basepath"
	ConfigKeyMaxZoom  = "max_zoom"
	ConfigKeyMinZoom  = "min_zoom"
	ConfigKeyBounds   = "bounds"
)

func init() {
	cache.Register(CacheType, New)
}

//Cache hold the cache configuration
type Cache struct {
	Basepath string
	Bounds   string
	// MinZoom determines the min zoom the cache to persist. Before this
	// zoom, cache Set() calls will be ignored.
	MinZoom uint
	// MaxZoom determines the max zoom the cache to persist. Beyond this
	// zoom, cache Set() calls will be ignored. This is useful if the cache
	// should not be leveraged for higher zooms when data changes often.
	MaxZoom uint
}

//TODO Ignore from cache layer name (not suported by mbtiles)
//TODO Create one file for each map name (use `default` is .MapName is not set)

//Get reads a z,x,y entry from the cache and returns the contents
// if there is a hit. the second argument denotes a hit or miss
// so the consumer does not need to sniff errors for cache read misses
func (fc *Cache) Get(key *cache.Key) ([]byte, bool, error) {
	//TODO
	return nil, false, nil
}

//Set save a z,x,y entry in the cache
func (fc *Cache) Set(key *cache.Key, val []byte) error {
	//TODO
	return nil
}

//Purge clear a z,x,y entry from the cache
func (fc *Cache) Purge(key *cache.Key) error {
	//TODO
	return nil
}
