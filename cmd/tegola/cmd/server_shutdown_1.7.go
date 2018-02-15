// +build !go1.8

package cmd

import (
	"net/http"
)

// On anything before 1.8; we just shutdown the program, and graceful shutdown.
func shutdown(srv *http.Server) {}
