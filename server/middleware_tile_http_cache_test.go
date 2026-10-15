package server_test

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexeydott/geom/encoding/mvt"
	"github.com/alexeydott/tegola/cache/memory"
	"github.com/alexeydott/tegola/server"
)

func TestTileHTTPCachePolicy(t *testing.T) {
	for _, tc := range []struct {
		name, query, contentType, auth, cookie, expected string
		status, maxAge                                   int
	}{
		{name: "tile", status: 200, maxAge: 300, contentType: mvt.MimeType, expected: "public, max-age=300"},
		{name: "disabled", status: 200, contentType: mvt.MimeType, expected: "private, max-age=10"},
		{name: "error", status: 500, maxAge: 300, contentType: mvt.MimeType, expected: "no-store"},
		{name: "missing", status: 404, maxAge: 300, expected: "no-store"},
		{name: "empty", status: 204, maxAge: 300, expected: "no-store"},
		{name: "status", query: "?tile=status", status: 200, maxAge: 300, contentType: "application/json", expected: "no-store"},
		{name: "update", query: "?tile=update", status: 202, maxAge: 300, expected: "no-store"},
		{name: "getupdated", query: "?tile=getupdated", status: 200, maxAge: 300, contentType: mvt.MimeType, expected: "no-store"},
		{name: "dirty", query: "?dirty=true", status: 200, maxAge: 300, contentType: mvt.MimeType, expected: "no-store"},
		{name: "parameter", query: "?v=1", status: 200, maxAge: 300, contentType: mvt.MimeType, expected: "no-store"},
		{name: "empty query", query: "?", status: 200, maxAge: 300, contentType: mvt.MimeType, expected: "no-store"},
		{name: "auth", auth: "Bearer test", status: 200, maxAge: 300, contentType: mvt.MimeType, expected: "no-store"},
		{name: "cookie", cookie: "session=test", status: 200, maxAge: 300, contentType: mvt.MimeType, expected: "no-store"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.Header().Set("Cache-Control", "private, max-age=10")
				w.Header().Set("Expires", "Wed, 21 Oct 2037 07:28:00 GMT")
				w.WriteHeader(tc.status)
			})
			r := httptest.NewRequest("GET", "/maps/test/1/0/0.pbf"+tc.query, nil)
			r.Header.Set("Authorization", tc.auth)
			r.Header.Set("Cookie", tc.cookie)
			w := httptest.NewRecorder()
			server.TileHTTPCacheHandler(tc.maxAge, next).ServeHTTP(w, r)
			if got := w.Header().Get("Cache-Control"); got != tc.expected {
				t.Fatalf("Cache-Control = %q, want %q", got, tc.expected)
			}
			if tc.maxAge > 0 && w.Header().Get("Expires") != "" {
				t.Fatal("stale Expires retained")
			}
			if w.Code != tc.status {
				t.Fatalf("status = %d", w.Code)
			}
		})
	}
}

func TestTileHTTPCacheRouterHitMiss(t *testing.T) {
	oldAge, oldPrefix := server.TileHTTPMaxAge, server.URIPrefix
	defer func() { server.TileHTTPMaxAge, server.URIPrefix = oldAge, oldPrefix }()
	server.TileHTTPMaxAge, server.URIPrefix = 300, "/"
	for _, uri := range []string{"/maps/test-map/10/2/3.pbf", "/maps/test-map/test-layer/4/2/3.pbf"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			for _, encoding := range []string{"", "gzip"} {
				a := newTestMapWithLayers(testLayer1, testLayer2, testLayer3)
				c, _ := memory.New(nil)
				a.SetCache(c)
				router := server.NewRouter(a)
				var first []byte
				for _, expected := range []string{"MISS", "HIT"} {
					r := httptest.NewRequest(method, uri, nil)
					r.Header.Set("Accept-Encoding", encoding)
					w := httptest.NewRecorder()
					router.ServeHTTP(w, r)
					if w.Code != 200 || w.Header().Get("Tegola-Cache") != expected || w.Header().Get("Cache-Control") != "public, max-age=300" {
						t.Fatalf("%s %s: status %d headers %v", uri, expected, w.Code, w.Header())
					}
					if w.Header().Get("Vary") != "Accept-Encoding" {
						t.Fatal("encoding variation lost")
					}
					if first == nil {
						first = append([]byte(nil), w.Body.Bytes()...)
					} else if !bytes.Equal(first, w.Body.Bytes()) {
						t.Fatal("HIT body changed")
					}
				}
			}
		}
	}

}

func TestTileHTTPCacheGzipFailure(t *testing.T) {
	for _, valid := range []bool{true, false} {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", mvt.MimeType)
			if valid {
				var b bytes.Buffer
				z := gzip.NewWriter(&b)
				_, _ = z.Write([]byte("tile"))
				_ = z.Close()
				_, _ = w.Write(b.Bytes())
			} else {
				_, _ = w.Write([]byte("invalid gzip"))
			}
		})
		w := httptest.NewRecorder()
		server.TileHTTPCacheHandler(300, server.GZipHandler(next)).ServeHTTP(w, httptest.NewRequest("GET", "/maps/test/1/0/0.pbf", nil))
		if valid && w.Header().Get("Cache-Control") != "public, max-age=300" {
			t.Fatal("implicit 200 not cached")
		}
		if !valid && (w.Code != 500 || w.Header().Get("Cache-Control") != "no-store") {
			t.Fatalf("gzip error cached: %d %v", w.Code, w.Header())
		}
	}
}
