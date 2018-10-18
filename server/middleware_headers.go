package server

import "net/http"

func HeadersHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// default CORS headers. may be overwritten by the user
		w.Header().Set("Access-Control-Allow-Origin", CORSAllowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", CORSAllowedMethods)

		for name, value := range Headers {
			v, ok := value.(string)
			if ok {
				w.Header().Set(name, v)
			}
		}

		next.ServeHTTP(w, r)

		return
	})
}
