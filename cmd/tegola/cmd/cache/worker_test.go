package cache

import (
	"bytes"
	"context"
	"testing"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/cache/memory"
	"github.com/go-spatial/tegola/internal/env"
	"github.com/go-spatial/tegola/provider/test"
)

const testMapName = "test-map"

func TestMain(m *testing.M) {
	testMap := atlas.NewWebMercatorMap(testMapName)
	testMap.Layers = append(testMap.Layers, atlas.Layer{
		Name:              "test-layer",
		ProviderLayerName: "test-layer",
		MinZoom:           0,
		MaxZoom:           20,
		Provider:          &test.TileProvider{},
		GeomType:          geom.Point{},
		DefaultTags:       env.Dict{},
	})
	atlas.AddMap(testMap)

	cacher, err := memory.New(nil)
	if err != nil {
		panic(err)
	}
	atlas.SetCache(cacher)

	m.Run()
}

// TestSeedWorkerSucceeds verifies the happy path of seedWorker — the exact
// code path where runtime.GC() used to be called — completes without error.
func TestSeedWorkerSucceeds(t *testing.T) {
	mt := MapTile{
		MapName: testMapName,
		Tile:    slippy.Tile{Z: 1, X: 1, Y: 1},
	}

	worker := seedWorker(true, 0)
	if err := worker(context.Background(), mt); err != nil {
		t.Fatalf("seedWorker returned unexpected error: %v", err)
	}
}

func TestSeedWorkerNoOverwriteSkipsExistingTile(t *testing.T) {
	mt := MapTile{
		MapName: testMapName,
		Tile:    slippy.Tile{Z: 1, X: 2, Y: 3},
	}
	z, x, y := mt.Tile.ZXY()
	key := cache.Key{
		MapName: mt.MapName,
		Z:       uint(z),
		X:       x,
		Y:       y,
	}

	expected := []byte("already-cached")
	c := atlas.GetCache()
	if c == nil {
		t.Fatal("expected cache to be configured")
	}
	if err := c.Set(context.Background(), &key, expected); err != nil {
		t.Fatalf("failed to prefill cache: %v", err)
	}

	worker := seedWorker(false, 0)
	if err := worker(context.Background(), mt); err != nil {
		t.Fatalf("seedWorker returned unexpected error: %v", err)
	}

	got, hit, err := c.Get(context.Background(), &key)
	if err != nil {
		t.Fatalf("failed reading cache after worker run: %v", err)
	}
	if !hit {
		t.Fatalf("expected cache hit for key %+v", key)
	}
	if !bytes.Equal(got, expected) {
		t.Fatalf("cache value changed for existing tile: got %q want %q", got, expected)
	}
}

func TestSeedWorkerNoOverwriteSeedsOnCacheMiss(t *testing.T) {
	mt := MapTile{
		MapName: testMapName,
		Tile:    slippy.Tile{Z: 1, X: 3, Y: 4},
	}
	z, x, y := mt.Tile.ZXY()
	key := cache.Key{
		MapName: mt.MapName,
		Z:       uint(z),
		X:       x,
		Y:       y,
	}

	c := atlas.GetCache()
	if c == nil {
		t.Fatal("expected cache to be configured")
	}
	if err := c.Purge(context.Background(), &key); err != nil {
		t.Fatalf("failed to purge cache key before test: %v", err)
	}

	worker := seedWorker(false, 0)
	if err := worker(context.Background(), mt); err != nil {
		t.Fatalf("seedWorker returned unexpected error: %v", err)
	}

	got, hit, err := c.Get(context.Background(), &key)
	if err != nil {
		t.Fatalf("failed reading cache after worker run: %v", err)
	}
	if !hit {
		t.Fatalf("expected cache miss to be seeded for key %+v", key)
	}
	if len(got) == 0 {
		t.Fatal("expected seeded cache value to be non-empty")
	}
}
