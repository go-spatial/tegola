package multilevel

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/env"
)

type fakeCache struct {
	value       []byte
	hit         bool
	getErr      error
	setErr      error
	purgeErr    error
	getCalls    int
	setCalls    int
	purgeCalls  int
	lastSetData []byte
}

func (c *fakeCache) Get(context.Context, *cache.Key) ([]byte, bool, error) {
	c.getCalls++
	return c.value, c.hit, c.getErr
}

func (c *fakeCache) Set(_ context.Context, _ *cache.Key, value []byte) error {
	c.setCalls++
	c.lastSetData = append([]byte(nil), value...)
	return c.setErr
}

func (c *fakeCache) Purge(context.Context, *cache.Key) error {
	c.purgeCalls++
	return c.purgeErr
}

func TestGetUsesMemoryBeforeFile(t *testing.T) {
	memory := &fakeCache{value: []byte("memory"), hit: true}
	file := &fakeCache{value: []byte("file"), hit: true}
	c := &Cache{memory: memory, file: file}

	value, hit, err := c.Get(context.Background(), &cache.Key{Z: 1, X: 1, Y: 1})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !hit || !reflect.DeepEqual(value, []byte("memory")) {
		t.Fatalf("Get() = (%q, %v), want (%q, true)", value, hit, "memory")
	}
	if file.getCalls != 0 {
		t.Fatalf("file backend received %d Get calls, want 0", file.getCalls)
	}
}

func TestGetPromotesFileHit(t *testing.T) {
	memory := &fakeCache{}
	file := &fakeCache{value: []byte("file"), hit: true}
	c := &Cache{memory: memory, file: file}

	value, hit, err := c.Get(context.Background(), &cache.Key{Z: 1, X: 1, Y: 1})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !hit || !reflect.DeepEqual(value, []byte("file")) {
		t.Fatalf("Get() = (%q, %v), want (%q, true)", value, hit, "file")
	}
	if memory.getCalls != 1 || file.getCalls != 1 {
		t.Fatalf("Get() calls = (%d, %d), want (1, 1)", memory.getCalls, file.getCalls)
	}
	if memory.setCalls != 1 || !reflect.DeepEqual(memory.lastSetData, []byte("file")) {
		t.Fatalf("memory promotion = (%d, %q), want (1, %q)", memory.setCalls, memory.lastSetData, "file")
	}
}

func TestGetFallsBackToFileOnMemoryError(t *testing.T) {
	memoryErr := errors.New("memory unavailable")
	memory := &fakeCache{getErr: memoryErr}
	file := &fakeCache{value: []byte("file"), hit: true}
	c := &Cache{memory: memory, file: file}

	value, hit, err := c.Get(context.Background(), &cache.Key{Z: 1, X: 1, Y: 1})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !hit || !reflect.DeepEqual(value, []byte("file")) {
		t.Fatalf("Get() = (%q, %v), want (%q, true)", value, hit, "file")
	}
	if memory.getCalls != 1 || file.getCalls != 1 {
		t.Fatalf("Get() calls = (%d, %d), want (1, 1)", memory.getCalls, file.getCalls)
	}
}

func TestGetReturnsBothBackendErrors(t *testing.T) {
	memoryErr := errors.New("memory unavailable")
	fileErr := errors.New("file unavailable")
	memory := &fakeCache{getErr: memoryErr}
	file := &fakeCache{getErr: fileErr}
	c := &Cache{memory: memory, file: file}

	_, hit, err := c.Get(context.Background(), &cache.Key{Z: 1, X: 1, Y: 1})
	if hit {
		t.Fatal("Get() reported a hit for two backend errors")
	}
	if !errors.Is(err, memoryErr) || !errors.Is(err, fileErr) {
		t.Fatalf("Get() error = %v, want both backend errors", err)
	}
}

