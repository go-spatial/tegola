package prometheus

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/go-spatial/tegola/observability"

	"github.com/go-spatial/tegola/dict"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type byteSize uint64

const (
	b  byteSize = 1
	kb byteSize = 1 << (10 * iota)
	mb
	gb
)

const (
	Name = "prometheus"

	httpAPI    = "tegola_api"
	httpViewer = "tegola_viewer"
)

func init() {
	err := observability.Register(Name, New)
	if err != nil {
		panic(err)
	}
}

type observer struct {
	// URLPrefix is the server's prefix
	URLPrefix string

	// observeVars are the vars (:foo) in a url that should be turned into a label
	// Default values for this via new is []string{":map_name",":layer_name",":z"}
	observeVars []string

	httpHandlers map[string]*httpHandler
	registry     prometheus.Registerer
}

func New(config dict.Dicter) (observability.Interface, error) {
	// We don't have anything for now for the config
	var obs observer
	obs.registry = prometheus.DefaultRegisterer
	obs.httpHandlers = make(map[string]*httpHandler)

	obs.observeVars, _ = config.StringSlice("variables")
	if len(obs.observeVars) == 0 {
		obs.observeVars = []string{":map_name", ":layer_name", ":z"}
	}

	return &obs, nil
}

func (*observer) Name() string { return Name }

func (observer) Handler(string) http.Handler { return promhttp.Handler() }

func (obs *observer) InstrumentedAPIHttpHandler(method, route string, next http.Handler) http.Handler {
	if obs == nil {
		return next
	}
	handler := obs.httpHandlers[httpAPI]
	if handler == nil {
		// need to initialize the handler
		handler = newHttpHandler(obs.registry, httpAPI, obs.URLPrefix, obs.observeVars)
		obs.httpHandlers[httpAPI] = handler
	}
	return handler.InstrumentedHttpHandler(method, route, next)
}

func (obs *observer) InstrumentedViewerHttpHandler(method, route string, next http.Handler) http.Handler {
	if obs == nil {
		return next
	}
	handler := obs.httpHandlers[httpViewer]
	if handler == nil {
		// need to initialize the handler
		handler = newHttpHandler(obs.registry, httpViewer, obs.URLPrefix, obs.observeVars)
		obs.httpHandlers[httpViewer] = handler
	}
	return handler.InstrumentedHttpHandler(method, route, next)
}
