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

// TestTileCacheCancelledLeadersNeverServeEmptyToLiveWaiter is the audit's
// acceptance test for P5-1: a leader canceled twice in a row while a live
// waiter waits must never hand the waiter an empty 200. The waiter either
// receives the complete tile or a proper 5xx error, and the shared render
// survives the leaders' disconnects.
func TestTileCacheCancelledLeadersNeverServeEmptyToLiveWaiter(t *testing.T) {
	prevPrefix := URIPrefix
	URIPrefix = "/"
	defer func() { URIPrefix = prevPrefix }()

	tileBody := []byte{0x1a, 0x02, 0x08, 0x01} // arbitrary MVT-ish payload
	key, err := cache.ParseKey("/m/l/1/0/0")
	if err != nil {
		t.Fatal(err)
	}

	for iter := 0; iter < 8; iter++ {
		var (
			mu      sync.Mutex
			renders int
		)
		entered := make(chan struct{}, 16)
		release := make(chan struct{})

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			renders++
			mu.Unlock()
			entered <- struct{}{}
			select {
			case <-release:
			case <-r.Context().Done():
				// an aborted render writes nothing, like a provider that
				// honors context cancellation
				return
			}
			w.Header().Set("Content-Type", mvt.MimeType)
			_, _ = w.Write(tileBody)
		})

		cacher := newFakeTileCache()
		a := &atlas.Atlas{}
		a.SetCache(cacher)
		handler := TileCacheHandler(a, next)

		reqURL := "/maps/m/l/1/0/0"

		// leader L1 starts the render and is canceled while it is in flight
		ctx1, cancel1 := context.WithCancel(context.Background())
		go handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, reqURL, nil).WithContext(ctx1))
		<-entered

		// a live waiter and a second leader L2 join the in-flight render
		recW := httptest.NewRecorder()
		wDone := make(chan struct{})
		go func() {
			defer close(wDone)
			handler.ServeHTTP(recW, httptest.NewRequest(http.MethodGet, reqURL, nil))
		}()
		ctx2, cancel2 := context.WithCancel(context.Background())
		go handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, reqURL, nil).WithContext(ctx2))

		// join window; same style as the other tile cache concurrency tests
		time.Sleep(200 * time.Millisecond)

		// leaders canceled twice in a row while the waiter keeps waiting
		cancel1()
		time.Sleep(100 * time.Millisecond)
		cancel2()

		// let any surviving render finish
		close(release)
		select {
		case <-wDone:
		case <-timeAfter(t):
			t.Fatalf("iteration %d: live waiter never completed", iter)
		}
		cancel1()
		cancel2()

		// the audit invariant: the live waiter never receives an empty 200
		switch {
		case recW.Code == http.StatusOK:
			if !bytes.Equal(recW.Body.Bytes(), tileBody) {
				t.Fatalf("iteration %d: waiter got body %q with 200; want complete tile %q (an empty 200 is never correct)",
					iter, recW.Body.Bytes(), tileBody)
			}
		case recW.Code < 500:
			t.Fatalf("iteration %d: waiter status = %d, want complete tile (200) or a 5xx error", iter, recW.Code)
		}

		// the shared render must survive the leaders' disconnects: exactly
		// one render serves all three requests
		mu.Lock()
		got := renders
		mu.Unlock()
		if got != 1 {
			t.Fatalf("iteration %d: renders = %d, want 1 (a leader disconnect must not abort a shared render)", iter, got)
		}

		// the complete tile also lands in the cache for later requests
		if recW.Code == http.StatusOK {
			cached, hit, err := cacher.Get(context.Background(), key)
			if err != nil || !hit || !bytes.Equal(cached, tileBody) {
				t.Fatalf("iteration %d: cached = %v, %v, %v; want complete tile, true, nil", iter, cached, hit, err)
			}
		}
	}
}
