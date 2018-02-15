// +build go1.8

package cmd

import (
	"context"
	"net/http"
	"time"

	gdcmd "github.com/gdey/cmd"
)

func shutdown(srv *http.Server) {
	gdcmd.OnComplete(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel() // releases resources if slowOperation completes before timeout elapses
		srv.Shutdown(ctx)
	})

}
