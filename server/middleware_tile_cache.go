package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/go-spatial/geom/encoding/mvt"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/internal/log"
)

// TileCacheHandler implements a request cache for tiles on requests when the URLs
// have a /:z/:x/:y scheme suffix (i.e. /osm/1/3/4.pbf)
func TileCacheHandler(a *atlas.Atlas, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error

		// check if a cache backend exists
		cacher := a.GetCache()
		if cacher == nil {
			// nope. move on
			next.ServeHTTP(w, r)
			return
		}

		// ignore requests with query parameters
		if r.URL.RawQuery != "" {
			next.ServeHTTP(w, r)
			return
		}

		// parse our URI into a cache key structure (remove any configured URIPrefix + "maps/" )
		key, err := cache.ParseKey(strings.TrimPrefix(r.URL.Path, path.Join(URIPrefix, "maps")))
		if err != nil {
			log.Errorf("cache middleware: ParseKey err: %v", err)
			next.ServeHTTP(w, r)
			return
		}

		// use the URL path as the key
		cachedTile, hit, err := cacher.Get(r.Context(), key)
		if err != nil {
			log.Errorf("cache middleware: error reading from cache: %v", err)
			next.ServeHTTP(w, r)
			return
		}

		// cache miss
		if !hit {
			// buffer which will hold a copy of the response for writing to the cache
			var buff bytes.Buffer

			// overwrite our current responseWriter with a tileCacheResponseWriter
			w = newTileCacheResponseWriter(w, &buff)

			next.ServeHTTP(w, r)

			// check if our request context has been canceled
			if r.Context().Err() != nil {
				return
			}

			// if nothing has been written to the buffer, don't write to the cache
			if buff.Len() == 0 {
				return
			}

			if err := cacher.Set(r.Context(), key, buff.Bytes()); err != nil {
				log.Warnf("cache response writer err: %v", err)
			}
			return
		}

		// mimetype for mapbox vector tiles
		w.Header().Add("Content-Type", mvt.MimeType)

		// communicate the cache is being used
		w.Header().Add("Tegola-Cache", "HIT")
		w.Header().Add("Content-Length", fmt.Sprintf("%d", len(cachedTile)))

		w.Write(cachedTile)
		return
	})
}

func newTileCacheResponseWriter(resp http.ResponseWriter, w io.Writer) http.ResponseWriter {
	return &tileCacheResponseWriter{
		resp:  resp,
		multi: io.MultiWriter(w, resp),
	}
}

// tileCacheResponseWriter wraps http.ResponseWriter (https://golang.org/pkg/net/http/#ResponseWriter)
// to additionally write the response to a cache when there is a cache MISS
type tileCacheResponseWriter struct {
	// status response code
	status int
	resp   http.ResponseWriter
	multi  io.Writer
}

func (w *tileCacheResponseWriter) Header() http.Header {
	// communicate the cache is being used
	w.resp.Header().Set("Tegola-Cache", "MISS")

	return w.resp.Header()
}

func (w *tileCacheResponseWriter) Write(b []byte) (int, error) {
	// only write to the multi writer when http response == StatusOK
	if w.status == http.StatusOK {

		// write to our multi writer
		return w.multi.Write(b)
	}

	// write to the original response writer
	return w.resp.Write(b)
}

func (w *tileCacheResponseWriter) WriteHeader(i int) {
	w.status = i

	w.resp.WriteHeader(i)
}
