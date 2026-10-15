package memory

import (
	"context"
	"sync"
	"time"

	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/dict"
)

const CacheType = "memory"

const (
	ConfigKeyMaxZoom = "max_zoom"
	ConfigKeyTTL     = "ttl"
)

var (
	defaultMaxZoom = uint(tegola.MaxZ)
	defaultTTL     = 0
)

func init() {
	_ = cache.Register(CacheType, New)
}

func New(config dict.Dicter) (cache.Interface, error) {
	if config == nil {
		config = dict.Dict{}
	}

	maxZoom, err := config.Uint(ConfigKeyMaxZoom, &defaultMaxZoom)
	if err != nil {
		return nil, err
	}

	ttl, err := config.Int(ConfigKeyTTL, &defaultTTL)
	if err != nil {
		return nil, err
	}

	return &MemoryCache{
		keyVals:    map[string]memoryEntry{},
		MaxZoom:    maxZoom,
		Expiration: time.Duration(ttl) * time.Second,
	}, nil
}

type memoryEntry struct {
	value     []byte
	expiresAt time.Time
}

// MemoryCache is a process-local tile cache. Entries are lost when the
// process exits and are lazily removed after their TTL expires.
type MemoryCache struct {
	keyVals    map[string]memoryEntry
	MaxZoom    uint
	Expiration time.Duration
	sync.RWMutex
}

func (mc *MemoryCache) Get(ctx context.Context, key *cache.Key) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	mc.Lock()
	defer mc.Unlock()

	entry, ok := mc.keyVals[key.String()]
	if !ok {
		return nil, false, nil
	}
	if mc.Expiration > 0 && time.Now().After(entry.expiresAt) {
		delete(mc.keyVals, key.String())
		return nil, false, nil
	}

	return clone(entry.value), true, nil
}

func (mc *MemoryCache) Set(ctx context.Context, key *cache.Key, val []byte) error {
	if key.Z > mc.MaxZoom {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	mc.Lock()
	defer mc.Unlock()

	entry := memoryEntry{value: clone(val)}
	if mc.Expiration > 0 {
		entry.expiresAt = time.Now().Add(mc.Expiration)
	}
	mc.keyVals[key.String()] = entry

	return nil
}

func (mc *MemoryCache) Purge(ctx context.Context, key *cache.Key) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	mc.Lock()
	defer mc.Unlock()

	delete(mc.keyVals, key.String())

	return nil
}

func clone(value []byte) []byte {
	if value == nil {
		return nil
	}
	cloned := make([]byte, len(value))
	copy(cloned, value)
	return cloned
}
