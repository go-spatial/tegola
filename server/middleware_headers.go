package server

import "net/http"

// HeadersHandler is middleware for adding user defined response headers
func HeadersHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// default CORS headers. may be overwritten by the user
		w.Header().Set("Access-Control-Allow-Origin", CORSAllowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", CORSAllowedMethods)

		addUserDefinedHeaders(w)

		next.ServeHTTP(w, r)

		return
	})
}

func addUserDefinedHeaders(w http.ResponseWriter) {
	for name, value := range Headers {
		v, ok := value.(string)
		if ok {
			w.Header().Set(name, v)
		}
	}
}
