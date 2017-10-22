package filecache

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/terranodo/tegola/cache"
	"github.com/terranodo/tegola/util/dict"
)

var (
	ErrMissingBasepath = errors.New("filecache: missing required param 'basepath'")
	ErrCacheMiss       = errors.New("filecache: cache miss")
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
//
func New(config map[string]interface{}) (cache.Interface, error) {
	var err error

	c := dict.M(config)

	/*
		maxZoom, err := c.Uint(ConfigKeyMaxZoom, nil)
		if err != nil {

			return nil, ErrMissingBasepath
		}
	*/
	basepath, err := c.String(ConfigKeyBasepath, nil)
	if err != nil {
		return nil, ErrMissingBasepath
	}

	if basepath == "" {
		return nil, ErrMissingBasepath
	}

	//	make our basepath if it does not exist
	if err = os.MkdirAll(basepath, os.ModePerm); err != nil {
		return nil, err
	}

	fc := Filecache{
		Basepath: basepath,
		Locker:   map[string]sync.RWMutex{},
	}

	//	TODO: walk our basepath and full our Locker with already rendered keys
	err = filepath.Walk(basepath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		//	skip directories
		if info.IsDir() {
			return nil
		}

		//	remove the basepath for the file key
		fileKey := path[len(basepath):]

		cacheKey, err := cache.ParseKey(fileKey)
		if err != nil {
			log.Println("filecache: ", err.Error())
			return nil
		}

		//	write our key
		fc.Lock()
		fc.Locker[cacheKey.String()] = sync.RWMutex{}
		fc.Unlock()

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &fc, nil
}

type Filecache struct {
	Basepath string

	//	we need a cache mutex to avoid concurrent writes to our Locker
	sync.RWMutex

	//	Locker tracks which cache keys are being operated on.
	//	when the cache is being written to a Lock() is set.
	//	when being read from an RLock() is used so we don't
	//	block concurrent reads.
	//
	//	TODO: store a hash of the cache blob along with the Locker mutex
	Locker map[string]sync.RWMutex

	//	MaxZoom determins which zoom max should leverage the cache.
	//	This is useful if the cache should not be leveraged for higher
	//	zooms (i.e. 10+).
	//
	//	TODO: implement
	MaxZoom uint
}

// 	Get reads a z,x,y entry from the cache and returns the contents
//	if there is a hit. the second argument denotes a hit or miss
//	so the consumer does not need to sniff errors for cache read misses
func (fc *Filecache) Get(key *cache.Key) ([]byte, bool, error) {
	path := filepath.Join(fc.Basepath, key.String())

	//	lookup our mutex
	fc.RLock()
	mutex, ok := fc.Locker[key.String()]
	fc.RUnlock()
	if !ok {
		//	no entry, return a miss
		return nil, false, nil
	}

	//	read lock
	mutex.RLock()
	defer mutex.RUnlock()

	f, err := os.Open(path)
	if err != nil {
		//	something is wrong with opening this file
		//	remove the key from the cache if it exists
		fc.Lock()
		delete(fc.Locker, key.String())
		fc.Unlock()

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

	path := filepath.Join(fc.Basepath, key.String())

	//	lookup our mutex
	mutex, ok := fc.Locker[key.String()]
	if !ok {
		fc.Lock()
		fc.Locker[key.String()] = sync.RWMutex{}
		fc.Unlock()
		mutex = fc.Locker[key.String()]
	}
	//	the key can have a directory syntax so we need to makeAll
	if err = os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return err
	}

	//	write lock
	mutex.Lock()
	defer mutex.Unlock()

	//	create the file
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	//	copy the contents
	_, err = f.Write(val)
	if err != nil {
		return err
	}

	return nil
}

func (fc *Filecache) Purge(key *cache.Key) error {
	path := filepath.Join(fc.Basepath, key.String())

	//	check if we have a file. if no file exists, return
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	//	remove the locker key on purge
	fc.Lock()
	delete(fc.Locker, key.String())
	fc.Unlock()

	return os.Remove(path)
}
