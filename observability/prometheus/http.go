package prometheus

import (
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type httpHandler struct {
	observeVars       []string
	URLPrefix         string
	inFlightGauge     prometheus.Gauge
	counter           *prometheus.CounterVec
	durationSeconds   *prometheus.HistogramVec
	responseSizeBytes *prometheus.HistogramVec
}

var (
	httpHandlerDurationBuckets     = []float64{.25, .5, 1, 2.5, 5, 10}
	httpHandlerResponseSizeBuckets = []float64{float64(500 * kb), float64(1 * mb), float64(5 * mb)}
)

func newHttpHandler(registry prometheus.Registerer, prefix string, URLPrefix string, observeVars []string) *httpHandler {
	handler := httpHandler{
		observeVars: observeVars,
		URLPrefix:   URLPrefix,
	}

	handler.inFlightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: prefix + "_flight_requests",
		Help: "A gauge of requests currently being served by the wrapped handler.",
	})

	handler.counter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prefix + "_requests_total",
			Help: "A counter for requests to the wrapped handler.",
		},
		[]string{"code", "method"},
	)

	durationLabels := []string{"handler", "method"}
	// duration is partitioned by the HTTP method and handler. It uses custom
	// buckets based on the expected request duration.
	handler.durationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    prefix + "_duration_seconds",
			Help:    "A histogram of latencies for requests.",
			Buckets: httpHandlerDurationBuckets,
		},
		durationLabels,
	)

	// responseSize has no labels, making it a zero-dimensional
	// ObserverVec.
	handler.responseSizeBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    prefix + "_response_size_bytes",
			Help:    "A histogram of response sizes for requests.",
			Buckets: httpHandlerResponseSizeBuckets,
		},
		[]string{},
	)

	registry.MustRegister(
		handler.inFlightGauge,
		handler.counter,
		handler.durationSeconds,
		handler.responseSizeBytes,
	)

	return &handler
}

func (handler *httpHandler) instrumentHandlerDuration(originalRoute string, next http.Handler) http.HandlerFunc {
	type partKeyVal struct {
		Name  string
		Index int
	}
	var partsMap []partKeyVal
	parts := strings.Split(originalRoute, "/")
	if len(handler.observeVars) > 0 {
		partsMap = make([]partKeyVal, 0, len(handler.observeVars))
	partsLoop:
		for i, part := range parts {
			if !strings.HasPrefix(part, ":") {
				continue
			}
			for _, lbl := range handler.observeVars {
				if part == lbl {
					continue partsLoop
				}
			}
			partsMap = append(partsMap, partKeyVal{Name: part, Index: i})
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var routeURL = r.URL.Path

		parts := strings.Split(strings.TrimPrefix(routeURL, handler.URLPrefix), "/")

		for _, val := range partsMap {
			parts[val.Index] = val.Name
		}
		labels := prometheus.Labels{
			"handler": strings.Join(parts, "/"),
		}
		promhttp.InstrumentHandlerDuration(handler.durationSeconds.MustCurryWith(labels), next).ServeHTTP(w, r)
	})
}

func (handler *httpHandler) InstrumentedHttpHandler(_, route string, next http.Handler) http.Handler {
	return promhttp.InstrumentHandlerInFlight(handler.inFlightGauge,
		handler.instrumentHandlerDuration(route,
			promhttp.InstrumentHandlerCounter(handler.counter,
				promhttp.InstrumentHandlerResponseSize(handler.responseSizeBytes, next),
			),
		),
	)
}
