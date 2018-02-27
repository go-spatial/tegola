package memory

import (
	"sync"

	"github.com/go-spatial/tegola/cache"
)

func New() *MemoryCache {
	return &MemoryCache{
		keyVals: map[string][]byte{},
	}
}

//	test cacher, implements the cache.Interface
type MemoryCache struct {
	keyVals map[string][]byte
	sync.RWMutex
}

func (mc *MemoryCache) Get(key *cache.Key) ([]byte, bool, error) {
	mc.RLock()
	defer mc.RUnlock()

	val, ok := mc.keyVals[key.String()]
	if !ok {
		return nil, false, nil
	}

	return val, true, nil
}

func (mc *MemoryCache) Set(key *cache.Key, val []byte) error {
	mc.Lock()
	defer mc.Unlock()

	mc.keyVals[key.String()] = val

	return nil
}

func (mc *MemoryCache) Purge(key *cache.Key) error {
	mc.Lock()
	defer mc.Unlock()

	delete(mc.keyVals, key.String())

	return nil
}
