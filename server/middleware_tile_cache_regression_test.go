package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alexeydott/geom/encoding/mvt"
	"github.com/alexeydott/tegola/cache"
)

func TestRenderTileForCacheCachesImplicitSuccessfulWrite(t *testing.T) {
	cacher := newFakeTileCache()
	key := &cache.Key{MapName: "m", LayerName: "l", Z: 4, X: 3, Y: 2}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", mvt.MimeType)
		// no explicit WriteHeader: net/http semantics imply 200
		_, _ = w.Write([]byte("tile"))
	})

	req := httptest.NewRequest(http.MethodGet, "/maps/m/l/4/3/2", nil)
	res := renderTileForCache(req.Context(), req, handler, cacher, key, true)

	if res.status != http.StatusOK {
		t.Fatalf("rendered status = %d, want 200", res.status)
	}
	if !bytes.Equal(res.body, []byte("tile")) {
		t.Fatalf("rendered body = %q, want %q", res.body, "tile")
	}
	if got := cacher.setCount(key.String()); got != 1 {
		t.Fatalf("cacher.Set calls = %d, want 1", got)
	}
}

func TestTileUpdateCoordinatorSerializesMetatileWork(t *testing.T) {
	coordinator := newTileUpdateCoordinator()
	const lockKey = "map/layer/4/8/8"

	state, unlock, err := coordinator.acquire(context.Background(), lockKey)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer coordinator.release(lockKey, state)

	acquired := make(chan struct{})
	done := make(chan struct{})
	go func() {
		otherState, otherUnlock, err := coordinator.acquire(context.Background(), lockKey)
		if err != nil {
			t.Errorf("second acquire: %v", err)
			close(done)
			return
		}
		defer coordinator.release(lockKey, otherState)
		close(acquired)
		otherUnlock()
		close(done)
	}()

	select {
	case <-acquired:
		t.Fatal("same-metatile work acquired the lock concurrently")
	case <-time.After(20 * time.Millisecond):
	}

	unlock()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("same-metatile work did not acquire the lock after release")
	}
}

func TestTileUpdateCoordinatorAcquireHonorsContextCancel(t *testing.T) {
	coordinator := newTileUpdateCoordinator()
	const lockKey = "map/layer/4/8/8"

	state, unlock, err := coordinator.acquire(context.Background(), lockKey)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer coordinator.release(lockKey, state)
	defer unlock()

	// a waiter whose context is canceled must leave the queue immediately
	// instead of blocking until the lock is free
	ctx, cancel := context.WithCancel(context.Background())
	waitErr := make(chan error, 1)
	go func() {
		waitState, waitUnlock, err := coordinator.acquire(ctx, lockKey)
		if err == nil {
			defer coordinator.release(lockKey, waitState)
			waitUnlock()
		}
		waitErr <- err
	}()

	cancel()

	select {
	case err := <-waitErr:
		if err == nil {
			t.Fatal("canceled acquire returned nil error")
		}
	case <-time.After(time.Second):
		t.Fatal("canceled acquire stayed queued")
	}
}

// blockingSetCache wraps a cache.Interface and stalls the first Set call after
// arm until the gate opens, modeling a cache write that is delayed in flight.
type blockingSetCache struct {
	cache.Interface
	mu         sync.Mutex
	armed      bool
	setEntered chan struct{}
	gate       chan struct{}
}

func newBlockingSetCache(under cache.Interface) *blockingSetCache {
	return &blockingSetCache{
		Interface:  under,
		setEntered: make(chan struct{}),
		gate:       make(chan struct{}),
	}
}

func (b *blockingSetCache) arm() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.armed = true
}

func (b *blockingSetCache) Set(ctx context.Context, key *cache.Key, val []byte) error {
	b.mu.Lock()
	block := b.armed
	b.armed = false
	b.mu.Unlock()
	if block {
		b.setEntered <- struct{}{}
		select {
		case <-b.gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return b.Interface.Set(ctx, key, val)
}

// TestStaleRenderWriteCannotClobberNewerGeneration reproduces the P5-2 race
// deterministically through a controlled seam: a miss render whose cache
// write is delayed in flight (after its generation check) while a metatile
// mutation regenerates the metatile. The stale render's delayed write must
// never land on top of the newer generation's tiles.
func TestStaleRenderWriteCannotClobberNewerGeneration(t *testing.T) {
	under := newFakeTileCache()
	cacher := newBlockingSetCache(under)
	cacher.arm()
	key := &cache.Key{MapName: "m", LayerName: "l", Z: 1, X: 0, Y: 0}
	lockKey := metatileLockKeyForCacheKey(key)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", mvt.MimeType)
		_, _ = w.Write([]byte("stale"))
	})

	renderDone := make(chan struct{})
	go func() {
		defer close(renderDone)
		req := httptest.NewRequest(http.MethodGet, "/maps/m/l/1/0/0", nil)
		renderTileForCache(context.Background(), req, handler, cacher, key, true)
	}()

	// the stale render has passed its generation check and its cache write is
	// now delayed in flight
	<-cacher.setEntered

	// a metatile mutation ( ?tile=update / ?dirty contract ) regenerates the
	// metatile now and writes the fresh tile
	mutationDone := make(chan struct{})
	go func() {
		defer close(mutationDone)
		ctx := context.Background()
		state, unlock, err := tileUpdateLocks.acquire(ctx, lockKey)
		if err != nil {
			t.Error(err)
			return
		}
		defer unlock()
		tileUpdateLocks.beginRegeneration(state)
		defer tileUpdateLocks.endRegeneration(state)
		if err := cacher.Set(ctx, key, []byte("fresh")); err != nil {
			t.Error(err)
		}
	}()

	// give the mutation time to run: without the atomic write claim it
	// completes and stores "fresh" while the stale write is still in flight
	time.Sleep(100 * time.Millisecond)

	// release the delayed stale write and let everything settle
	close(cacher.gate)
	<-renderDone
	<-mutationDone

	cached, hit, err := under.Get(context.Background(), key)
	if err != nil || !hit {
		t.Fatalf("Get = %q, %v, %v; want cached tile", cached, hit, err)
	}
	if !bytes.Equal(cached, []byte("fresh")) {
		t.Fatalf("cached tile = %q, want %q (the newer generation's write must win)", cached, "fresh")
	}
}
