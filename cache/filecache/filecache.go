package filecache

import (
	"errors"
	"os"
	"path/filepath"

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
	}, nil
}

type Filecache struct {
	Basepath string

	//	locker tracks which cache keys are being operated on.
	//	when the cache is being written to a Lock() is used.
	//	when being read from an RLock() is used so we don't
	//	block concurrent reads
	//
	//	TODO: currently the map keys are not cleaned up after they're
	//	created. this will cause more memory to be used.
	locker map[string]Item
}

func (fc *Filecache) Get(key string) (*os.File, error) {
	path := filepath.Join(fc.Basepath, key)
	/*
		mutex, ok := fc.locker[key]
		if !ok {
			fc.locker[key] = sync.RWMutex{}
			mutex = fc.locker[key]
		}
		mutex.RLock()
		defer mutex.RUnlock()
	*/
	return os.Open(path)
}

/*
func (fc *Filecache) Set(key string) (*os.File, error) {
	var err error

	//	build our filepath
	path := filepath.Join(fc.Basepath, key)

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
	_, err = io.Copy(f, value)
	if err != nil {
		return err
	}

	return nil
}
*/

func (fc *Filecache) Set(key string) (*os.File, error) {
	var err error

	//	build our filepath
	path := filepath.Join(fc.Basepath, key)

	//	the key can have a directory syntax so we need to makeAll
	if err = os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return nil, err
	}

	//	create the file
	return os.Create(path)
}

func (fc *Filecache) Purge(key string) error {
	path := filepath.Join(fc.Basepath, key)

	//	check if we have a file. if no file exists, return
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	return os.Remove(path)
}
