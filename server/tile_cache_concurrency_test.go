package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/test"
	"github.com/alexeydott/tegola/server"
)

// blockingTiler wraps the test tile provider and can hold the first N
// TileFeatures calls inside the provider, so tests can pause a render at a
// well defined point.
type blockingTiler struct {
	test.TileProvider

	mu        sync.Mutex
	blockLeft int
	calls     int
	entered   chan struct{}
	release   chan struct{}
}

func newBlockingTiler(blockFirst int) *blockingTiler {
	return &blockingTiler{
		blockLeft: blockFirst,
		entered:   make(chan struct{}, 1),
		release:   make(chan struct{}),
	}
}

func (b *blockingTiler) TileFeatures(ctx context.Context, layer string, t provider.Tile, params provider.Params, fn func(f *provider.Feature) error) error {
	b.mu.Lock()
	b.calls++
	block := b.blockLeft > 0
	if block {
		b.blockLeft--
	}
	b.mu.Unlock()

	if block {
		select {
		case b.entered <- struct{}{}:
		default:
		}
		select {
		case <-b.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return b.TileProvider.TileFeatures(ctx, layer, t, params, fn)
}

func (b *blockingTiler) callCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.calls
}

// blockingResponseWriter signals every Write call and then blocks until
// released, so concurrent response writes can be observed to overlap.
type blockingResponseWriter struct {
	header  http.Header
	code    int
	arrived chan struct{}
	release <-chan struct{}
}

func (w *blockingResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}

	return w.header
}

func (w *blockingResponseWriter) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
}

func (w *blockingResponseWriter) Write(b []byte) (int, error) {
	w.arrived <- struct{}{}
	<-w.release

	return len(b), nil
}

type tileOpStatus struct {
	Cached   bool `json:"cached"`
	Updating bool `json:"updating"`
}

func fetchTileOpStatus(t *testing.T, router http.Handler, uri string) tileOpStatus {
	t.Helper()

	w := tileOpsRequest(router, uri, testTileOpsToken)
	if w.Code != http.StatusOK {
		t.Fatalf("status request: status = %d, want 200 (%s)", w.Code, w.Body.String())
	}

	var status tileOpStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("status response is not valid JSON: %v (%s)", err, w.Body.String())
	}

	return status
}

// TestTileCacheHitsServedConcurrently guards against serializing cache hits on
// the metatile lock: four hits for the same cached tile must reach the
// response writer at the same time.
func TestTileCacheHitsServedConcurrently(t *testing.T) {
	a := newTestMapWithLayers(testLayer1)
	_, router := newTileOpsTestServer(t, a)

	base := "/maps/test-map/test-layer/4/2/3.pbf"

	// warm the cache so the requests below are all hits
	w := tileOpsRequest(router, base, "")
	if w.Code != http.StatusOK {
		t.Fatalf("warmup request: status = %d, want 200 (%s)", w.Code, w.Body.String())
	}

	const workers = 4
	arrived := make(chan struct{}, workers*2)
	release := make(chan struct{})

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			r := httptest.NewRequest(http.MethodGet, base, nil)
			// a gzip accepting client receives the cached bytes directly,
			// so Write is called while the handler is still running
			r.Header.Set("Accept-Encoding", "gzip")
			cw := &blockingResponseWriter{arrived: arrived, release: release}
			router.ServeHTTP(cw, r)

			if cw.code != http.StatusOK {
				t.Errorf("hit status = %d, want 200", cw.code)
			}
		}()
	}

	for i := 0; i < workers; i++ {
		select {
		case <-arrived:
		case <-time.After(5 * time.Second):
			close(release)
			wg.Wait()
			t.Fatalf("cache hits are serialized: only %d of %d response writes overlapped", i, workers)
		}
	}

	close(release)
	wg.Wait()
}

// TestTileCacheConcurrentMissesShareOneRender guards the singleflight miss
// path: concurrent misses for one tile must trigger exactly one provider
// render and one cache write.
func TestTileCacheConcurrentMissesShareOneRender(t *testing.T) {
	tiler := newBlockingTiler(1)
	layer := testLayer1
	layer.Provider = tiler

	a := newTestMapWithLayers(layer)
	cacher, router := newTileOpsTestServer(t, a)

	base := "/maps/test-map/test-layer/4/2/3.pbf"

	const workers = 5
	var wg sync.WaitGroup
	codes := make([]int, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			w := tileOpsRequest(router, base, "")
			codes[idx] = w.Code
		}(i)
	}

	// let all workers pile up behind the shared render
	<-tiler.entered
	time.Sleep(100 * time.Millisecond)
	close(tiler.release)
	wg.Wait()

	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i, code)
		}
	}
	if got := tiler.callCount(); got != 1 {
		t.Errorf("provider renders = %d, want 1 (concurrent misses must share one render)", got)
	}
	if got := cacher.setCount(); got != 1 {
		t.Errorf("cache writes = %d, want 1", got)
	}
}

