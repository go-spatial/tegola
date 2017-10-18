package server

import (
	"log"
	"net/http"

	"github.com/terranodo/tegola/cache"
)

//	TileCacheHandler implements a request cache for tiles on requests when the URLs
//	have a /:z/:x/:y scheme suffix (i.e. /osm/1/3/4.pbf)
func TileCacheHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error

		//	check if a cache backend exists
		if Cache == nil {
			//	nope. move on
			next.ServeHTTP(w, r)
			return
		}

		//	parse our URI into a cache key structure
		key, err := cache.ParseKey(r.URL.Path)
		if err != nil {
			log.Println("cache middleware: ParseKey err: %v", err)
			next.ServeHTTP(w, r)
			return
		}

		//	use the URL path as the key
		cachedTile, hit, err := Cache.Get(key)
		if err != nil {
			log.Printf("cache middleware: error reading from cache: %v", err)
			next.ServeHTTP(w, r)
			return
		}
		//	cache miss
		if !hit {
			//	ovewrite our current responseWriter with a tileCacheResponseWriter
			w = &tileCacheResponseWriter{
				cacheKey: key,
				resp:     w,
			}
			next.ServeHTTP(w, r)
			return
		}

		//	TODO: how configurable do we want the CORS policy to be?
		//	set CORS header
		w.Header().Add("Access-Control-Allow-Origin", "*")

		//	mimetype for protocol buffers
		w.Header().Add("Content-Type", "application/x-protobuf")

		//	communicate the cache is being used
		w.Header().Add("Tegola-Cache", "HIT")

		w.Write(cachedTile)
		return
	})
}

//	cacheResponsWriter wraps http.ResponseWriter (https://golang.org/pkg/net/http/#ResponseWriter)
//	to also write the response to a cache when there is a cache MISS
type tileCacheResponseWriter struct {
	cacheKey *cache.Key
	resp     http.ResponseWriter
}

func (w *tileCacheResponseWriter) Header() http.Header {
	//	communicate the tegola cache is being used
	w.resp.Header().Set("Tegola-Cache", "MISS")

	return w.resp.Header()
}

func (w *tileCacheResponseWriter) Write(b []byte) (int, error) {
	//	after we write the response, persist the data to the cache
	//	use anonymous function to output the error
	defer func(key *cache.Key) {
		if Cache == nil {
			return
		}
		if err := Cache.Set(key, b); err != nil {
			log.Println("cache response writer err: %v", err)
		}
	}(w.cacheKey)

	return w.resp.Write(b)
}

func (w *tileCacheResponseWriter) WriteHeader(i int) {
	w.resp.WriteHeader(i)
}
