package observability

import (
	"net/http"

	"github.com/go-spatial/tegola/dict"
)

var NullObserver nullObserverType

func noneInit(dict.Dicter) (Interface, error) { return NullObserver, nil }

type nullObserverType struct{}

// Handler does nothing.
func (nullObserverType) Handler(string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { return })
}

// InstrumentedHttpHandler does not do anything and just returns the handler
func (nullObserverType) InstrumentedHttpHandler(_, _ string, handler http.Handler) http.Handler {
	return handler
}

func (nullObserverType) Name() string { return "none" }
