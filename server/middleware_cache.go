package server

import "net/http"

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
			//	log.Printf("cache err: %v", err)

			//	ovewrite our current responseWriter with a cacheResponseWriter
			w = &cacheResponseWriter{
				cacheKey: r.URL.Path,
				resp:     w,
			}

		} else {
			//	TODO: how configurable do we want the CORS policy to be?
			//	set CORS header
			w.Header().Add("Access-Control-Allow-Origin", "*")

			//	mimetype for protocol buffers
			w.Header().Add("Content-Type", "application/x-protobuf")

			//	communicate the cache is being used
			w.Header().Add("Tegola-Cache", "HIT")

			w.Write(cachedTile)
			return
		}

		next.ServeHTTP(w, r)
	})
}

//	cacheResponsWriter wraps http.ResponseWriter (https://golang.org/pkg/net/http/#ResponseWriter)
//	to also write the response to a cache when there is a cache MISS
type cacheResponseWriter struct {
	cacheKey string
	resp     http.ResponseWriter
}

func (w *cacheResponseWriter) Header() http.Header {
	//	communicate the tegola cache is being used
	w.resp.Header().Add("Tegola-Cache", "MISS")

	return w.resp.Header()
}

func (w *cacheResponseWriter) Write(b []byte) (int, error) {
	//	after we write the response, persist the data to the cache
	defer Cache.Set(w.cacheKey, b)

	return w.resp.Write(b)
}

func (w *cacheResponseWriter) WriteHeader(i int) {
	w.resp.WriteHeader(i)
}
