package server_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	vectorTile "github.com/alexeydott/geom/encoding/mvt/vector_tile"
	"github.com/alexeydott/geom/slippy"
	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/cache/multilevel"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/server"
	"google.golang.org/protobuf/proto"
)

func TestSeededMapServesLayerFromColdFileCache(t *testing.T) {
	oldPrefix := server.URIPrefix
	server.URIPrefix = "/"
	t.Cleanup(func() { server.URIPrefix = oldPrefix })
	root := t.TempDir()
	cfg := dict.Dict{"memory": dict.Dict{"max_zoom": uint(18)}, "file": dict.Dict{"basepath": root, "max_zoom": uint(22), "ttl": 86400}}
	a := newTestMapWithLayers(testLayer2, testLayer3)
	seed, err := multilevel.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	a.SetCache(seed)
	m, err := a.Map(testMapName)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.SeedMapTile(context.Background(), m.FilterLayersByZoom(slippy.Zoom(10)), 10, 2, 3); err != nil {
		t.Fatal(err)
	}
	mapKey := cache.Key{MapName: testMapName, Z: 10, X: 2, Y: 3}
	full, hit, err := seed.Get(context.Background(), &mapKey)
	if err != nil || !hit {
		t.Fatalf("seed hit=%v err=%v", hit, err)
	}
	for _, name := range []string{testLayer2.Name, testLayer2.ProviderLayerName} {
		t.Run(name, func(t *testing.T) {
			// New instance has an empty L1: only the file written by seed can hit.
			cold, err := multilevel.New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			a.SetCache(cold)
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("seeded layer re-rendered") })
			w := httptest.NewRecorder()
			server.TileCacheHandler(a, next).ServeHTTP(w, httptest.NewRequest("GET", "/maps/"+testMapName+"/"+name+"/10/2/3.pbf", nil))
			if w.Header().Get("Tegola-Cache") != "HIT" {
				t.Fatalf("headers=%v", w.Header())
			}
			zr, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(zr)
			_ = zr.Close()
			if err != nil {
				t.Fatal(err)
			}
			var tile vectorTile.Tile
			if err = proto.Unmarshal(raw, &tile); err != nil {
				t.Fatal(err)
			}
			if len(tile.Layers) != 1 || tile.Layers[0].GetName() != testLayer2.Name {
				t.Fatalf("wrong layers: %v", tile.Layers)
			}
			layerKey := mapKey
			layerKey.LayerName = name
			if _, hit, err = cold.Get(context.Background(), &layerKey); err != nil || hit {
				t.Fatalf("derived tile persisted: hit=%v err=%v", hit, err)
			}
		})
	}
	t.Run("whole map stays byte identical", func(t *testing.T) {
		w := httptest.NewRecorder()
		server.TileCacheHandler(a, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("map rendered") })).ServeHTTP(w, httptest.NewRequest("GET", "/maps/"+testMapName+"/10/2/3.pbf", nil))
		if !bytes.Equal(full, w.Body.Bytes()) {
			t.Fatal("whole map cache changed")
		}
	})
	for _, suffix := range []string{"?debug=true", "?custom=value"} {
		t.Run(suffix, func(t *testing.T) {
			called := false
			server.TileCacheHandler(a, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/maps/"+testMapName+"/"+testLayer2.Name+"/10/2/3.pbf"+suffix, nil))
			if !called {
				t.Fatal("parameterized request incorrectly used default map cache")
			}
		})
	}
	t.Run("separate layer cache has priority", func(t *testing.T) {
		cold, err := multilevel.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		a.SetCache(cold)
		key := mapKey
		key.LayerName = testLayer2.Name
		separate := []byte("separately regenerated layer")
		if err = cold.Set(context.Background(), &key, separate); err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		server.TileCacheHandler(a, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("layer rendered") })).ServeHTTP(w, httptest.NewRequest("GET", "/maps/"+testMapName+"/"+testLayer2.Name+"/10/2/3.pbf", nil))
		if !bytes.Equal(w.Body.Bytes(), separate) {
			t.Fatal("map tile replaced newer layer")
		}
		if err = cold.Purge(context.Background(), &key); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("expired seeded file is not reused", func(t *testing.T) {
		old := time.Now().Add(-48 * time.Hour)
		if err = os.Chtimes(filepath.Join(root, mapKey.String()), old, old); err != nil {
			t.Fatal(err)
		}
		cold, err := multilevel.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		a.SetCache(cold)
		called := false
		server.TileCacheHandler(a, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/maps/"+testMapName+"/"+testLayer2.Name+"/10/2/3.pbf", nil))
		if !called {
			t.Fatal("expired file reused")
		}
	})
}
