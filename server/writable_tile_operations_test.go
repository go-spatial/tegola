//go:build cgo

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/cache/memory"
	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/provider/test"
)

func TestWritableRouterTileMaintenance(t *testing.T) {
	for _, protocol := range []string{"features", "wfs"} {
		t.Run(protocol, func(t *testing.T) {
			preserveDiscoveryGlobals(t)
			oldOps := TileOperations
			TileOperations = TileOperationsConfig{Enabled: true, Token: "maintenance-test", RatePerMinute: 1000, MaxConcurrent: 100}
			t.Cleanup(func() { AwaitMetatileRegeneration(); TileOperations = oldOps })
			svc, _ := part4Service(t)
			api := part4API(t, svc)
			options := RouterOptions{Features: api}
			if protocol == "wfs" {
				options = RouterOptions{WFS: &WFSHandler{Service: svc, Config: config.WFSConfig{Enabled: true}.Resolved(), WriteConfig: api.cfg.Write}}
			}
			a := &atlas.Atlas{}
			m := atlas.NewWebMercatorMap("maintenance-map")
			m.Layers = []atlas.Layer{{Name: "points", ProviderLayerName: "test-layer-1", MinZoom: 0, MaxZoom: 22, Provider: &test.TileProvider{}, GeomType: geom.Point{}}}
			a.AddMap(m)
			c, err := memory.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			a.SetCache(c)
			router, err := NewRouterWithOptions(a, options)
			if err != nil {
				t.Fatal(err)
			}
			const uri = "/maps/maintenance-map/points/4/2/3.pbf"
			key := &cache.Key{MapName: "maintenance-map", LayerName: "points", Z: 4, X: 2, Y: 3}
			stale := []byte("stale persisted tile")
			if err := c.Set(context.Background(), key, stale); err != nil {
				t.Fatal(err)
			}
			request := func(query, token string) *httptest.ResponseRecorder {
				r := httptest.NewRequest(http.MethodGet, uri+query, nil)
				r.Header.Set(TileOperationsTokenHeader, token)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, r)
				return w
			}
			for _, op := range []string{"status", "update", "getupdated"} {
				if w := request("?tile="+op, ""); w.Code != http.StatusForbidden {
					t.Fatalf("unauthenticated %s: %d", op, w.Code)
				}
			}
			assertOrdinaryCached := func(want string) {
				t.Helper()
				w := request("", "")
				if w.Code != http.StatusOK || bytes.Equal(w.Body.Bytes(), stale) || w.Header().Get("Tegola-Cache") != want || w.Header().Get("Cache-Control") != "no-store" {
					t.Fatalf("ordinary writable read cache mismatch: %d %v %q", w.Code, w.Header(), w.Body.Bytes())
				}
			}
			assertOrdinaryCached("MISS")
			w := request("?tile=status", "maintenance-test")
			var status tileStatusResponse
			if w.Code != http.StatusOK {
				t.Fatalf("status: %d %s", w.Code, w.Body.String())
			}
			if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
				t.Fatal(err)
			}
			if !status.Cached {
				t.Fatal("status ignored configured cache")
			}
			w = request("?tile=update", "maintenance-test")
			if w.Code != http.StatusAccepted {
				t.Fatalf("update: %d %s", w.Code, w.Body.String())
			}
			AwaitMetatileRegeneration()
			for y := uint(0); y < 8; y++ {
				for x := uint(0); x < 8; x++ {
					r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/maps/maintenance-map/points/4/%d/%d.pbf?tile=status", x, y), nil)
					r.Header.Set(TileOperationsTokenHeader, "maintenance-test")
					w := httptest.NewRecorder()
					router.ServeHTTP(w, r)
					var state tileStatusResponse
					if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil || w.Code != 200 || !state.Cached {
						t.Fatalf("metatile %d/%d: %d %s", x, y, w.Code, w.Body.String())
					}
				}
			}
			if err := c.Set(context.Background(), key, stale); err != nil {
				t.Fatal(err)
			}
			w = request("?tile=getupdated", "maintenance-test")
			if w.Code != http.StatusOK || w.Body.Len() == 0 || bytes.Equal(w.Body.Bytes(), stale) {
				t.Fatalf("getupdated: %d %q", w.Code, w.Body.Bytes())
			}
			AwaitMetatileRegeneration()
			assertOrdinaryCached("HIT")
		})
	}
}
