package observer

import "net/http"

type Null struct{}

// Handler does nothing.
func (Null) Handler(string) http.Handler { return nil }

// InstrumentedAPIHttpHandler does not do anything and just returns the handler
func (Null) InstrumentedAPIHttpHandler(_, _ string, handler http.Handler) http.Handler {
	return handler
}

// InstrumentedViewerHttpHandler does not do anything and just returns the handler
func (Null) InstrumentedViewerHttpHandler(_, _ string, handler http.Handler) http.Handler {
	return handler
}

func (Null) Name() string { return "none" }
