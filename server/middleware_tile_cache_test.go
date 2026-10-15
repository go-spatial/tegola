package server_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/cache/memory"
	"github.com/alexeydott/tegola/server"
)

func TestMiddlewareTileCacheHandler(t *testing.T) {
	type tcase struct {
		uri       string
		uriPrefix string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			var err error

			if tc.uriPrefix != "" {
				server.URIPrefix = tc.uriPrefix
			} else {
				server.URIPrefix = "/"
			}

			a := newTestMapWithLayers(testLayer1, testLayer2, testLayer3)
			cacher, _ := memory.New(nil)
			a.SetCache(cacher)

			w, router, err := doRequest(t, a, http.MethodGet, tc.uri, nil)
			if err != nil {
				t.Errorf("error making request, expected nil got %v", err)
				return
			}

			// first response we expect the cache to MISS
			if w.Header().Get("Tegola-Cache") != "MISS" {
				t.Errorf("header Tegola-Cache, expected MISS got %v", w.Header().Get("Tegola-Cache"))
				return
			}

			// play the request again to get a HIT
			r, err := http.NewRequest("GET", tc.uri, nil)
			if err != nil {
				t.Errorf("error making request, expected nil got %v", err)
				return
			}

			w = httptest.NewRecorder()
			router.ServeHTTP(w, r)

			if w.Header().Get("Tegola-Cache") != "HIT" {
				t.Errorf("Tegoal-Cache, expected HIT got %v", w.Header().Get("Tegola-Cache"))
				return
			}
		}
	}

	tests := map[string]tcase{
		"map": {
			uri: "/maps/test-map/10/2/3.pbf",
		},
		"map layer": {
			uri: "/maps/test-map/test-layer/4/2/3.pbf",
		},
		"map and uri prefix": {
			uri:       "/tegola/maps/test-map/10/2/3.pbf",
			uriPrefix: "/tegola",
		},
		"map layer and uri prefix": {
			uri:       "/tegola/maps/test-map/test-layer/4/2/3.pbf",
			uriPrefix: "/tegola",
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestMiddlewareTileCacheHandlerIgnoreParams(t *testing.T) {
	type tcase struct {
		uri       string
		uriPrefix string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			var err error

			if tc.uriPrefix != "" {
				server.URIPrefix = tc.uriPrefix
			} else {
				server.URIPrefix = "/"
			}

			a := newTestMapWithLayers(testLayer1, testLayer2, testLayer3)
			cacher, _ := memory.New(nil)
			a.SetCache(cacher)

			w, router, err := doRequest(t, a, http.MethodGet, tc.uri, nil)
			if err != nil {
				t.Errorf("error making request, expected nil got %v", err)
				return
			}

			// we expect the cache to not being used
			if w.Header().Get("Tegola-Cache") != "" {
				t.Errorf("no header Tegola-Cache is expected, got %v", w.Header().Get("Tegola-Cache"))
				return
			}

			// play the request again
			r, err := http.NewRequest("GET", tc.uri, nil)
			if err != nil {
				t.Errorf("error making request, expected nil got %v", err)
				return
			}

			w = httptest.NewRecorder()
			router.ServeHTTP(w, r)

			if w.Header().Get("Tegola-Cache") != "" {
				t.Errorf("no header Tegola-Cache is expected, got %v", w.Header().Get("Tegola-Cache"))
				return
			}
		}
	}

	tests := map[string]tcase{
		"map params": {
			uri: "/maps/test-map/10/2/3.pbf?param=value",
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestMiddlewareTileCacheHandlerDirtyRegenerates(t *testing.T) {
	server.URIPrefix = "/"
	enableTileOperations(t, serverTileOpsTestConfig())
	a := newTestMapWithLayers(testLayer1)
	cacher, _ := memory.New(nil)
	a.SetCache(cacher)
	router := server.NewRouter(a)
	uri := "/maps/test-map/4/2/3.pbf"

	request := func(uri string) *httptest.ResponseRecorder {
		r, err := http.NewRequest(http.MethodGet, uri, nil)
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set(server.TileOperationsTokenHeader, testTileOpsToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}

	if w := request(uri); w.Header().Get("Tegola-Cache") != "MISS" {
		t.Fatalf("initial request should miss cache, got %q", w.Header().Get("Tegola-Cache"))
	}
	if w := request(uri); w.Header().Get("Tegola-Cache") != "HIT" {
		t.Fatalf("second request should hit cache, got %q", w.Header().Get("Tegola-Cache"))
	}
	if w := request(uri + "?" + server.QueryKeyDirty + "=true"); w.Header().Get("Tegola-Cache") != "MISS" {
		t.Fatalf("dirty request should regenerate and miss cache, got %q", w.Header().Get("Tegola-Cache"))
	}
	if w := request(uri); w.Header().Get("Tegola-Cache") != "HIT" {
		t.Fatalf("request after dirty regeneration should hit cache, got %q", w.Header().Get("Tegola-Cache"))
	}
}

func TestTileOperationsRegenerateMetatile(t *testing.T) {
	server.URIPrefix = "/"
	enableTileOperations(t, serverTileOpsTestConfig())
	a := newTestMapWithLayers(testLayer1)
	cacher, _ := memory.New(nil)
	a.SetCache(cacher)
	router := server.NewRouter(a)

	request := func(uri string) *httptest.ResponseRecorder {
		t.Helper()
		r, err := http.NewRequest(http.MethodGet, uri, nil)
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set(server.TileOperationsTokenHeader, testTileOpsToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}

	const uri = "/maps/test-map/test-layer/4/2/3.pbf"

	status := request(uri + "?tile=status")
	if status.Code != http.StatusOK {
		t.Fatalf("status operation returned %d: %s", status.Code, status.Body.String())
	}
	var before struct {
		Cached bool `json:"cached"`
	}
	if err := json.Unmarshal(status.Body.Bytes(), &before); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if before.Cached {
		t.Fatal("tile should not be cached before metatile update")
	}

	compressedStatusRequest, err := http.NewRequest(http.MethodGet, uri+"?tile=status", nil)
	if err != nil {
		t.Fatal(err)
	}
	compressedStatusRequest.Header.Set("Accept-Encoding", "gzip")
	compressedStatusRequest.Header.Set(server.TileOperationsTokenHeader, testTileOpsToken)
	compressedStatusRecorder := httptest.NewRecorder()
	router.ServeHTTP(compressedStatusRecorder, compressedStatusRequest)
	if compressedStatusRecorder.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("gzip status response should advertise gzip encoding, got %q", compressedStatusRecorder.Header().Get("Content-Encoding"))
	}
	gzipReader, err := gzip.NewReader(bytes.NewReader(compressedStatusRecorder.Body.Bytes()))
	if err != nil {
		t.Fatalf("open compressed status response: %v", err)
	}
	if _, err := io.Copy(io.Discard, gzipReader); err != nil {
		t.Fatalf("read compressed status response: %v", err)
	}
	if err := gzipReader.Close(); err != nil {
		t.Fatalf("close compressed status response: %v", err)
	}

	update := request(uri + "?tile=update")
	if update.Code != http.StatusAccepted {
		t.Fatalf("update operation returned %d: %s", update.Code, update.Body.String())
	}
	if update.Body.Len() != 0 {
		t.Fatalf("update operation returned tile data: %d bytes", update.Body.Len())
	}
	if update.Header().Get("Content-Encoding") != "" {
		t.Fatalf("update operation must not advertise content encoding: %q", update.Header().Get("Content-Encoding"))
	}

	// the metatile regeneration runs in the background after the 202
	server.AwaitMetatileRegeneration()

	for y := uint(0); y < 8; y++ {
		for x := uint(0); x < 8; x++ {
			key := &cache.Key{MapName: "test-map", LayerName: "test-layer", Z: 4, X: x, Y: y}
			if _, hit, err := cacher.Get(context.Background(), key); err != nil || !hit {
				t.Fatalf("metatile tile %v/%v not cached: hit=%v err=%v", x, y, hit, err)
			}
		}
	}

	status = request(uri + "?tile=status")
	if status.Code != http.StatusOK {
		t.Fatalf("status operation after update returned %d", status.Code)
	}
	if err := json.Unmarshal(status.Body.Bytes(), &before); err != nil {
		t.Fatalf("decode status response after update: %v", err)
	}
	if !before.Cached {
		t.Fatal("tile should be cached after metatile update")
	}

	updated := request(uri + "?tile=getupdated")
	if updated.Code != http.StatusOK {
		t.Fatalf("getupdated operation returned %d: %s", updated.Code, updated.Body.String())
	}
	if updated.Body.Len() == 0 {
		t.Fatal("getupdated operation returned an empty tile")
	}

	// getupdated schedules a metatile regeneration in the background as well
	server.AwaitMetatileRegeneration()

	normal := request(uri)
	if normal.Code != http.StatusOK || normal.Header().Get("Tegola-Cache") != "HIT" {
		t.Fatalf("ordinary request after getupdated should be a cache HIT, got code=%d cache=%q", normal.Code, normal.Header().Get("Tegola-Cache"))
	}

	invalid := request(uri + "?tile=status&param=value")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("tile operation with an extra parameter should be rejected, got %d", invalid.Code)
	}
}
