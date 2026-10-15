package prometheus

import (
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/observability"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
)

func TestFeatureMetricsPrivateScrapeAndBoundedLabels(t *testing.T) {
	first, err := New(dict.Dict{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(dict.Dict{})
	if err != nil {
		t.Fatal(err)
	}
	observer := first.(*observer)
	observer.ObserveFeatureQuery(features.QueryObservation{Backend: provider.FeatureQueryBackendGPKG, Class: features.QueryClassFilter, Pushdown: features.QueryPushdownSQLFilter, Outcome: features.QueryOutcomeOK, Duration: time.Millisecond, RowsReturned: 3})
	observer.ObserveFeatureQuery(features.QueryObservation{Backend: 255, Class: 255, Pushdown: 255, Outcome: 255, Duration: -time.Second})
	observer.ObserveFeatureRequest(observability.FeatureRequestObservation{Resource: 255, Method: 255, Status: 999, Duration: -time.Second})
	response := httptest.NewRecorder()
	first.Handler("/metrics").ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	body := response.Body.String()
	for _, text := range []string{
		`tegola_feature_provider_queries_total{backend="gpkg",class="filter",outcome="ok",pushdown="sql_filter"} 1`,
		`tegola_feature_provider_queries_total{backend="unknown",class="unknown",outcome="unknown",pushdown="unknown"} 1`,
		`tegola_feature_rows_returned_total{backend="gpkg"} 3`,
		`tegola_feature_requests_total{method="other",resource="unknown",status="other"} 1`,
		`including synchronous service encoding and caller callbacks`,
	} {
		if !strings.Contains(body, text) {
			t.Fatal("missing scraped feature metric", text, body)
		}
	}
	if strings.Contains(body, `status="999"`) || strings.Contains(body, `backend="255"`) {
		t.Fatal("unbounded enum escaped")
	}
	response = httptest.NewRecorder()
	second.Handler("/metrics").ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	if strings.Contains(response.Body.String(), "tegola_feature_provider_queries_total{") {
		t.Fatal("registry instances shared measurements")
	}
}

func TestFeatureMetricsConcurrentDelivery(t *testing.T) {
	installed, err := New(dict.Dict{})
	if err != nil {
		t.Fatal(err)
	}
	observer := installed.(*observer)
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			observer.ObserveFeatureQuery(features.QueryObservation{Backend: provider.FeatureQueryBackendMySQL, Class: features.QueryClassID, Pushdown: features.QueryPushdownNone, Outcome: features.QueryOutcomeOK, RowsReturned: 1})
			observer.ObserveFeatureRequest(observability.FeatureRequestObservation{Resource: observability.FeatureResourceItem, Method: observability.FeatureMethodGET, Status: 200})
		}()
	}
	wg.Wait()
	response := httptest.NewRecorder()
	installed.Handler("/metrics").ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	for _, text := range []string{`tegola_feature_rows_returned_total{backend="mysql"} 64`, `tegola_feature_requests_total{method="GET",resource="item",status="200"} 64`} {
		if !strings.Contains(response.Body.String(), text) {
			t.Fatal(text, response.Body.String())
		}
	}
}
