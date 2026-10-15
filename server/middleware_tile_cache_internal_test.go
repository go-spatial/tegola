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
	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/cache"
)

// fakeTileCache is an in-memory cache.Interface that records Set calls so
// tests can assert what was written to the cache.
type fakeTileCache struct {
	mu       sync.Mutex
	items    map[string][]byte
	setCalls map[string]int
}

func newFakeTileCache() *fakeTileCache {
	return &fakeTileCache{
		items:    make(map[string][]byte),
		setCalls: make(map[string]int),
	}
}

func (f *fakeTileCache) Get(_ context.Context, key *cache.Key) ([]byte, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.items[key.String()]
	return v, ok, nil
}

func (f *fakeTileCache) Set(_ context.Context, key *cache.Key, value []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items[key.String()] = value
	f.setCalls[key.String()]++
	return nil
}

func (f *fakeTileCache) Purge(_ context.Context, key *cache.Key) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.items, key.String())
	return nil
}

func (f *fakeTileCache) setCount(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.setCalls[key]
}

// timeAfter returns a channel that fires after a test-friendly timeout.
func timeAfter(t *testing.T) <-chan time.Time {
	t.Helper()
	return time.After(2 * time.Second)
}

func TestTileRenderCaptureCapturesResponse(t *testing.T) {
	c := newTileRenderCapture()

	// the capture is stamped as a cache miss like the old response writer
	if got := c.Header().Get("Tegola-Cache"); got != "MISS" {
		t.Fatalf("Tegola-Cache = %q, want MISS", got)
	}

	c.Header().Set("Content-Type", mvt.MimeType)
	if n, err := c.Write([]byte{0x1, 0x2}); err != nil || n != 2 {
		t.Fatalf("Write = %d, %v; want 2, nil", n, err)
	}
	if n, err := c.Write([]byte{0x3}); err != nil || n != 1 {
		t.Fatalf("Write = %d, %v; want 1, nil", n, err)
	}

	res := c.result(false)
	if res.status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (implicit)", res.status)
	}
	if !bytes.Equal(res.body, []byte{0x1, 0x2, 0x3}) {
		t.Fatalf("body = %v, want [1 2 3]", res.body)
	}
	if res.canceled {
		t.Fatal("result unexpectedly canceled")
	}
	if got := res.header.Get("Content-Type"); got != mvt.MimeType {
		t.Fatalf("result Content-Type = %q, want %q", got, mvt.MimeType)
	}

	// headers are frozen at commit: later mutations must not leak into the
	// captured result
	c.Header().Set("Content-Type", "text/plain")
	if got := res.header.Get("Content-Type"); got != mvt.MimeType {
		t.Fatalf("result Content-Type after mutation = %q, want %q", got, mvt.MimeType)
	}
}

func TestTileRenderCaptureWriteHeaderFirstWins(t *testing.T) {
	c := newTileRenderCapture()
	c.WriteHeader(http.StatusOK)
	c.WriteHeader(http.StatusInternalServerError)

	res := c.result(false)
	if res.status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (first WriteHeader wins)", res.status)
	}

	// a capture with no writes at all resolves to an empty 200
	empty := newTileRenderCapture().result(false)
	if empty.status != http.StatusOK || len(empty.body) != 0 {
		t.Fatalf("empty capture = %d, %q; want 200, empty", empty.status, empty.body)
	}
}

func TestRenderTileForCacheCachesOnlySuccessfulMVT(t *testing.T) {
	type tcase struct {
		status      int
		contentType string
		wantCached  bool
	}

	tests := map[string]tcase{
		"ok mvt":            {status: http.StatusOK, contentType: mvt.MimeType, wantCached: true},
		"ok mvt with codec": {status: http.StatusOK, contentType: mvt.MimeType + "; charset=utf-8", wantCached: true},
		"error status":      {status: http.StatusInternalServerError, contentType: mvt.MimeType, wantCached: false},
		"not mvt":           {status: http.StatusOK, contentType: "application/json", wantCached: false},
		"no content type":   {status: http.StatusOK, contentType: "", wantCached: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cacher := newFakeTileCache()
			key := &cache.Key{MapName: "m", LayerName: "l", Z: 1, X: 0, Y: 0}

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.contentType != "" {
					w.Header().Set("Content-Type", tc.contentType)
				}
				w.WriteHeader(tc.status)
				// write the body in parts to prove multi-write bodies are
				// captured whole
				_, _ = w.Write([]byte{0x1, 0x2})
				_, _ = w.Write([]byte{0x3})
			})

			req := httptest.NewRequest(http.MethodGet, "/maps/m/l/1/0/0", nil)
			res := renderTileForCache(req.Context(), req, handler, cacher, key, true)

			if res.status != tc.status {
				t.Fatalf("rendered status = %d, want %d", res.status, tc.status)
			}
			if tc.status == http.StatusOK && !bytes.Equal(res.body, []byte{0x1, 0x2, 0x3}) {
				t.Fatalf("rendered body = %v, want [1 2 3]", res.body)
			}

			want := 0
			if tc.wantCached {
				want = 1
			}
			if got := cacher.setCount(key.String()); got != want {
				t.Fatalf("cacher.Set calls = %d, want %d", got, want)
			}
			if tc.wantCached {
				cached, hit, err := cacher.Get(context.Background(), key)
				if err != nil || !hit || !bytes.Equal(cached, []byte{0x1, 0x2, 0x3}) {
					t.Fatalf("cached = %v, %v, %v; want [1 2 3], true, nil", cached, hit, err)
				}
			}
		})
	}
}

