package server

import (
	"io"
	"log"
	"net/http"
)

func CacheHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//	check if we have a cache backend initialized
		if Cache == nil {
			//	nope. move on
			next.ServeHTTP(w, r)
			return
		}

		//	use the URL path as the key
		cachedTile, err := Cache.Get(r.URL.Path)
		if err != nil {
			//	TODO: this should be a debug warning
			log.Printf("cache err: %v", err)
			cWriter, err := Cache.GetWriter(r.URL.Path)
			if err != nil {
				log.Printf("cache newWriter err: %v", err)
			}

			//	ovewrite our current response writer with the cache writer
			w = newCacheResponseWriter(w, cWriter)

		} else {
			//	TODO: how configurable do we want the CORS policy to be?
			//	set CORS header
			w.Header().Add("Access-Control-Allow-Origin", "*")

			//	mimetype for protocol buffers
			w.Header().Add("Content-Type", "application/x-protobuf")

			//	communicate the cache is being used
			w.Header().Add("Tegola-Cache", "HIT")

			io.Copy(w, cachedTile)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func newCacheResponseWriter(resp http.ResponseWriter, writers ...io.Writer) http.ResponseWriter {
	//	communicate the cache is being used
	resp.Header().Add("Tegola-Cache", "MISS")

	writers = append(writers, resp)

	return &cacheResponseWriter{
		resp:  resp,
		multi: io.MultiWriter(writers...),
	}
}

type cacheResponseWriter struct {
	resp  http.ResponseWriter
	multi io.Writer
}

// implement http.ResponseWriter
// https://golang.org/pkg/net/http/#ResponseWriter
func (w *cacheResponseWriter) Header() http.Header {
	return w.resp.Header()
}

func (w *cacheResponseWriter) Write(b []byte) (int, error) {
	return w.multi.Write(b)
}

func (w *cacheResponseWriter) WriteHeader(i int) {
	w.resp.WriteHeader(i)
}
