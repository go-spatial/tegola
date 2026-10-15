package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/cache/memory"
	"github.com/alexeydott/tegola/server"
)

const testTileOpsToken = "test-tile-ops-token"

// countingCache counts cache writes in total and per key.
type countingCache struct {
	cache.Interface
	mu    sync.Mutex
	sets  int
	byKey map[string]int
}

func (c *countingCache) Set(ctx context.Context, key *cache.Key, val []byte) error {
	c.mu.Lock()
	if c.byKey == nil {
		c.byKey = make(map[string]int)
	}
	c.sets++
	c.byKey[key.String()]++
	c.mu.Unlock()

	return c.Interface.Set(ctx, key, val)
}

func (c *countingCache) setCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.sets
}

func (c *countingCache) keyCount(key cache.Key) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.byKey[key.String()]
}

// newTileOpsTestServer wires a counting cache into the given atlas and returns
// the cache together with a router serving the atlas.
func newTileOpsTestServer(t *testing.T, a *atlas.Atlas) (*countingCache, http.Handler) {
	t.Helper()

	mem, err := memory.New(nil)
	if err != nil {
		t.Fatalf("memory.New() error = %v", err)
	}
	cacher := &countingCache{Interface: mem}
	a.SetCache(cacher)

	return cacher, server.NewRouter(a)
}

// enableTileOperations overrides the package level tile operations config for
// one test and restores the previous value afterwards.
func enableTileOperations(t *testing.T, cfg server.TileOperationsConfig) {
	t.Helper()

	old := server.TileOperations
	t.Cleanup(func() {
		server.TileOperations = old
	})
	server.TileOperations = cfg
}

// serverTileOpsTestConfig enables tile operations with a fixed token and very
// generous limits, so tests are not affected by the shared rate/concurrency
// window state.
func serverTileOpsTestConfig() server.TileOperationsConfig {
	return server.TileOperationsConfig{
		Enabled:       true,
		Token:         testTileOpsToken,
		RatePerMinute: 1000,
		MaxConcurrent: 100,
	}
}

