package file

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/terranodo/tegola/cache"
	"github.com/terranodo/tegola/util/dict"
)

var (
	ErrMissingBasepath = errors.New("filecache: missing required param 'basepath'")
)

const CacheType = "file"

const (
	ConfigKeyBasepath = "basepath"
	ConfigKeyMaxZoom  = "max_zoom"
)

func init() {
	cache.Register(CacheType, New)
}

//	New instantiates a Filecache. The config expects the following params:
//
//		basepath (string): a path to where the cache will be written
//		max_zoom (int): max zoom to use the cache. beyond this zoom cache Set() calls will be ignored
//
func New(config map[string]interface{}) (cache.Interface, error) {
	var err error

	//	new filecache
	fc := Filecache{}

	//	parse the config
	c := dict.M(config)

	//	TODO: this could be cleaner
	defaultMaxZoom := 0
	maxZoom, err := c.Int(ConfigKeyMaxZoom, &defaultMaxZoom)
	if err != nil {
		return nil, err
	}
	if maxZoom != 0 {
		mz := uint(maxZoom)
		fc.MaxZoom = &mz
	}

	fc.Basepath, err = c.String(ConfigKeyBasepath, nil)
	if err != nil {
		return nil, ErrMissingBasepath
	}

	if fc.Basepath == "" {
		return nil, ErrMissingBasepath
	}

	//	make our basepath if it does not exist
	if err = os.MkdirAll(fc.Basepath, os.ModePerm); err != nil {
		return nil, err
	}

	return &fc, nil
}

type Filecache struct {
	Basepath string
	//	MaxZoom determins the max zoom the cache to persist. Beyond this
	//	zoom, cache Set() calls will be ignored. This is useful if the cache
	//	should not be leveraged for higher zooms when data changes often.
	MaxZoom *uint
}

// 	Get reads a z,x,y entry from the cache and returns the contents
//	if there is a hit. the second argument denotes a hit or miss
//	so the consumer does not need to sniff errors for cache read misses
func (fc *Filecache) Get(key *cache.Key) ([]byte, bool, error) {
	path := filepath.Join(fc.Basepath, key.String())

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}

		return nil, false, err
	}

	val, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, false, err
	}

	return val, true, nil
}

func (fc *Filecache) Set(key *cache.Key, val []byte) error {
	var err error

	//	check for maxzoom
	if fc.MaxZoom != nil && key.Z > int(*fc.MaxZoom) {
		return nil
	}

	//	the tmpPath uses the destPath with a simple "-tmp" suffix. we're going to do
	//	a Rename at the end of this method and according to the os.Rename() docs:
	//	"If newpath already exists and is not a directory, Rename replaces it.
	//	OS-specific restrictions may apply when oldpath and newpath are in different directories"
	destPath := filepath.Join(fc.Basepath, key.String())
	tmpPath := destPath + "-tmp"

	//	the key can have a directory syntax so we need to makeAll
	if err = os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
		return err
	}

	//	create the file
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer f.Close()

	//	copy the contents
	_, err = f.Write(val)
	if err != nil {
		return err
	}

	//	move the temp file to the destination
	return os.Rename(tmpPath, destPath)
}

func (fc *Filecache) Purge(key *cache.Key) error {
	path := filepath.Join(fc.Basepath, key.String())

	//	check if we have a file. if no file exists, return
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	//	remove the locker key on purge
	return os.Remove(path)
}
