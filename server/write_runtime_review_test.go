//go:build cgo

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/cache/memory"
	"github.com/alexeydott/tegola/config"
)

func TestWritableRouterIgnoresPreStartupCachedTiles(t *testing.T) {
	preserveDiscoveryGlobals(t)
	oldAge := TileHTTPMaxAge
	TileHTTPMaxAge = 300
	t.Cleanup(func() { TileHTTPMaxAge = oldAge })
	svc, _ := part4Service(t)
	api := part4API(t, svc)
	a := &atlas.Atlas{}
	c, err := memory.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	a.SetCache(c)
	key := &cache.Key{MapName: "deleted-map", Z: 1, X: 0, Y: 0}
	if err := c.Set(context.Background(), key, []byte("stale tile")); err != nil {
		t.Fatal(err)
	}
	router, err := NewRouterWithOptions(a, RouterOptions{Features: api})
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(method, "/maps/deleted-map/1/0/0.pbf", nil))
		if w.Code == http.StatusOK || w.Header().Get("Tegola-Cache") == "HIT" {
			t.Fatalf("write runtime served stale cache: %d %v", w.Code, w.Header())
		}
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("write runtime cache policy = %q", got)
		}
	}
}

func TestNewFeatureAPIRejectsUnknownWriteAuth(t *testing.T) {
	svc, _ := part4Service(t)
	api := part4API(t, svc)
	cfg := api.cfg
	cfg.Write.AuthMode = "prodution"
	if _, err := NewFeatureAPI(svc, cfg); err == nil {
		t.Fatal("invalid write auth accepted by Go API")
	}
}

func TestRouterRejectsInvalidWFSRuntime(t *testing.T) {
	preserveDiscoveryGlobals(t)
	svc, _ := part4Service(t)
	api := part4API(t, svc)
	for _, tc := range []struct {
		name string
		opts RouterOptions
	}{
		{"nil service", RouterOptions{WFS: &WFSHandler{}}},
		{"reserved path", RouterOptions{WFS: &WFSHandler{Service: svc, Config: config.WFSConfig{BasePath: "/maps"}}}},
		{"unsupported version", RouterOptions{WFS: &WFSHandler{Service: svc, Config: config.WFSConfig{Versions: []string{"9.0"}}}}},
		{"overlapping paths", RouterOptions{Features: api, WFS: &WFSHandler{Service: svc, Config: config.WFSConfig{BasePath: "/features"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateRouterOptions(tc.opts); err == nil {
				t.Fatal("invalid WFS runtime accepted")
			}
		})
	}
}
