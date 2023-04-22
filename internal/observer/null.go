package observer

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/go-spatial/tegola/cache"
)

type Null struct{}

// Handler does nothing.
func (Null) Handler(string) http.Handler { return nil }

func (Null) Init()                                           {}
func (Null) Shutdown()                                       {}
func (Null) MustRegister(_ ...prometheus.Collector)          {}
func (Null) CollectorConfig(_ string) map[string]interface{} { return make(map[string]interface{}) }

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
