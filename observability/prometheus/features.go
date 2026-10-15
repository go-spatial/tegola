package prometheus

import (
	"strconv"

	"github.com/alexeydott/tegola/observability"
	"github.com/alexeydott/tegola/ogc/features"

	"github.com/prometheus/client_golang/prometheus"
)

type featureMetrics struct {
	queries         *prometheus.CounterVec
	queryDuration   *prometheus.HistogramVec
	rows            *prometheus.CounterVec
	requests        *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
}

func newFeatureMetrics(registry prometheus.Registerer) *featureMetrics {
	labels := []string{"backend", "class", "pushdown", "outcome"}
	buckets := []float64{.001, .005, .01, .05, .1, .5, 1, 5, 15, 30, 60}
	metrics := &featureMetrics{
		queries:         prometheus.NewCounterVec(prometheus.CounterOpts{Name: "tegola_feature_provider_queries_total", Help: "Raw feature provider query pipelines invoked."}, labels),
		queryDuration:   prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "tegola_feature_provider_query_duration_seconds", Help: "Provider query pipeline duration, including synchronous service encoding and caller callbacks; not database-only execution time.", Buckets: buckets}, labels),
		rows:            prometheus.NewCounterVec(prometheus.CounterOpts{Name: "tegola_feature_rows_returned_total", Help: "Feature rows successfully accepted by callbacks, including partial rows before query failure."}, []string{"backend"}),
		requests:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "tegola_feature_requests_total", Help: "Completed feature HTTP requests, separate from tile traffic."}, []string{"resource", "method", "status"}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "tegola_feature_request_duration_seconds", Help: "Completed feature HTTP pipeline durations.", Buckets: buckets}, []string{"resource", "method", "status"}),
	}
	registry.MustRegister(metrics.queries, metrics.queryDuration, metrics.rows, metrics.requests, metrics.requestDuration)
	return metrics
}

func boundedLabel[T ~uint8](value T, labels []string) string {
	if int(value) >= len(labels) {
		return labels[0]
	}
	return labels[int(value)]
}

func (obs *observer) ObserveFeatureQuery(event features.QueryObservation) {
	if obs == nil || obs.features == nil {
		return
	}
	backend := boundedLabel(event.Backend, []string{"unknown", "gpkg", "postgis", "mysql", "hana"})
	class := boundedLabel(event.Class, []string{"unknown", "unfiltered", "id", "bbox", "datetime", "filter", "crs", "mixed"})
	pushdown := boundedLabel(event.Pushdown, []string{"unknown", "none", "sql_filter"})
	outcome := boundedLabel(event.Outcome, []string{"unknown", "ok", "canceled", "deadline", "invalid", "unsupported", "source_error", "callback_error", "error"})
	seconds := event.Duration.Seconds()
	if seconds < 0 {
		seconds = 0
	}
	obs.features.queries.WithLabelValues(backend, class, pushdown, outcome).Inc()
	obs.features.queryDuration.WithLabelValues(backend, class, pushdown, outcome).Observe(seconds)
	obs.features.rows.WithLabelValues(backend).Add(float64(event.RowsReturned))
}

func (obs *observer) ObserveFeatureRequest(event observability.FeatureRequestObservation) {
	if obs == nil || obs.features == nil {
		return
	}
	resource := boundedLabel(event.Resource, []string{"unknown", "landing", "api", "conformance", "collections", "collection", "queryables", "items", "item"})
	method := boundedLabel(event.Method, []string{"other", "GET", "HEAD", "OPTIONS"})
	status := "other"
	switch event.Status {
	case 200, 301, 400, 404, 405, 406, 408, 414, 431, 500, 501:
		status = strconv.Itoa(event.Status)
	}
	seconds := event.Duration.Seconds()
	if seconds < 0 {
		seconds = 0
	}
	obs.features.requests.WithLabelValues(resource, method, status).Inc()
	obs.features.requestDuration.WithLabelValues(resource, method, status).Observe(seconds)
}

var _ features.QueryObserver = (*observer)(nil)
var _ observability.FeatureRequestObserver = (*observer)(nil)
