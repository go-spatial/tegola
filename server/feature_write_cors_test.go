//go:build cgo

package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFeatureWriteCORSPreflight(t *testing.T) {
	service, _ := part4Service(t)
	for _, mode := range []string{"all", "update", "readonly"} {
		t.Run(mode, func(t *testing.T) {
			api := part4API(t, service)
			if mode == "readonly" {
				api.cfg.Write.Enabled = false
			} else if mode == "update" {
				api.cfg.Write.Collections[0].Operations = []string{"update"}
			}
			srv := httptest.NewServer(part4Router(t, api))
			defer srv.Close()
			for _, item := range []bool{false, true} {
				path := "/features/collections/sites/items"
				want := "GET, HEAD, OPTIONS"
				if item {
					path += "/1"
					if mode == "all" {
						want += ", PUT, PATCH, DELETE"
					} else if mode == "update" {
						want += ", PATCH"
					}
				} else if mode == "all" {
					want += ", POST"
				}
				r := phase09Request(t, srv, http.MethodOptions, path, http.Header{
					"Origin":                         {"http://127.0.0.1:8081"},
					"Access-Control-Request-Method":  {"PATCH"},
					"Access-Control-Request-Headers": {"content-type,content-crs,if-match"},
				})
				if r.status != http.StatusOK && r.status != http.StatusNoContent {
					t.Fatalf("preflight status=%d body=%s", r.status, r.body)
				}
				if got := r.header.Get("Access-Control-Allow-Methods"); got != want {
					t.Errorf("%s CORS methods=%q want=%q (Allow=%q)", path, got, want, r.header.Get("Allow"))
				}
				if mode != "readonly" {
					for _, h := range []string{"content-type", "content-crs", "if-match"} {
						if !strings.Contains(strings.ToLower(r.header.Get("Access-Control-Allow-Headers")), h) {
							t.Errorf("preflight missing allowed header %s", h)
						}
					}
				}
			}
		})
	}
}
