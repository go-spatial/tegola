package cache

import (
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/maths"
)

//	Interface defines a cache back end
type Interface interface {
	Get(key *Key) (val []byte, hit bool, err error)
	Set(key *Key, val []byte) error
	Purge(key *Key) error
}

//	ParseKey will parse a string in the format /:map/:layer/:z/:x/:y into a Key struct. The :layer value is optional
//	ParseKey also supports other OS delimeters (i.e. Windows - "\")
func ParseKey(str string) (*Key, error) {
	var err error
	var key Key

	//	convert to all slashes to forward slashes. without this reading from certain OSes (i.e. windows)
	//	will fail our keyParts check since it uses backslashes.
	str = filepath.ToSlash(str)

	//	remove the basepath and the first slash, then split the parts
	keyParts := strings.Split(strings.TrimLeft(str, "/"), "/")
	//	we're expecting a z/x/y scheme
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
	case 5: //	map, layer, z, x, y
		key.MapName = keyParts[0]
		key.LayerName = keyParts[1]
		zxy = keyParts[2:]
	case 4: //	map, z, x, y
		key.MapName = keyParts[0]
		zxy = keyParts[1:]
	case 3: // z, x, y
		zxy = keyParts
	}

	//	parse our URL vals to ints
	key.Z, err = strconv.ParseUint(zxy[0], 10, 32)
	if err != nil || key.Z > tegola.MaxZ {
		err = ErrInvalidFileKey{
			path: str,
			key:  "Z",
			val:  zxy[0],
		}

		log.Printf(err.Error())
		return nil, err
	}

	maxXYatZ := maths.Exp2(key.Z) - 1

	key.X, err = strconv.ParseUint(zxy[1], 10, 32)
	if err != nil || key.X > maxXYatZ{
		err = ErrInvalidFileKey{
			path: str,
			key:  "X",
			val:  zxy[1],
		}

		log.Printf(err.Error())
		return nil, err
	}

	//	trim the extension if it exists
	yParts := strings.Split(zxy[2], ".")
	key.Y, err = strconv.ParseUint(yParts[0], 10, 64)
	if err != nil || key.Y > maxXYatZ {
		err = ErrInvalidFileKey{
			path: str,
			key:  "Y",
			val:  zxy[2],
		}

		log.Printf(err.Error())
		return nil, err
	}

	return &key, nil
}

type Key struct {
	MapName   string
	LayerName string
	Z         uint64
	X         uint64
	Y         uint64
}

func (k Key) String() string {
	return filepath.Join(
		k.MapName,
		k.LayerName,
		strconv.FormatUint(k.Z, 10),
		strconv.FormatUint(k.X, 10),
		strconv.FormatUint(k.Y, 10))
}

// InitFunc initilize a cache given a config map.
// The InitFunc should validate the config map, and report any errors.
// This is called by the For function.
type InitFunc func(map[string]interface{}) (Interface, error)

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

// Registered returns the caché's that have been registered.
func Registered() (c []string) {
	for k, _ := range cache {
		c = append(c, k)
	}
	sort.Strings(c)
	return c
}

// For function returns a configed cache of the given type, provided the correct config map.
func For(cacheType string, config map[string]interface{}) (Interface, error) {
	if cache == nil {
		return nil, fmt.Errorf("No cache backends registered.")
	}

	c, ok := cache[cacheType]
	if !ok {
		return nil, fmt.Errorf("No cache backends registered by the cache type: (%v)", cacheType)
	}

	return c(config)
}
