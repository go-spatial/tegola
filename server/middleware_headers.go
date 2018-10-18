package server

import (
	"net/http"
	"strings"
)

func HeadersHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		for name, value := range Headers {
			// skip CORS headers
			if strings.ToLower(name) == "access-control-allow-origin" {
				continue
			}
			if strings.ToLower(name) == "access-control-allow-methods" {
				continue
			}

			v, ok := value.(string)
			if ok {
				w.Header().Set(name, v)
			}
		}

		next.ServeHTTP(w, r)

		return
	})
}
