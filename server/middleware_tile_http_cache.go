package server

import (
	"mime"
	"net/http"
	"strconv"

	"github.com/alexeydott/geom/encoding/mvt"
)

// TileHTTPCacheHandler applies browser cache policy after rendering/cache lookup
// and content encoding have selected the final response status. It belongs only
// on tile routes, outside GZipHandler, so decompression errors are not cacheable.
func TileHTTPCacheHandler(maxAge int, next http.Handler) http.Handler {
	if maxAge <= 0 {
		return next
	}
	policy := "public, max-age=" + strconv.Itoa(maxAge)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ordinary := (r.Method == http.MethodGet || r.Method == http.MethodHead) && r.URL.RawQuery == "" && !r.URL.ForceQuery &&
			r.Header.Get("Authorization") == "" && r.Header.Get("Cookie") == ""
		out := &tileHTTPResponseWriter{ResponseWriter: w, ordinary: ordinary, policy: policy}
		next.ServeHTTP(out, r)
		if !out.wroteHeader {
			out.WriteHeader(http.StatusOK)
		}
	})
}

type tileHTTPResponseWriter struct {
	http.ResponseWriter
	ordinary    bool
	policy      string
	wroteHeader bool
}

func mutableTileNoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out := &tileHTTPResponseWriter{ResponseWriter: w}
		next.ServeHTTP(out, r)
		if !out.wroteHeader {
			out.WriteHeader(http.StatusOK)
		}
	})
}

func (w *tileHTTPResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *tileHTTPResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	if status >= 100 && status < 200 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	w.wroteHeader = true
	contentType, _, _ := mime.ParseMediaType(w.Header().Get("Content-Type"))
	// Override generic configured cache headers on this route, including errors
	// and privileged/query-dependent responses. Keep Vary: Accept-Encoding.
	w.Header().Del("Expires")
	w.Header().Set("Cache-Control", "no-store")
	if w.ordinary && status == http.StatusOK && contentType == mvt.MimeType {
		w.Header().Set("Cache-Control", w.policy)
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *tileHTTPResponseWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}
