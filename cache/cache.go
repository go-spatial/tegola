package cache

import (
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"
)

//	Interface defines a cache back end
type Interface interface {
	Get(key *Key) (val []byte, hit bool, err error)
	Set(key *Key, val []byte) error
	Purge(key *Key) error
}

//	ParseKey will parse a string in the format /:map/:layer/:z/:x/:y into a Key struct. The :layer value is optional
func ParseKey(str string) (*Key, error) {
	var err error
	var key Key

	//	remove the basepath and the first slash, then split the parts
	keyParts := strings.Split(strings.TrimLeft(str, "/"), "/")
	//	we're expecting a z/x/y scheme
	if len(keyParts) < 3 || len(keyParts) > 5 {
		err = ErrInvalidFileKey{
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
	key.Z, err = strconv.Atoi(zxy[0])
	if err != nil {
		err = ErrInvalidFileKey{
			path:          str,
			keyPartsCount: len(keyParts),
		}

		log.Printf("cache: invalid fileKey (%v). unable to parse Z value (%v) into int. skipping.", str, zxy[0])
		return nil, err
	}

	key.X, err = strconv.Atoi(zxy[1])
	if err != nil {
		err = ErrInvalidFileKey{
			path:          str,
			keyPartsCount: len(keyParts),
		}

		log.Printf("cache: invalid fileKey (%v). unable to parse X value (%v) into int. skipping.", str, zxy[0])
		return nil, err
	}

	//	trim the extension if it exists
	yParts := strings.Split(zxy[2], ".")
	key.Y, err = strconv.Atoi(yParts[0])
	if err != nil {
		err = ErrInvalidFileKey{
			path:          str,
			keyPartsCount: len(keyParts),
		}

		log.Printf("cache: invalid fileKey (%v). unable to parse Y value (%v) into int. skipping.", str, zxy[0])
		return nil, err
	}

	return &key, nil
}

type Key struct {
	MapName   string
	LayerName string
	Z         int
	X         int
	Y         int
}

func (k Key) String() string {
	return filepath.Join(k.MapName, k.LayerName, strconv.Itoa(k.Z), strconv.Itoa(k.X), strconv.Itoa(k.Y))
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
