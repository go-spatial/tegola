package server

import "net/http"

// HeadersHandler is middleware for adding user defined response headers
func HeadersHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// set default and user defined headers
		setHeaders(w)
		// move on
		next.ServeHTTP(w, r)
		return
	})
}
