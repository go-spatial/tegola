package file

import (
	"errors"
	"os"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/dict"
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

// New instantiates a Cache. The config expects the following params:
//
// 	basepath (string): a path to where the cache will be written
// 	max_zoom (int): max zoom to use the cache. beyond this zoom cache Set() calls will be ignored
//
func New(config dict.Dicter) (cache.Interface, error) {
	var err error

	// new filecache
	fc := Cache{}

	defaultMaxZoom := uint(tegola.MaxZ)
	fc.MaxZoom, err = config.Uint(ConfigKeyMaxZoom, &defaultMaxZoom)
	if err != nil {
		return nil, err
	}

	defaultMinZoom := uint(0)
	fc.MinZoom, err = config.Uint(ConfigKeyMinZoom, &defaultMinZoom)
	if err != nil {
		return nil, err
	}

	defaultBounds := "-180.0,-85,180,85"
	fc.Bounds, err = config.String(ConfigKeyBounds, &defaultBounds)
	if err != nil {
		return nil, err
	}

	fc.Basepath, err = config.String(ConfigKeyBasepath, nil)
	if err != nil {
		return nil, ErrMissingBasepath
	}

	if fc.Basepath == "" {
		return nil, ErrMissingBasepath
	}

	// make our basepath if it does not exist
	if err = os.MkdirAll(fc.Basepath, os.ModePerm); err != nil {
		return nil, err
	}

	return &fc, nil
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