// TestTileCacheWaiterGetsSharedCacheHeader guards cache header reporting
// across a shared render: the request that owns the render reports MISS,
// while requests that join an already in-flight render report SHARED, so
// instrumentation can tell a real cache miss from a coalesced wait.
func TestTileCacheWaiterGetsSharedCacheHeader(t *testing.T) {
	tiler := newBlockingTiler(1)
	layer := testLayer1
	layer.Provider = tiler

	a := newTestMapWithLayers(layer)
	_, router := newTileOpsTestServer(t, a)

	base := "/maps/test-map/test-layer/4/2/3.pbf"

	type result struct {
		code     int
		cacheHdr string
	}
	leaderCh := make(chan result, 1)
	waiterCh := make(chan result, 1)

	// the leader request owns the render
	go func() {
		w := tileOpsRequest(router, base, "")
		leaderCh <- result{code: w.Code, cacheHdr: w.Header().Get("Tegola-Cache")}
	}()

	// the render is now paused inside the provider; a second request joins it
	<-tiler.entered
	go func() {
		w := tileOpsRequest(router, base, "")
		waiterCh <- result{code: w.Code, cacheHdr: w.Header().Get("Tegola-Cache")}
	}()

	// give the waiter time to join the in-flight render
	time.Sleep(100 * time.Millisecond)
	close(tiler.release)

	leader := <-leaderCh
	waiter := <-waiterCh

	if leader.code != http.StatusOK {
		t.Errorf("leader: status = %d, want 200", leader.code)
	}
	if waiter.code != http.StatusOK {
		t.Errorf("waiter: status = %d, want 200", waiter.code)
	}
	if leader.cacheHdr != "MISS" {
		t.Errorf("leader Tegola-Cache = %q, want MISS", leader.cacheHdr)
	}
	if waiter.cacheHdr != "SHARED" {
		t.Errorf("waiter Tegola-Cache = %q, want SHARED (waiters must not inherit the leader's MISS)", waiter.cacheHdr)
	}
	if got := tiler.callCount(); got != 1 {
		t.Errorf("provider renders = %d, want 1 (waiter must join the in-flight render)", got)
	}
}

// TestTileUpdateNotOverwrittenByStaleMissRender guards the metatile
// generation check: a miss render that is in flight while ?tile=update
// regenerates the metatile must not write its stale bytes afterwards.
func TestTileUpdateNotOverwrittenByStaleMissRender(t *testing.T) {
	tiler := newBlockingTiler(1)
	layer := testLayer1
	layer.Provider = tiler

	a := newTestMapWithLayers(layer)
	cacher, router := newTileOpsTestServer(t, a)
	enableTileOperations(t, serverTileOpsTestConfig())

	base := "/maps/test-map/test-layer/4/2/3.pbf"

	type result struct {
		code     int
		cacheHdr string
	}
	missDone := make(chan result, 1)
	go func() {
		w := tileOpsRequest(router, base, "")
		missDone <- result{code: w.Code, cacheHdr: w.Header().Get("Tegola-Cache")}
	}()

	// the miss render is now paused inside the provider
	<-tiler.entered

	// regenerate the metatile while the miss render is still in flight; the
	// regeneration runs in the background and must not wait for the held miss
	// render
	w := tileOpsRequest(router, base+"?tile=update", testTileOpsToken)
	if w.Code != http.StatusAccepted {
		t.Fatalf("update: status = %d, want 202 (%s)", w.Code, w.Body.String())
	}

	// wait for the regeneration to finish BEFORE releasing the miss render:
	// the regeneration bumps the metatile generation, so the stale render's
	// write is skipped once it completes
	server.AwaitMetatileRegeneration()

	// let the stale render finish; its result must not overwrite the update
	close(tiler.release)
	res := <-missDone
	if res.code != http.StatusOK {
		t.Errorf("miss request: status = %d, want 200", res.code)
	}
	if res.cacheHdr != "MISS" {
		t.Errorf("miss Tegola-Cache = %q, want MISS", res.cacheHdr)
	}

	key := cache.Key{MapName: "test-map", LayerName: "test-layer", Z: 4, X: 2, Y: 3}
	if got := cacher.keyCount(key); got != 1 {
		t.Errorf("tile written %d times, want 1 (?tile=update write only; the stale miss render must be skipped)", got)
	}
}

