package prometheus

import (
	"net/http"

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
)

func init() {
	err := observability.Register(Name, New)
	if err != nil {
		panic(err)
	}
}

type observer struct {
}

func New(config dict.Dicter) (observability.Interface, error) {
	// We don't have anything for now for the config
	var obs observer

	return &obs, nil
}

func (observer) Handler(string) http.Handler { return promhttp.Handler() }

func (obs *observer) InstrumentedHttpHandler(method, route string, handler http.Handler) http.Handler {
	return handler
}

func (*observer) Name() string { return Name }
