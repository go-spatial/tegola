package cache

import (
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/maths"
)

// Interface defines a cache back end
type Interface interface {
	Get(key *Key) (val []byte, hit bool, err error)
	Set(key *Key, val []byte) error
	Purge(key *Key) error
}

// Wrapped Cache are for cache backend that wrap other cache backends
// Original will return the first cache backend to be wrapped
type Wrapped interface {
	Original() Interface
}

// ParseKey will parse a string in the format /:map/:layer/:z/:x/:y into a Key struct. The :layer value is optional
// ParseKey also supports other OS delimiters (i.e. Windows - "\")
func ParseKey(str string) (*Key, error) {
	var err error
	var key Key

	// convert to all slashes to forward slashes. without this reading from certain OSes (i.e. windows)
	// will fail our keyParts check since it uses backslashes.
	str = filepath.ToSlash(str)

	// remove the base-path and the first slash, then split the parts
	keyParts := strings.Split(strings.TrimLeft(str, "/"), "/")

	// we're expecting a z/x/y scheme
	if len(keyParts) < 3 || len(keyParts) > 5 {
		err = ErrInvalidFileKeyParts{
			path:          str,
			keyPartsCount: len(keyParts),
		}

		log.Println(err.Error())
		return nil, err
	}

	var zxy []string

	switch len(keyParts) {
	case 5: // map, layer, z, x, y
		key.MapName = keyParts[0]
		key.LayerName = keyParts[1]
		zxy = keyParts[2:]
	case 4: // map, z, x, y
		key.MapName = keyParts[0]
		zxy = keyParts[1:]
	case 3: // z, x, y
		zxy = keyParts
	}

	// parse our URL vals into integers
	var placeholder uint64
	placeholder, err = strconv.ParseUint(zxy[0], 10, 32)
	if err != nil || placeholder > tegola.MaxZ {
		err = ErrInvalidFileKey{
			path: str,
			key:  "Z",
			val:  zxy[0],
		}

		log.Printf(err.Error())
		return nil, err
	}

	key.Z = uint(placeholder)
	maxXYatZ := maths.Exp2(placeholder) - 1

	placeholder, err = strconv.ParseUint(zxy[1], 10, 32)
	if err != nil || placeholder > maxXYatZ {
		err = ErrInvalidFileKey{
			path: str,
			key:  "X",
			val:  zxy[1],
		}

		log.Printf(err.Error())
		return nil, err
	}

	key.X = uint(placeholder)

	// trim the extension if it exists
	yParts := strings.Split(zxy[2], ".")
	placeholder, err = strconv.ParseUint(yParts[0], 10, 64)
	if err != nil || placeholder > maxXYatZ {
		err = ErrInvalidFileKey{
			path: str,
			key:  "Y",
			val:  zxy[2],
		}

		log.Printf(err.Error())
		return nil, err
	}
	key.Y = uint(placeholder)

	return &key, nil
}

type Key struct {
	MapName   string
	LayerName string
	Z         uint
	X         uint
	Y         uint
}

func (k Key) String() string {
	return filepath.Join(
		k.MapName,
		k.LayerName,
		strconv.FormatUint(uint64(k.Z), 10),
		strconv.FormatUint(uint64(k.X), 10),
		strconv.FormatUint(uint64(k.Y), 10))
}

// InitFunc initialize a cache given a config map.
// The InitFunc should validate the config map, and report any errors.
// This is called by the For function.
type InitFunc func(dict.Dicter) (Interface, error)

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

// Registered returns the cache's that have been registered.
func Registered() (c []string) {
	for k := range cache {
		c = append(c, k)
	}
	sort.Strings(c)
	return c
}

// For function returns a configured cache of the given type, provided the correct config map.
func For(cacheType string, config dict.Dicter) (Interface, error) {
	if cache == nil {
		return nil, fmt.Errorf("No cache backends registered.")
	}

	c, ok := cache[cacheType]
	if !ok {
		return nil, fmt.Errorf("No cache backends registered by the cache type: (%v)", cacheType)
	}

	return c(config)
}