// TestTileStatusUpdatingOnlyDuringMutatingOperations guards the N3 status
// semantics: only in-flight ?tile=update style operations may report
// updating=true; ordinary render requests must not.
func TestTileStatusUpdatingOnlyDuringMutatingOperations(t *testing.T) {
	tiler := newBlockingTiler(1)
	layer := testLayer1
	layer.Provider = tiler

	a := newTestMapWithLayers(layer)
	_, router := newTileOpsTestServer(t, a)
	enableTileOperations(t, serverTileOpsTestConfig())

	base := "/maps/test-map/test-layer/4/2/3.pbf"

	// an ordinary in-flight render must not report updating
	missDone := make(chan int, 1)
	go func() {
		w := tileOpsRequest(router, base, "")
		missDone <- w.Code
	}()
	<-tiler.entered

	if status := fetchTileOpStatus(t, router, base+"?tile=status"); status.Updating {
		t.Error("ordinary request in flight must not report updating=true")
	}

	close(tiler.release)
	if code := <-missDone; code != http.StatusOK {
		t.Errorf("ordinary request: status = %d, want 200", code)
	}

	// an in-flight ?tile=update must report updating
	tiler2 := newBlockingTiler(1)
	layer2 := testLayer1
	layer2.Provider = tiler2

	a2 := newTestMapWithLayers(layer2)
	_, router2 := newTileOpsTestServer(t, a2)
	enableTileOperations(t, serverTileOpsTestConfig())

	updateDone := make(chan int, 1)
	go func() {
		w := tileOpsRequest(router2, base+"?tile=update", testTileOpsToken)
		updateDone <- w.Code
	}()
	// the metatile regeneration is now rendering in the background (the
	// update counter was already taken)
	<-tiler2.entered

	if status := fetchTileOpStatus(t, router2, base+"?tile=status"); !status.Updating {
		t.Error("?tile=update in flight must report updating=true")
	}

	// the request itself is answered while the regeneration still runs
	if code := <-updateDone; code != http.StatusAccepted {
		t.Errorf("?tile=update: status = %d, want 202", code)
	}

	close(tiler2.release)
	server.AwaitMetatileRegeneration()

	// once finished the flag must fall back to false
	if status := fetchTileOpStatus(t, router2, base+"?tile=status"); status.Updating {
		t.Error("updating must be false after ?tile=update finished")
	}
	if status := fetchTileOpStatus(t, router2, base+"?tile=status"); !status.Cached {
		t.Error("tile must be cached after ?tile=update")
	}
}

// TestTileUpdateSkipsOutOfBoundsMetatileTiles guards the N4 bounds filter:
// metatile tiles outside the map bounds must neither be rendered nor cached.
func TestTileUpdateSkipsOutOfBoundsMetatileTiles(t *testing.T) {
	// map bounds (0,0)-(10,10) in 4326; at z4 tile 8/7 covers lon [0,22.5] x
	// lat [0,21.94] and intersects the bounds, while all other tiles of its
	// metatile x[8..15] y[0..7] fall outside
	a := newTestMapWithBounds(0, 0, 10, 10)
	cacher, router := newTileOpsTestServer(t, a)
	enableTileOperations(t, serverTileOpsTestConfig())

	w := tileOpsRequest(router, "/maps/test-map/test-layer/4/8/7.pbf?tile=update", testTileOpsToken)
	if w.Code != http.StatusAccepted {
		t.Fatalf("update: status = %d, want 202 (%s)", w.Code, w.Body.String())
	}
	server.AwaitMetatileRegeneration()

	// the requested tile is within bounds and must be cached exactly once
	inKey := cache.Key{MapName: "test-map", LayerName: "test-layer", Z: 4, X: 8, Y: 7}
	if got := cacher.keyCount(inKey); got != 1 {
		t.Errorf("in-bounds tile written %d times, want 1", got)
	}

	// metatile tiles outside the map bounds must not be cached at all
	for _, key := range []cache.Key{
		{MapName: "test-map", LayerName: "test-layer", Z: 4, X: 8, Y: 0},  // north of bounds
		{MapName: "test-map", LayerName: "test-layer", Z: 4, X: 9, Y: 7},  // east of maxx
		{MapName: "test-map", LayerName: "test-layer", Z: 4, X: 15, Y: 7}, // far east
		{MapName: "test-map", LayerName: "test-layer", Z: 4, X: 15, Y: 0}, // far east + north
	} {
		if got := cacher.keyCount(key); got != 0 {
			t.Errorf("out-of-bounds tile %d/%d/%d was cached %d times, want 0", key.Z, key.X, key.Y, got)
		}
	}

	if got := cacher.setCount(); got != 1 {
		t.Errorf("cache writes = %d, want 1 (only the in-bounds tile)", got)
	}
}
