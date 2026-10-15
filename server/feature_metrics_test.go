package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/observability"
	metricprovider "github.com/alexeydott/tegola/observability/prometheus"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
)

type featureMetricQuerier struct{ panic bool }

func (*featureMetricQuerier) FeatureQueryExecutionInfo() (provider.FeatureQueryExecutionMetadata, error) {
	return provider.FeatureQueryExecutionMetadata{Backend: provider.FeatureQueryBackendGPKG, ScalarFilter: provider.FeatureFilterExecutionSQL}, nil
}

func (q *featureMetricQuerier) QueryFeatures(_ context.Context, _ string, _ provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	if q.panic {
		panic("private business panic")
	}
	if err := fn(&provider.Feature{ID: 1, SRID: 4326, Geometry: geom.Point{1, 2}}); err != nil {
		return provider.FeatureQueryResult{}, err
	}
	return provider.FeatureQueryResult{NumberReturned: 1}, nil
}

func TestFeatureMetricsInstalledEndpointAndFinalStatuses(t *testing.T) {
	preserveDiscoveryGlobals(t)
	installed, err := metricprovider.New(dict.Dict{})
	if err != nil {
		t.Fatal(err)
	}
	a := &atlas.Atlas{}
	a.SetObservability(installed)
	q := &featureMetricQuerier{}
	service, err := features.NewService([]features.CollectionSource{{ID: "public", Layer: discoveryLayer{}, Querier: q}})
	if err != nil {
		t.Fatal(err)
	}
	api, err := NewFeatureAPI(service, FeatureAPIConfig{BasePath: "/features", DefaultLimit: 1, MaxLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	router, err := NewRouterWithOptions(a, RouterOptions{Features: api})
	if err != nil {
		t.Fatal(err)
	}
	if router.featureAPI == api || api.requestObserver != nil {
		t.Fatal("router mutated original API observer binding")
	}
	for _, test := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/features/collections", 200},
		{"HEAD", "/features/collections", 200},
		{"OPTIONS", "/features/collections/public/items", 200},
		{"POST", "/features/collections/public/items", 405},
		{"GET", "/features/collections/public/items", 200},
		{"GET", "/features/collections/public/items/1", 200},
		{"GET", "/features/collections/public/items?limit=0", 400},
		{"GET", "/features/unknown-sensitive-name", 404},
		{"GET", "/features/collections/public/items?" + strings.Repeat("x", 65537), 414},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != test.status {
			t.Fatal(test.path, response.Code, response.Body.String())
		}
	}
	q.panic = true
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/features/collections/public/items", nil))
	if response.Code != 500 || strings.Contains(response.Body.String(), "private") {
		t.Fatal(response.Code, response.Body.String())
	}
	for i := 0; i < 32; i++ {
		response = httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/features/unmatched-attacker-"+strconv.Itoa(i), nil))
		if response.Code != 404 {
			t.Fatal(response.Code)
		}
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	body := response.Body.String()
	for _, text := range []string{
		`tegola_feature_requests_total{method="GET",resource="items",status="500"} 1`,
		`tegola_feature_requests_total{method="GET",resource="items",status="200"} 1`,
		`tegola_feature_requests_total{method="HEAD",resource="collections",status="200"} 1`,
		`tegola_feature_requests_total{method="OPTIONS",resource="items",status="200"} 1`,
		`tegola_feature_requests_total{method="other",resource="items",status="405"} 1`,
		`tegola_feature_requests_total{method="GET",resource="items",status="414"} 1`,
		`tegola_feature_provider_queries_total{backend="gpkg",class="unfiltered",outcome="error",pushdown="none"} 1`,
		`tegola_feature_rows_returned_total{backend="gpkg"} 2`,
		`tegola_feature_requests_total{method="GET",resource="unknown",status="404"} 33`,
	} {
		if !strings.Contains(body, text) {
			t.Fatal("missing final endpoint metric", text, body)
		}
	}
	// The generic legacy API metrics may contain path templates. Only new feature
	// series are asserted to have no configured names or raw path/query data.
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "tegola_api_duration_seconds") && (strings.Contains(line, "features") || strings.Contains(line, "attacker")) {
			t.Fatal("legacy dynamic labels recorded dedicated feature traffic", line)
		}
		if strings.HasPrefix(line, "tegola_feature_") && (strings.Contains(line, "public") || strings.Contains(line, "sensitive") || strings.Contains(line, "limit=")) {
			t.Fatal("feature label leaked request data", line)
		}
	}
}

type panicFeatureRequestObserver struct{}

func (panicFeatureRequestObserver) ObserveFeatureRequest(observability.FeatureRequestObservation) {
	panic("private observer value")
}

func TestFeatureRequestObserverPanicDoesNotReplaceResponse(t *testing.T) {
	api := discoveryAPI(t)
	api.requestObserver = panicFeatureRequestObserver{}
	response := httptest.NewRecorder()
	api.serveObservedFeature(response, httptest.NewRequest("GET", "/features/api", nil), "/features", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("accepted"))
	}))
	if response.Code != 200 || response.Body.String() != "accepted" {
		t.Fatal(response.Code, response.Body.String())
	}
}
