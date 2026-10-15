package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/observability"
)

type featureMetricsResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *featureMetricsResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *featureMetricsResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *featureMetricsResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func featureResource(path, base string) observability.FeatureResource {
	suffix := strings.TrimPrefix(path, base)
	switch suffix {
	case "", "/":
		return observability.FeatureResourceLanding
	case "/api":
		return observability.FeatureResourceAPI
	case "/conformance":
		return observability.FeatureResourceConformance
	case "/collections":
		return observability.FeatureResourceCollections
	}
	rest, ok := strings.CutPrefix(suffix, "/collections/")
	if !ok {
		return observability.FeatureResourceUnknown
	}
	_, rest, hasRest := strings.Cut(rest, "/")
	if !hasRest {
		return observability.FeatureResourceCollection
	}
	if rest == "queryables" {
		return observability.FeatureResourceQueryables
	}
	if rest == "items" {
		return observability.FeatureResourceItems
	}
	if id, ok := strings.CutPrefix(rest, "items/"); ok && id != "" && !strings.Contains(id, "/") {
		return observability.FeatureResourceItem
	}
	return observability.FeatureResourceUnknown
}

func featureMethod(method string) observability.FeatureMethod {
	switch method {
	case http.MethodGet:
		return observability.FeatureMethodGET
	case http.MethodHead:
		return observability.FeatureMethodHEAD
	case http.MethodOptions:
		return observability.FeatureMethodOPTIONS
	default:
		return observability.FeatureMethodOther
	}
}

func (api *FeatureAPI) observeRequest(event observability.FeatureRequestObservation) {
	defer func() {
		if recover() != nil {
			log.Error("feature request observer failed")
		}
	}()
	api.requestObserver.ObserveFeatureRequest(event)
}

func (api *FeatureAPI) serveObservedFeature(w http.ResponseWriter, r *http.Request, base string, next http.Handler) {
	if api.requestObserver == nil {
		api.serveFeatureBuffered(w, r, next)
		return
	}
	writer := &featureMetricsResponseWriter{ResponseWriter: w}
	started := time.Now()
	api.serveFeatureBuffered(writer, r, next)
	status := writer.status
	if status == 0 {
		status = http.StatusOK
	}
	api.observeRequest(observability.FeatureRequestObservation{Resource: featureResource(r.URL.Path, base), Method: featureMethod(r.Method), Status: status, Duration: time.Since(started)})
}
