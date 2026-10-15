package multilevel

import (
	"context"
	"errors"
	"fmt"

	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/cache/file"
	"github.com/alexeydott/tegola/cache/memory"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/log"
)

const CacheType = "multilevel"

const (
	ConfigKeyMemory = "memory"
	ConfigKeyFile   = "file"
)

var (
	ErrMemoryConfigMissing = errors.New("multilevel cache: memory configuration is required")
	ErrFileConfigMissing   = errors.New("multilevel cache: file configuration is required")
)

func init() {
	_ = cache.Register(CacheType, New)
}

// New creates a two-level cache. The memory backend is the L1 cache and the
// file backend is the L2 cache. Each backend receives its own configuration,
// including independent max_zoom and ttl values.
func New(config dict.Dicter) (cache.Interface, error) {
	if config == nil {
		return nil, errors.New("multilevel cache: configuration is required")
	}

	memoryConfig, err := requiredConfig(config, ConfigKeyMemory, ErrMemoryConfigMissing)
	if err != nil {
		return nil, err
	}
	fileConfig, err := requiredConfig(config, ConfigKeyFile, ErrFileConfigMissing)
	if err != nil {
		return nil, err
	}

	l1, err := memory.New(memoryConfig)
	if err != nil {
		return nil, fmt.Errorf("multilevel cache: configure memory backend: %w", err)
	}
	l2, err := file.New(fileConfig)
	if err != nil {
		return nil, fmt.Errorf("multilevel cache: configure file backend: %w", err)
	}

	return &Cache{memory: l1, file: l2}, nil
}

func requiredConfig(config dict.Dicter, key string, missing error) (dict.Dicter, error) {
	if _, ok := config.Interface(key); !ok {
		return nil, missing
	}

	subconfig, err := config.Map(key)
	if err != nil {
		return nil, fmt.Errorf("multilevel cache: invalid %s configuration: %w", key, err)
	}
	return subconfig, nil
}

// Cache implements an L1 memory and L2 file tile cache.
type Cache struct {
	memory cache.Interface
	file   cache.Interface
}

// Get checks memory first and falls back to file on a memory miss or a
// recoverable memory error. A file hit is promoted to memory. Promotion is
// best-effort: the file hit remains a valid cache response if L1 promotion
// fails, but the failure is logged.
func (c *Cache) Get(ctx context.Context, key *cache.Key) ([]byte, bool, error) {
	value, hit, err := c.memory.Get(ctx, key)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, false, fmt.Errorf("multilevel cache: memory get: %w", err)
		}
		log.Warnf("multilevel cache: memory get failed, falling back to file: %v", err)
	}
	if err == nil && hit {
		return value, true, nil
	}

	fileValue, fileHit, fileErr := c.file.Get(ctx, key)
	if fileErr != nil {
		if err != nil {
			return nil, false, errors.Join(
				fmt.Errorf("multilevel cache: memory get: %w", err),
				fmt.Errorf("multilevel cache: file get: %w", fileErr),
			)
		}
		return nil, false, fmt.Errorf("multilevel cache: file get: %w", fileErr)
	}
	if !fileHit {
		return nil, false, nil
	}

	if err := c.memory.Set(ctx, key, fileValue); err != nil {
		log.Warnf("multilevel cache: promoting %s from file to memory failed: %v", key.String(), err)
	}
	return fileValue, true, nil
}

// Set writes to both levels. Both writes are attempted so a transient failure
// in one backend does not prevent the other level from being refreshed.
func (c *Cache) Set(ctx context.Context, key *cache.Key, value []byte) error {
	var errs []error
	if err := c.memory.Set(ctx, key, value); err != nil {
		errs = append(errs, fmt.Errorf("memory set: %w", err))
	}
	if err := c.file.Set(ctx, key, value); err != nil {
		errs = append(errs, fmt.Errorf("file set: %w", err))
	}
	return errors.Join(errs...)
}

// Purge removes the tile from both levels. Both purges are attempted and all
// failures are returned.
func (c *Cache) Purge(ctx context.Context, key *cache.Key) error {
	var errs []error
	if err := c.memory.Purge(ctx, key); err != nil {
		errs = append(errs, fmt.Errorf("memory purge: %w", err))
	}
	if err := c.file.Purge(ctx, key); err != nil {
		errs = append(errs, fmt.Errorf("file purge: %w", err))
	}
	return errors.Join(errs...)
}