func TestGetDoesNotFallbackAfterContextCancellation(t *testing.T) {
	memory := &fakeCache{getErr: context.Canceled}
	file := &fakeCache{value: []byte("file"), hit: true}
	c := &Cache{memory: memory, file: file}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, hit, err := c.Get(ctx, &cache.Key{Z: 1, X: 1, Y: 1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Get() error = %v, want context.Canceled", err)
	}
	if hit {
		t.Fatal("Get() reported a hit after context cancellation")
	}
	if file.getCalls != 0 {
		t.Fatalf("file backend received %d Get calls, want 0", file.getCalls)
	}
}

func TestGetPromotionFailureKeepsFileHit(t *testing.T) {
	promotionErr := errors.New("memory unavailable")
	memory := &fakeCache{setErr: promotionErr}
	file := &fakeCache{value: []byte("file"), hit: true}
	c := &Cache{memory: memory, file: file}

	value, hit, err := c.Get(context.Background(), &cache.Key{Z: 1, X: 1, Y: 1})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !hit || !reflect.DeepEqual(value, []byte("file")) {
		t.Fatalf("Get() = (%q, %v), want (%q, true)", value, hit, "file")
	}
}

func TestSetAndPurgeUseBothLevels(t *testing.T) {
	memoryErr := errors.New("memory set failed")
	fileErr := errors.New("file purge failed")
	memory := &fakeCache{setErr: memoryErr}
	file := &fakeCache{purgeErr: fileErr}
	c := &Cache{memory: memory, file: file}
	key := &cache.Key{Z: 1, X: 1, Y: 1}

	if err := c.Set(context.Background(), key, []byte("tile")); !errors.Is(err, memoryErr) {
		t.Fatalf("Set() error = %v, want memory error", err)
	}
	if memory.setCalls != 1 || file.setCalls != 1 {
		t.Fatalf("Set() calls = (%d, %d), want (1, 1)", memory.setCalls, file.setCalls)
	}

	if err := c.Purge(context.Background(), key); !errors.Is(err, fileErr) {
		t.Fatalf("Purge() error = %v, want file error", err)
	}
	if memory.purgeCalls != 1 || file.purgeCalls != 1 {
		t.Fatalf("Purge() calls = (%d, %d), want (1, 1)", memory.purgeCalls, file.purgeCalls)
	}
}

func TestSetAndPurgeJoinBothBackendErrors(t *testing.T) {
	memorySetErr := errors.New("memory set failed")
	fileSetErr := errors.New("file set failed")
	memoryPurgeErr := errors.New("memory purge failed")
	filePurgeErr := errors.New("file purge failed")
	memory := &fakeCache{setErr: memorySetErr, purgeErr: memoryPurgeErr}
	file := &fakeCache{setErr: fileSetErr, purgeErr: filePurgeErr}
	c := &Cache{memory: memory, file: file}
	key := &cache.Key{Z: 1, X: 1, Y: 1}

	err := c.Set(context.Background(), key, []byte("tile"))
	if !errors.Is(err, memorySetErr) || !errors.Is(err, fileSetErr) {
		t.Fatalf("Set() error = %v, want both backend errors", err)
	}

	err = c.Purge(context.Background(), key)
	if !errors.Is(err, memoryPurgeErr) || !errors.Is(err, filePurgeErr) {
		t.Fatalf("Purge() error = %v, want both backend errors", err)
	}
}

func TestNewRequiresNestedBackends(t *testing.T) {
	if _, err := New(dict.Dict{}); !errors.Is(err, ErrMemoryConfigMissing) {
		t.Fatalf("New() error = %v, want %v", err, ErrMemoryConfigMissing)
	}
	if _, err := New(dict.Dict{ConfigKeyMemory: dict.Dict{}}); !errors.Is(err, ErrFileConfigMissing) {
		t.Fatalf("New() error = %v, want %v", err, ErrFileConfigMissing)
	}
}

func TestNewWithMemoryAndFileBackends(t *testing.T) {
	c, err := New(dict.Dict{
		ConfigKeyMemory: dict.Dict{},
		ConfigKeyFile: dict.Dict{
			"basepath": t.TempDir(),
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	key := &cache.Key{MapName: "map", LayerName: "layer", Z: 1, X: 2, Y: 3}
	if err := c.Set(context.Background(), key, []byte("tile")); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if value, hit, err := c.Get(context.Background(), key); err != nil {
		t.Fatalf("Get() error = %v", err)
	} else if !hit || !reflect.DeepEqual(value, []byte("tile")) {
		t.Fatalf("Get() = (%q, %v), want (%q, true)", value, hit, "tile")
	}
}

func TestNewWithConfigEnvironmentDict(t *testing.T) {
	c, err := New(env.Dict{
		ConfigKeyMemory: env.Dict{
			"max_zoom": uint(18),
			"ttl":      60,
		},
		ConfigKeyFile: env.Dict{
			"basepath": t.TempDir(),
			"max_zoom": uint(22),
			"ttl":      86400,
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	key := &cache.Key{MapName: "map", LayerName: "layer", Z: 1, X: 2, Y: 3}
	if err := c.Set(context.Background(), key, []byte("tile")); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if value, hit, err := c.Get(context.Background(), key); err != nil {
		t.Fatalf("Get() error = %v", err)
	} else if !hit || !reflect.DeepEqual(value, []byte("tile")) {
		t.Fatalf("Get() = (%q, %v), want (%q, true)", value, hit, "tile")
	}
}