func TestRenderTileForCacheSkipsStaleWrites(t *testing.T) {
	cacher := newFakeTileCache()
	key := &cache.Key{MapName: "m", LayerName: "l", Z: 1, X: 0, Y: 0}
	lockKey := metatileLockKeyForCacheKey(key)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// a ?tile=update regenerates this metatile while the render is in
		// flight: its generation advances and the render result goes stale
		state := tileUpdateLocks.retain(lockKey)
		defer tileUpdateLocks.release(lockKey, state)
		tileUpdateLocks.beginUpdate(state)
		tileUpdateLocks.endUpdate(state)

		w.Header().Set("Content-Type", mvt.MimeType)
		_, _ = w.Write([]byte("stale"))
	})

	req := httptest.NewRequest(http.MethodGet, "/maps/m/l/1/0/0", nil)
	renderTileForCache(req.Context(), req, handler, cacher, key, true)

	if got := cacher.setCount(key.String()); got != 0 {
		t.Fatalf("cacher.Set calls = %d, want 0 (stale render must not write)", got)
	}
}

func TestTileRenderGroupSharesResults(t *testing.T) {
	var group tileRenderGroup
	started := make(chan struct{})
	var mu sync.Mutex
	var calls int
	var results []*tileRenderResult

	// a single render result is shared by all concurrent callers
	fn := func(context.Context) *tileRenderResult {
		mu.Lock()
		calls++
		mu.Unlock()
		close(started)
		// give the concurrent callers time to join this call
		time.Sleep(50 * time.Millisecond)
		return &tileRenderResult{status: http.StatusOK, body: []byte("tile")}
	}

	var ready sync.WaitGroup
	var wg sync.WaitGroup
	wg.Add(4)
	ready.Add(4)
	for i := 0; i < 4; i++ {
		go func() {
			defer wg.Done()
			ready.Done()
			res, _ := group.do(context.Background(), "k", fn)
			mu.Lock()
			results = append(results, res)
			mu.Unlock()
		}()
	}

	ready.Wait()
	<-started
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Fatalf("render calls = %d, want 1", calls)
	}
	for i, res := range results {
		if res == nil || !bytes.Equal(res.body, []byte("tile")) {
			t.Fatalf("result %d = %+v, want shared 200 body", i, res)
		}
	}
}

func TestTileRenderGroupWaiterUnblocksOnContextCancel(t *testing.T) {
	var group tileRenderGroup
	started := make(chan struct{})
	release := make(chan struct{})

	leaderDone := make(chan struct{})
	go func() {
		defer close(leaderDone)
		_, _ = group.do(context.Background(), "k", func(context.Context) *tileRenderResult {
			close(started)
			<-release
			return &tileRenderResult{status: http.StatusOK, body: []byte("tile")}
		})
	}()
	<-started

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	waiterDone := make(chan *tileRenderResult, 1)
	go func() {
		res, _ := group.do(ctx, "k", func(context.Context) *tileRenderResult {
			t.Error("waiter must not run its own render")
			return nil
		})
		waiterDone <- res
	}()

	// cancel the waiter while the leader is still rendering
	cancel()
	select {
	case res := <-waiterDone:
		if !res.canceled {
			t.Fatalf("waiter result = %+v, want canceled", res)
		}
	case <-timeAfter(t):
		t.Fatal("waiter stayed queued after its context was canceled")
	}

	// the leader is unaffected and finishes normally
	close(release)
	select {
	case <-leaderDone:
	case <-timeAfter(t):
		t.Fatal("leader did not finish")
	}
}