// tileOpsRequest runs a GET request through the router; a non-empty token is
// sent in the tile operations token header.
func tileOpsRequest(router http.Handler, uri, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, uri, nil)
	if token != "" {
		r.Header.Set(server.TileOperationsTokenHeader, token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	return w
}

func TestTileOperationsDisabledRefusesOperations(t *testing.T) {
	a := newTestMapWithLayers(testLayer1)
	cacher, router := newTileOpsTestServer(t, a)
	enableTileOperations(t, server.TileOperationsConfig{})

	base := "/maps/test-map/test-layer/4/2/3.pbf"
	for _, op := range []string{
		"?tile=update",
		"?tile=getupdated",
		"?tile=status",
		"?dirty=true",
		"?dirty=1",
		"?dirty",
	} {
		// even a well formed token must not enable operations while the
		// feature flag is off
		w := tileOpsRequest(router, base+op, testTileOpsToken)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403 (%s)", op, w.Code, w.Body.String())
		}
	}

	if got := cacher.setCount(); got != 0 {
		t.Errorf("tile operations wrote to the cache %d times while disabled, want 0", got)
	}

	// ordinary tile serving keeps working while operations are disabled
	w := tileOpsRequest(router, base, "")
	if w.Code != http.StatusOK {
		t.Fatalf("ordinary tile status = %d, want 200 (%s)", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Tegola-Cache"); got != "MISS" {
		t.Errorf("Tegola-Cache = %q, want MISS", got)
	}
	if got := cacher.setCount(); got != 1 {
		t.Errorf("ordinary tile cache writes = %d, want 1", got)
	}

	w = tileOpsRequest(router, base, "")
	if got := w.Header().Get("Tegola-Cache"); got != "HIT" {
		t.Errorf("Tegola-Cache = %q, want HIT", got)
	}
}

func TestTileOperationsRequireToken(t *testing.T) {
	a := newTestMapWithLayers(testLayer1)
	cacher, router := newTileOpsTestServer(t, a)
	enableTileOperations(t, server.TileOperationsConfig{
		Enabled:       true,
		Token:         testTileOpsToken,
		RatePerMinute: 1000,
		MaxConcurrent: 100,
	})

	base := "/maps/test-map/test-layer/4/2/3.pbf"

	// missing and wrong tokens are refused
	w := tileOpsRequest(router, base+"?tile=update", "")
	if w.Code != http.StatusForbidden {
		t.Errorf("missing token: status = %d, want 403", w.Code)
	}
	w = tileOpsRequest(router, base+"?tile=update", "wrong-token")
	if w.Code != http.StatusForbidden {
		t.Errorf("wrong token: status = %d, want 403", w.Code)
	}
	w = tileOpsRequest(router, base+"?dirty=true", "")
	if w.Code != http.StatusForbidden {
		t.Errorf("dirty without token: status = %d, want 403", w.Code)
	}

	if got := cacher.setCount(); got != 0 {
		t.Fatalf("refused operations wrote to the cache %d times, want 0", got)
	}

	// a valid token authorizes the operation
	w = tileOpsRequest(router, base+"?tile=update", testTileOpsToken)
	if w.Code != http.StatusAccepted {
		t.Fatalf("authorized update: status = %d, want 202 (%s)", w.Code, w.Body.String())
	}
	server.AwaitMetatileRegeneration()
	// z4 tile 2/3 lives in metatile x[0..7] y[0..7]; the map has no bounds
	// configured so all 64 tiles must be cached
	if got := cacher.setCount(); got != 64 {
		t.Errorf("update cache writes = %d, want 64", got)
	}

	// status works with a token
	w = tileOpsRequest(router, base+"?tile=status", testTileOpsToken)
	if w.Code != http.StatusOK {
		t.Fatalf("status: status = %d, want 200 (%s)", w.Code, w.Body.String())
	}

	// dirty regenerates a single tile with a valid token
	before := cacher.keyCount(cache.Key{MapName: "test-map", LayerName: "test-layer", Z: 4, X: 2, Y: 3})
	w = tileOpsRequest(router, base+"?dirty=true", testTileOpsToken)
	if w.Code != http.StatusOK {
		t.Fatalf("dirty: status = %d, want 200 (%s)", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Tegola-Cache"); got != "MISS" {
		t.Errorf("dirty Tegola-Cache = %q, want MISS", got)
	}
	if got, want := cacher.keyCount(cache.Key{MapName: "test-map", LayerName: "test-layer", Z: 4, X: 2, Y: 3}), before+1; got != want {
		t.Errorf("dirty tile cache writes = %d, want %d", got, want)
	}
}

// TestConcurrentTileUpdatesShareOneRegeneration guards the no-stampede
// property: concurrent ?tile=update requests for the same metatile must share
// a single regeneration, so the metatile is rendered and cached exactly once.
func TestConcurrentTileUpdatesShareOneRegeneration(t *testing.T) {
	tiler := newBlockingTiler(1)
	layer := testLayer1
	layer.Provider = tiler

	a := newTestMapWithLayers(layer)
	cacher, router := newTileOpsTestServer(t, a)
	enableTileOperations(t, serverTileOpsTestConfig())

	uri := "/maps/test-map/test-layer/4/2/3.pbf?tile=update"

	const requestCount = 5
	codes := make(chan int, requestCount)
	for i := 0; i < requestCount; i++ {
		go func() {
			w := tileOpsRequest(router, uri, testTileOpsToken)
			codes <- w.Code
		}()
	}

	// every request is answered while the shared regeneration is held inside
	// the provider: the request path must not wait for the metatile render
	for i := 0; i < requestCount; i++ {
		if code := <-codes; code != http.StatusAccepted {
			t.Fatalf("concurrent update: status = %d, want %d", code, http.StatusAccepted)
		}
	}

	// all requests joined the one regeneration held in the provider; release
	// it and let the shared regeneration finish
	close(tiler.release)
	server.AwaitMetatileRegeneration()

	// z4 tile 2/3 lives in metatile x[0..7] y[0..7] (64 tiles): one shared
	// regeneration must write exactly 64 tiles, not 5x64
	if got := cacher.setCount(); got != 64 {
		t.Errorf("concurrent updates wrote %d cache entries, want 64 (one regeneration)", got)
	}
}

func TestTileOperationsEnabledWithoutTokenFailsClosed(t *testing.T) {
	a := newTestMapWithLayers(testLayer1)
	cacher, router := newTileOpsTestServer(t, a)
	enableTileOperations(t, server.TileOperationsConfig{
		Enabled:       true,
		Token:         "",
		RatePerMinute: 1000,
		MaxConcurrent: 100,
	})

	base := "/maps/test-map/test-layer/4/2/3.pbf?tile=update"
	for _, token := range []string{"", "anything"} {
		w := tileOpsRequest(router, base, token)
		if w.Code != http.StatusForbidden {
			t.Errorf("token %q: status = %d, want 403", token, w.Code)
		}
	}

	if got := cacher.setCount(); got != 0 {
		t.Errorf("fail closed operations wrote to the cache %d times, want 0", got)
	}
}

func TestTileOperationsRateLimitEnforced(t *testing.T) {
	a := newTestMapWithLayers(testLayer1)
	_, router := newTileOpsTestServer(t, a)
	enableTileOperations(t, server.TileOperationsConfig{
		Enabled:       true,
		Token:         testTileOpsToken,
		RatePerMinute: 1,
		MaxConcurrent: 100,
	})

	// the rate limiter window is shared, so depending on operations issued by
	// other tests the first request may already be refused; what must hold is
	// that a single operation per minute refuses at least one of the two.
	base := "/maps/test-map/test-layer/4/2/3.pbf?tile=status"
	var codes []int
	for i := 0; i < 2; i++ {
		w := tileOpsRequest(router, base, testTileOpsToken)
		codes = append(codes, w.Code)
		if w.Code != http.StatusTooManyRequests && w.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200 or 429", i, w.Code)
		}
	}

	if codes[0] != http.StatusTooManyRequests && codes[1] != http.StatusTooManyRequests {
		t.Errorf("rate limit of 1 per minute was not enforced: statuses %v", codes)
	}
}

// TestFalsyDirtyValuesBypassDisabledOperationsGate verifies P5-6: a ?dirty
// query that does not actually request regeneration (?dirty=0, ?dirty=false,
// ...) behaves as an ordinary cache query. It must not trip the tile
// operations gate, even when tile operations are disabled.
func TestFalsyDirtyValuesBypassDisabledOperationsGate(t *testing.T) {
	a := newTestMapWithLayers(testLayer1)
	_, router := newTileOpsTestServer(t, a)
	enableTileOperations(t, server.TileOperationsConfig{})

	// one tile per case so every first request is a cache miss
	for i, op := range []string{"?dirty=0", "?dirty=false", "?dirty=FALSE", "?dirty=no"} {
		base := fmt.Sprintf("/maps/test-map/test-layer/4/2/%d.pbf", i)
		w := tileOpsRequest(router, base+op, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200 (%s)", op, w.Code, w.Body.String())
		}
		if got := w.Header().Get("Tegola-Cache"); got != "MISS" {
			t.Errorf("%s: Tegola-Cache = %q, want MISS", op, got)
		}

		// a falsy ?dirty is inert: the repeat request is an ordinary cache hit
		w = tileOpsRequest(router, base+op, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s repeat: status = %d, want 200 (%s)", op, w.Code, w.Body.String())
		}
		if got := w.Header().Get("Tegola-Cache"); got != "HIT" {
			t.Errorf("%s repeat: Tegola-Cache = %q, want HIT", op, got)
		}
	}

	// absent ?dirty keeps behaving as an ordinary cache query
	base := "/maps/test-map/test-layer/4/2/8.pbf"
	w := tileOpsRequest(router, base, "")
	if w.Code != http.StatusOK || w.Header().Get("Tegola-Cache") != "MISS" {
		t.Errorf("absent dirty: status/cache = %d/%q, want 200/MISS", w.Code, w.Header().Get("Tegola-Cache"))
	}

	// the regenerating variants are still refused while disabled
	for _, op := range []string{"?dirty=1", "?dirty=true", "?dirty", "?dirty=TRUE"} {
		w := tileOpsRequest(router, "/maps/test-map/test-layer/4/2/9.pbf"+op, testTileOpsToken)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403 (%s)", op, w.Code, w.Body.String())
		}
	}
}

// TestFalsyDirtyValuesNotGatedWhenOperationsEnabled verifies P5-6 from the
// other side: even with tile operations enabled, only the regenerating ?dirty
// variant requires the token. A falsy value is served to anyone as an
// ordinary cache query.
func TestFalsyDirtyValuesNotGatedWhenOperationsEnabled(t *testing.T) {
	a := newTestMapWithLayers(testLayer1)
	_, router := newTileOpsTestServer(t, a)
	enableTileOperations(t, serverTileOpsTestConfig())

	// no token: falsy ?dirty is not an operation
	w := tileOpsRequest(router, "/maps/test-map/test-layer/4/2/0.pbf?dirty=0", "")
	if w.Code != http.StatusOK {
		t.Fatalf("?dirty=0 without token: status = %d, want 200 (%s)", w.Code, w.Body.String())
	}

	// no token: regenerating ?dirty still requires authentication
	w = tileOpsRequest(router, "/maps/test-map/test-layer/4/2/0.pbf?dirty=true", "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("?dirty=true without token: status = %d, want 403 (%s)", w.Code, w.Body.String())
	}
}
