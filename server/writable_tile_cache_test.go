package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	vectorTile "github.com/alexeydott/geom/encoding/mvt/vector_tile"
	"github.com/alexeydott/geom/slippy"
	"google.golang.org/protobuf/proto"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexeydott/geom/encoding/mvt"
	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/cache/memory"
)

func TestWritableCacheGenerationInflight(t *testing.T) {
	backend, _ := memory.New(nil)
	a := &atlas.Atlas{}
	a.SetCache(backend)
	state := newWritableTileCacheState()
	started, release := make(chan struct{}), make(chan struct{})
	var renders atomic.Int32
	h := state.Wrap(TileCacheHandler(a, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := renders.Add(1)
		if n == 1 {
			close(started)
			<-release
		}
		w.Header().Set("Content-Type", mvt.MimeType)
		if n == 1 {
			if _, err := w.Write([]byte("old")); err != nil {
				t.Errorf("write old tile: %v", err)
			}
		} else {
			if _, err := w.Write([]byte("new")); err != nil {
				t.Errorf("write new tile: %v", err)
			}
		}
	})))
	request := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/maps/generation/2/1/1.pbf", nil))
		return w
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- request() }()
	<-started
	state.Invalidate()
	newer := make(chan *httptest.ResponseRecorder, 1)
	go func() { newer <- request() }()
	select {
	case w := <-newer:
		if w.Body.String() != "new" {
			t.Fatalf("joined old render: %s", w.Body.String())
		}
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("new generation joined old render")
	}
	close(release)
	<-done
	w := request()
	if w.Body.String() != "new" || w.Header().Get("Tegola-Cache") != "HIT" {
		t.Fatalf("old render poisoned current cache: %s %s", w.Body.String(), w.Header().Get("Tegola-Cache"))
	}
}
func TestWritableCacheEditorBypass(t *testing.T) {
	backend, _ := memory.New(nil)
	a := &atlas.Atlas{}
	a.SetCache(backend)
	state := newWritableTileCacheState()
	var renders int
	h := state.Wrap(TileCacheHandler(a, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		renders++
		w.Header().Set("Content-Type", mvt.MimeType)
		if _, err := w.Write([]byte("tile")); err != nil {
			t.Errorf("write tile: %v", err)
		}
	})))
	request := func(editor string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/maps/editor/2/1/1.pbf", nil)
		r.Header.Set(EditorActiveHeader, editor)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if request("").Header().Get("Tegola-Cache") != "MISS" {
		t.Fatal("expected initial MISS")
	}
	if request("").Header().Get("Tegola-Cache") != "HIT" {
		t.Fatal("expected ordinary HIT")
	}
	w := request("true")
	if w.Header().Get("Tegola-Cache") != "" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("editor used cache")
	}
	if request("").Header().Get("Tegola-Cache") != "HIT" || renders != 2 {
		t.Fatal("editor damaged browsing cache")
	}
}
func TestWritableCacheSnapshotKeys(t *testing.T) {
	backend, _ := memory.New(nil)
	state := newWritableTileCacheState()
	snapshot := func() cache.Interface {
		var result cache.Interface
		state.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { result = requestTileCache(r.Context(), backend) })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		return result
	}
	old := snapshot()
	key := &cache.Key{MapName: "moek", Z: 2, X: 1, Y: 1}
	if err := old.Set(context.Background(), key, []byte("old")); err != nil {
		t.Fatalf("cache old tile: %v", err)
	}
	state.Invalidate()
	current := snapshot()
	if _, hit, _ := current.Get(context.Background(), key); hit {
		t.Fatal("new generation reads old whole-map key")
	}
	if cacheCoordinationKey(old, key) == cacheCoordinationKey(current, key) || cacheMetatileLockKey(old, key) == cacheMetatileLockKey(current, key) {
		t.Fatal("coordination keys overlap generations")
	}
	if err := old.Set(context.Background(), key, []byte("late")); err != nil {
		t.Fatalf("cache late tile: %v", err)
	}
	if _, hit, _ := current.Get(context.Background(), key); hit {
		t.Fatal("late old generation write visible")
	}
	if _, hit, _ := old.Get(context.Background(), key); !hit {
		t.Fatal("snapshot changed after invalidation")
	}
	restart := newWritableTileCacheState()
	if state.startup == restart.startup {
		t.Fatal("startup namespace reused")
	}
}

func TestWritableCacheWholeMapAliasStatus(t *testing.T) {
	backend, _ := memory.New(nil)
	a := &atlas.Atlas{}
	a.SetCache(backend)
	m := atlas.Map{Name: "moek", Layers: []atlas.Layer{{Name: "points", ProviderLayerName: "provider-points", MaxZoom: 22}}}
	a.AddMap(m)
	state := newWritableTileCacheState()
	var ctx context.Context
	state.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ctx = r.Context() })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	c := requestTileCache(ctx, backend)
	name := "points"
	version := uint32(2)
	extent := uint32(4096)
	raw, err := proto.Marshal(&vectorTile.Tile{Layers: []*vectorTile.Tile_Layer{{Name: &name, Version: &version, Extent: &extent}}})
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	gz := gzip.NewWriter(&body)
	if _, err := gz.Write(raw); err != nil {
		t.Fatalf("compress tile: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("finish compressed tile: %v", err)
	}
	whole := cache.Key{MapName: "moek", Z: 2, X: 1, Y: 1}
	if err = c.Set(ctx, &whole, body.Bytes()); err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{"points", "provider-points"} {
		key := whole
		key.LayerName = alias
		if _, hit, err := cachedMapLayer(ctx, a, c, &key); err != nil || !hit {
			t.Fatalf("alias %s: hit=%v err=%v", alias, hit, err)
		}
		req := HandleMapLayerZXY{Atlas: a, mapName: "moek", layerName: alias}
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		if err := req.serveTileOperation(w, r, m, slippy.Tile{Z: 2, X: 1, Y: 1}, tileOperationStatus); err != nil {
			t.Fatal(err)
		}
		reader, err := gzip.NewReader(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		payload, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			t.Fatalf("read compressed status: %v", readErr)
		}
		if closeErr != nil {
			t.Fatalf("close compressed status: %v", closeErr)
		}
		var status tileStatusResponse
		if err = json.Unmarshal(payload, &status); err != nil {
			t.Fatal(err)
		}
		if !status.Cached {
			t.Fatalf("status ignores whole-map alias %s", alias)
		}
	}
}