// waitForRenderRefs waits until the in-flight render call for key has exactly
// want waiting requests attached.
func waitForRenderRefs(t *testing.T, group *tileRenderGroup, key string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		group.mu.Lock()
		refs := -1
		if call, ok := group.calls[key]; ok {
			refs = call.refs
		}
		group.mu.Unlock()
		if refs == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("render refs = %d, want %d", refs, want)
		}
		time.Sleep(time.Millisecond)
	}
}

// TestTileRenderGroupWaiterGetsFullResultAfterLeaderCancel verifies the core
// P5-1 invariant at the group level: the initiator of a shared render
// disconnecting must not abort the render a live waiter depends on. The
// waiter receives the complete tile, never an empty/aborted result.
func TestTileRenderGroupWaiterGetsFullResultAfterLeaderCancel(t *testing.T) {
	var group tileRenderGroup
	started := make(chan struct{})
	release := make(chan struct{})

	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	defer cancelLeader()
	go func() {
		_, _ = group.do(leaderCtx, "k", func(ctx context.Context) *tileRenderResult {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return &tileRenderResult{canceled: true}
			}
			return &tileRenderResult{status: http.StatusOK, body: []byte("tile")}
		})
	}()
	<-started

	waiterDone := make(chan *tileRenderResult, 1)
	go func() {
		res, _ := group.do(context.Background(), "k", func(context.Context) *tileRenderResult {
			t.Error("waiter must not run its own render")
			return nil
		})
		waiterDone <- res
	}()
	waitForRenderRefs(t, &group, "k", 2)

	// the initiator disconnects while the waiter is still live
	cancelLeader()

	// release the render; the live waiter must receive the complete tile
	close(release)
	select {
	case res := <-waiterDone:
		if res == nil || res.canceled || string(res.body) != "tile" {
			t.Fatalf("waiter result = %+v, want complete tile", res)
		}
	case <-timeAfter(t):
		t.Fatal("waiter never received the shared result")
	}
}

// TestTileRenderGroupCancelsRenderWhenAllWaitersLeave verifies the flip side
// of the P5-1 detached-render contract: once every request waiting on a shared
// render has disconnected, the detached render is canceled instead of running
// to completion for nobody.
func TestTileRenderGroupCancelsRenderWhenAllWaitersLeave(t *testing.T) {
	var group tileRenderGroup
	started := make(chan struct{})
	renderEnded := make(chan struct{})

	fn := func(ctx context.Context) *tileRenderResult {
		close(started)
		<-ctx.Done() // the detached render only ends when it is canceled
		close(renderEnded)
		return &tileRenderResult{canceled: true}
	}

	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel1()
	defer cancel2()
	res1 := make(chan *tileRenderResult, 1)
	res2 := make(chan *tileRenderResult, 1)
	go func() {
		res, _ := group.do(ctx1, "k", fn)
		res1 <- res
	}()
	<-started
	go func() {
		res, _ := group.do(ctx2, "k", fn)
		res2 <- res
	}()
	waitForRenderRefs(t, &group, "k", 2)

	// every waiter disconnects
	cancel1()
	cancel2()

	select {
	case <-renderEnded:
	case <-timeAfter(t):
		t.Fatal("detached render was not canceled after all waiters left")
	}
	for _, ch := range []chan *tileRenderResult{res1, res2} {
		select {
		case res := <-ch:
			if res == nil || !res.canceled {
				t.Fatalf("disconnected request result = %+v, want canceled", res)
			}
		case <-timeAfter(t):
			t.Fatal("disconnected request never unblocked")
		}
	}
}

// TestTileCacheAbandonedRenderYieldsErrorNotEmptyOK verifies that when the
// bounded shared-render timeout abandons a render, the live request receives a
// proper 5xx error and never an empty 200 (P5-1).
func TestTileCacheAbandonedRenderYieldsErrorNotEmptyOK(t *testing.T) {
	prevTimeout := tileRenderTimeout
	tileRenderTimeout = 50 * time.Millisecond
	defer func() { tileRenderTimeout = prevTimeout }()

	prevPrefix := URIPrefix
	URIPrefix = "/"
	defer func() { URIPrefix = prevPrefix }()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// a render that never finishes on its own; only the bounded render
		// timeout can end it
		<-r.Context().Done()
	})

	cacher := newFakeTileCache()
	a := &atlas.Atlas{}
	a.SetCache(cacher)
	handler := TileCacheHandler(a, next)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/maps/m/l/1/0/0", nil))

	// the render was abandoned; a live request must get a proper error
	if rec.Code == http.StatusOK {
		t.Fatalf("abandoned render served with 200 and %d bytes; want 5xx error", rec.Body.Len())
	}
	if rec.Code < 500 {
		t.Fatalf("abandoned render status = %d, want 5xx", rec.Code)
	}
}
