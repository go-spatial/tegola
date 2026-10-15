package prometheus

import (
	"context"
	"errors"
	"testing"

	tegolaCache "github.com/alexeydott/tegola/cache"
	"github.com/prometheus/client_golang/prometheus"
)

// failingPurgeCache is a stub cache whose Purge always fails.
type failingPurgeCache struct {
	purgeErr error
}

func (c *failingPurgeCache) Get(context.Context, *tegolaCache.Key) ([]byte, bool, error) {
	return nil, false, nil
}

func (c *failingPurgeCache) Set(context.Context, *tegolaCache.Key, []byte) error {
	return nil
}

func (c *failingPurgeCache) Purge(context.Context, *tegolaCache.Key) error {
	return c.purgeErr
}

// TestPurgePropagatesWrappedError guards the prometheus cache wrapper:
// Purge must return the wrapped cache's error instead of swallowing it.
func TestPurgePropagatesWrappedError(t *testing.T) {
	wantErr := errors.New("purge failed")
	stub := &failingPurgeCache{purgeErr: wantErr}

	// a private registry keeps this test independent of global registration
	co := newCache(prometheus.NewRegistry(), "test_cache", nil, stub)

	key := tegolaCache.Key{MapName: "test-map", LayerName: "test-layer", Z: 4, X: 2, Y: 3}
	err := co.Purge(context.Background(), &key)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Purge error = %v, want %v (must not be swallowed)", err, wantErr)
	}
}
