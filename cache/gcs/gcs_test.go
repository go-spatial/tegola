package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"cloud.google.com/go/storage"

	"github.com/alexeydott/tegola/cache"
)

// errTransient simulates a backend failure (e.g. a network blip or a 5xx).
var errTransient = errors.New("gcs: transient backend failure")

func testKey() *cache.Key {
	return &cache.Key{MapName: "test-map", LayerName: "test-layer", Z: 0, X: 0, Y: 0}
}

// A transient backend error must be surfaced as an error, not swallowed into a
// cache miss (regression test for upstream go-spatial/tegola#938).
func TestGetTransientErrorIsNotAMiss(t *testing.T) {
	c := &GCSCache{
		newReader: func(ctx context.Context, key string) (io.ReadCloser, error) {
			return nil, errTransient
		},
	}

	val, hit, err := c.Get(context.Background(), testKey())
	if err == nil {
		t.Fatal("expected a backend error to be returned, got nil (error swallowed as a cache miss)")
	}
	if !errors.Is(err, errTransient) {
		t.Errorf("expected the backend error to propagate, got %v", err)
	}
	if hit {
		t.Error("expected hit=false on a backend error")
	}
	if val != nil {
		t.Errorf("expected nil value on a backend error, got %v", val)
	}
}

// A missing object must yield a clean, error-free miss.
func TestGetObjectNotExistIsACleanMiss(t *testing.T) {
	for name, readErr := range map[string]error{
		"bare":    storage.ErrObjectNotExist,
		"wrapped": fmt.Errorf("gcs read: %w", storage.ErrObjectNotExist),
	} {
		t.Run(name, func(t *testing.T) {
			c := &GCSCache{
				newReader: func(ctx context.Context, key string) (io.ReadCloser, error) {
					return nil, readErr
				},
			}

			val, hit, err := c.Get(context.Background(), testKey())
			if err != nil {
				t.Fatalf("a missing object must be a clean miss, got error %v", err)
			}
			if hit {
				t.Error("expected hit=false for a missing object")
			}
			if val != nil {
				t.Errorf("expected nil value for a missing object, got %v", val)
			}
		})
	}
}

// Sanity check that a successful read still returns the payload as a hit.
func TestGetSuccess(t *testing.T) {
	const want = "tile-bytes"
	c := &GCSCache{
		newReader: func(ctx context.Context, key string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(want)), nil
		},
	}

	val, hit, err := c.Get(context.Background(), testKey())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hit {
		t.Fatal("expected hit=true on a successful read")
	}
	if string(val) != want {
		t.Errorf("value mismatch: got %q, want %q", val, want)
	}
}

// fakeCache drives the New() self-test with configurable outcomes.
type fakeCache struct {
	data       []byte
	getData    []byte // what Get reports (defaults to data when nil)
	hit        bool   // what Get reports
	getErr     error
	setErr     error
	purgeErr   error
	setCalls   int
	purgeCalls int
}

func (f *fakeCache) Set(ctx context.Context, key *cache.Key, val []byte) error {
	f.setCalls++
	if f.setErr != nil {
		return f.setErr
	}
	f.data = append([]byte(nil), val...)
	return nil
}

func (f *fakeCache) Get(ctx context.Context, key *cache.Key) ([]byte, bool, error) {
	if f.getErr != nil {
		return nil, false, f.getErr
	}
	if f.getData != nil {
		return f.getData, f.hit, nil
	}
	return f.data, f.hit, nil
}

func (f *fakeCache) Purge(ctx context.Context, key *cache.Key) error {
	f.purgeCalls++
	return f.purgeErr
}

// The cache self-test must require an actual cache hit on read: a silent
// cache miss (no error, hit=false) proves the cache is not usable and must
// fail New().
func TestSelfTestCacheRequiresHit(t *testing.T) {
	f := &fakeCache{hit: false}
	err := selfTestCache(context.Background(), f, testKey(), []byte("test"))
	if err == nil {
		t.Fatal("selfTestCache must fail when the read does not hit the cache")
	}
	if !strings.Contains(err.Error(), "did not hit") {
		t.Errorf("unexpected error: %v", err)
	}
	if f.setCalls != 1 {
		t.Errorf("Set() calls = %d, want 1", f.setCalls)
	}
	if f.purgeCalls != 0 {
		t.Errorf("Purge() calls = %d, want 0 (the test object must not be purged on failure)", f.purgeCalls)
	}
}

// A read that returns different bytes than were written must also fail.
func TestSelfTestCacheRequiresIdenticalData(t *testing.T) {
	f := &fakeCache{hit: true, getData: []byte("different")}

	err := selfTestCache(context.Background(), f, testKey(), []byte("written"))
	if err == nil {
		t.Fatal("selfTestCache must fail when the read returns different data")
	}
	if !strings.Contains(err.Error(), "different data") {
		t.Errorf("unexpected error: %v", err)
	}
}

// Get errors must propagate.
func TestSelfTestCachePropagatesGetError(t *testing.T) {
	f := &fakeCache{hit: true, getErr: errTransient}

	err := selfTestCache(context.Background(), f, testKey(), []byte("test"))
	if err == nil {
		t.Fatal("selfTestCache must fail when the read errors")
	}
	if !strings.Contains(err.Error(), errTransient.Error()) {
		t.Errorf("expected the read error to propagate, got %v", err)
	}
}

// The happy path must write, read back as a hit with identical bytes, and
// purge the test object.
func TestSelfTestCacheSuccess(t *testing.T) {
	f := &fakeCache{hit: true}

	if err := selfTestCache(context.Background(), f, testKey(), []byte("test")); err != nil {
		t.Fatalf("selfTestCache() error = %v", err)
	}
	if f.setCalls != 1 {
		t.Errorf("Set() calls = %d, want 1", f.setCalls)
	}
	if f.purgeCalls != 1 {
		t.Errorf("Purge() calls = %d, want 1", f.purgeCalls)
	}
}
