package observer

import (
	"net/http"

	"github.com/go-spatial/tegola/cache"
)

type Null struct{}

// Handler does nothing.
func (Null) Handler(string) http.Handler { return nil }

func (Null) Init()     {}
func (Null) Shutdown() {}

// InstrumentedAPIHttpHandler does not do anything and just returns the handler
func (Null) InstrumentedAPIHttpHandler(_, _ string, handler http.Handler) http.Handler {
	return handler
}

// InstrumentedViewerHttpHandler does not do anything and just returns the handler
func (Null) InstrumentedViewerHttpHandler(_, _ string, handler http.Handler) http.Handler {
	return handler
}

func (Null) Name() string { return "none" }

func (Null) InstrumentedCache(c cache.Interface) cache.Interface { return c }
