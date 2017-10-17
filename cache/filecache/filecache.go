package filecache

import (
	"errors"
	"io/ioutil"
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
)

func init() {
	cache.Register(CacheType, New)
}

//	New instantiates a Filecache. The config expects the following params:
//
//		basepath (string): a path to where the cache will be written

func New(config map[string]interface{}) (cache.Cacher, error) {
	var err error

	c := dict.M(config)

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

	return &Filecache{
		Basepath: basepath,
		Locker:   map[string]sync.RWMutex{},
	}, nil
}

type Filecache struct {
	Basepath string

	//	Locker tracks which cache keys are being operated on.
	//	when the cache is being written to a Lock() is set.
	//	when being read from an RLock() is used so we don't
	//	block concurrent reads.
	//
	//	TODO: currently the map keys are not cleaned up after they're
	//	created. this will cause more memory to be used.
	Locker map[string]sync.RWMutex

	//	we need a cache mutex to avoid concurrent writes to our Locker
	sync.RWMutex
}

func (fc *Filecache) Get(key string) ([]byte, error) {
	path := filepath.Join(fc.Basepath, key)

	//	lookup our mutex
	fc.RLock()
	mutex, ok := fc.Locker[key]
	fc.RUnlock()
	if !ok {
		//	no entry, return
		return nil, os.ErrNotExist
	}

	//	read lock
	mutex.RLock()
	defer mutex.RUnlock()

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return ioutil.ReadAll(f)
}

func (fc *Filecache) Set(key string, val []byte) error {
	var err error

	//	build our filepath
	path := filepath.Join(fc.Basepath, key)

	//	lookup our mutex
	mutex, ok := fc.Locker[key]
	if !ok {
		fc.Lock()
		fc.Locker[key] = sync.RWMutex{}
		fc.Unlock()
		mutex = fc.Locker[key]
	}
	//	write lock
	mutex.Lock()
	defer mutex.Unlock()

	//	the key can have a directory syntax so we need to makeAll
	if err = os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return err
	}

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

func (fc *Filecache) Purge(key string) error {
	path := filepath.Join(fc.Basepath, key)

	//	check if we have a file. if no file exists, return
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	return os.Remove(path)
}
