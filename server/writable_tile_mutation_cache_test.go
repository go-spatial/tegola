//go:build cgo

package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/cache/memory"
	"github.com/alexeydott/tegola/config"
)

func TestWritableTileCacheInvalidatesRESTAndWFS(t *testing.T) {
	for _, protocol := range []string{"REST", "WFS"} {
		t.Run(protocol, func(t *testing.T) {
			preserveDiscoveryGlobals(t)
			svc, gp := part4Service(t)
			api := part4API(t, svc)
			a := &atlas.Atlas{}
			c, err := memory.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			a.SetCache(c)
			m := atlas.NewWebMercatorMap("test")
			m.Layers = []atlas.Layer{{Name: "sites", ProviderLayerName: "sites", Provider: gp, MaxZoom: 22}}
			a.AddMap(m)
			opts := RouterOptions{Features: api}
			if protocol == "WFS" {
				opts.WFS = &WFSHandler{Service: svc, Config: config.WFSConfig{Enabled: true}.Resolved(), WriteConfig: api.cfg.Write}
			}
			router, err := NewRouterWithOptions(a, opts)
			if err != nil {
				t.Fatal(err)
			}
			tile := func(editor bool) *httptest.ResponseRecorder {
				r := httptest.NewRequest(http.MethodGet, "/maps/test/sites/0/0/0.pbf", nil)
				if editor {
					r.Header.Set(EditorActiveHeader, "true")
				}
				w := httptest.NewRecorder()
				router.ServeHTTP(w, r)
				if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
					t.Fatalf("tile %d %v %s", w.Code, w.Header(), w.Body.String())
				}
				return w
			}
			before := tile(false)
			if before.Header().Get("Tegola-Cache") != "MISS" {
				t.Fatal(before.Header())
			}
			if w := tile(false); w.Header().Get("Tegola-Cache") != "HIT" {
				t.Fatal(w.Header())
			}
			if w := tile(true); w.Header().Get("Tegola-Cache") == "HIT" {
				t.Fatal("editor served persisted tile")
			}
			if w := tile(false); w.Header().Get("Tegola-Cache") != "HIT" {
				t.Fatal("editor damaged viewing cache")
			}
			path, body, contentType := "/features/collections/sites/items", `{"type":"Feature","geometry":{"type":"Point","coordinates":[0,0]},"properties":{"name":"fresh","seats":2}}`, "application/geo+json"
			want := http.StatusCreated
			if protocol == "WFS" {
				path = "/wfs"
				contentType = "application/xml"
				want = 200
				body = `<wfs:Transaction xmlns:wfs="http://www.opengis.net/wfs/2.0" xmlns:app="http://example.com/tegola/sites" xmlns:gml="http://www.opengis.net/gml/3.2" service="WFS" version="2.0.0"><wfs:Insert><app:sites><app:geom><gml:Point srsName="EPSG:4326"><gml:pos>0 0</gml:pos></gml:Point></app:geom><app:name>fresh</app:name><app:seats>2</app:seats></app:sites></wfs:Insert></wfs:Transaction>`
			}
			r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
			r.Header.Set("Content-Type", contentType)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != want {
				t.Fatalf("mutation %d %s", w.Code, w.Body.String())
			}
			after := tile(false)
			if after.Header().Get("Tegola-Cache") != "MISS" || bytes.Equal(before.Body.Bytes(), after.Body.Bytes()) {
				t.Fatalf("stale mutation tile: %v", after.Header())
			}
			if hit := tile(false); hit.Header().Get("Tegola-Cache") != "HIT" || !bytes.Equal(hit.Body.Bytes(), after.Body.Bytes()) {
				t.Fatal("fresh tile not cached")
			}
		})
	}
}
