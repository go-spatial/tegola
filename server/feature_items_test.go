package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
)

func TestFeatureItemsParameters(t *testing.T) {
	api := discoveryAPI(t)
	for _, raw := range []string{"unknown=1", "limit=1&limit=2", "limit=0", "limit=-1", "limit=10001", "offset=18446744073709551616", "limit=%zz", "limit=1;offset=2", "bbox=1,2,3", "bbox=1,2,3,4,5", "bbox=NaN,0,1,1", "bbox=-181,0,1,1", "bbox=0,3,1,2", "bbox=0,0,5,1,1,4", "datetime=../..", "datetime=2020-01-01T1:00:00Z", "datetime=2020-01-01T00:00:00%2B24:00", "datetime=2020-01-01T00:00:00%2B00:60", "datetime=2020-01-01T00:00:00,1Z", "datetime=2020-01-02T00:00:00Z/2020-01-01T00:00:00Z"} {
		t.Run(raw, func(t *testing.T) {
			if _, _, err := api.parseItemsQuery(raw); err == nil {
				t.Fatal("invalid query accepted")
			}
		})
	}
	// Over-maximum limits are rejected, not silently clamped: the clamp could
	// still blow the response budget with a 500.
	if _, _, err := api.parseItemsQuery("limit=999999999999999999999999&offset=3"); err == nil {
		t.Fatal("over-maximum limit accepted")
	}
	q, _, err := api.parseItemsQuery("limit=10000&offset=3&bbox=170,-10,-5,-170,10,9")
	if err != nil {
		t.Fatal(err)
	}
	if q.Limit != 10000 || q.Offset != 3 || q.BoundsSRID != 4326 || q.BoundsVerticalCRS != provider.CRS84h || len(q.Bounds3D) != 2 || q.Bounds3D[0] != (provider.Extent3D{170, -10, -5, 180, 10, 9}) || q.Bounds3D[1] != (provider.Extent3D{-180, -10, -5, -170, 10, 9}) {
		t.Fatalf("unexpected query: %#v", q)
	}
	q, _, err = api.parseItemsQuery("bbox=170,-10,-170,10&datetime=../2020-01-01t00:00:00z")
	if err != nil || len(q.Bounds) != 2 || q.Temporal.Start != nil || q.Temporal.End == nil {
		t.Fatalf("query=%#v err=%v", q, err)
	}
}

type itemsQuerier struct {
	query provider.FeatureQuery
	err   error
}

func (q *itemsQuerier) QueryFeatures(ctx context.Context, layer string, query provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	q.query = query
	if q.err != nil {
		return provider.FeatureQueryResult{}, q.err
	}
	id := uint64(7)
	if len(query.IDs) > 0 {
		id = query.IDs[0]
		if id == 99 {
			return provider.FeatureQueryResult{}, nil
		}
	}
	if err := fn(&provider.Feature{ID: id, SRID: 4326, Geometry: geom.Point{15, 0}, Tags: map[string]interface{}{"name": "example"}}); err != nil {
		return provider.FeatureQueryResult{}, err
	}
	return provider.FeatureQueryResult{NumberReturned: 1, HasMore: len(query.IDs) == 0}, nil
}

func TestFeatureItemsHTTP(t *testing.T) {
	preserveDiscoveryGlobals(t)
	backend := &itemsQuerier{}
	service, err := features.NewService([]features.CollectionSource{{ID: "alpha", Layer: discoveryLayer{}, Querier: backend}})
	if err != nil {
		t.Fatal(err)
	}
	api, err := NewFeatureAPI(service, FeatureAPIConfig{BasePath: "/features", DefaultLimit: 1, MaxLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	router, err := NewRouterWithOptions(nil, RouterOptions{Features: api})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/features/collections/alpha/items?limit=10&offset=3", 200},
		{"/features/collections/alpha/items?limit=100&offset=3", 400},
		{"/features/collections/alpha/items/7", 200},
		{"/features/collections/alpha/items/99", 404},
		{"/features/collections/missing/items", 404},
		{"/features/collections/alpha/items/-1", 400},
		{"/features/collections/alpha/items/18446744073709551616", 400},
		{"/features/collections/alpha/items/7?limit=1", 400},
		{"/features/collections/alpha/items?unknown=1", 400},
	} {
		t.Run(tc.path, func(t *testing.T) {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(method, tc.path, nil))
				if response.Code != tc.status || response.Header().Get("Cache-Control") != "no-store" {
					t.Fatalf("%s: %d %s", method, response.Code, response.Body.String())
				}
				if method == http.MethodHead && response.Body.Len() != 0 {
					t.Fatal("HEAD body")
				}
				if tc.status == 200 && response.Header().Get("Content-Type") != "application/geo+json" {
					t.Fatal("wrong media type")
				}
				if method == http.MethodGet && tc.status == 200 {
					var payload map[string]interface{}
					if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
	backend.err = errors.New("SQL secret token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/features/collections/alpha/items", nil))
	if response.Code != 500 || strings.Contains(response.Body.String(), "secret") {
		t.Fatal("backend error leaked")
	}
}

func TestFeatureItemsPagingLinks(t *testing.T) {
	preserveDiscoveryGlobals(t)
	backend := &itemsQuerier{}
	service, err := features.NewService([]features.CollectionSource{{ID: "alpha", Layer: discoveryLayer{}, Querier: backend}})
	if err != nil {
		t.Fatal(err)
	}
	api, err := NewFeatureAPI(service, FeatureAPIConfig{BasePath: "/features", DefaultLimit: 1, MaxLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	router, err := NewRouterWithOptions(nil, RouterOptions{Features: api})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/features/collections/alpha/items?limit=10&offset=3&bbox=170,-10,-170,10&datetime=2020-01-01T00%3A00%3A00%2B02%3A00", nil))
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	var body struct {
		Links          []featureLink `json:"links"`
		NumberReturned uint64        `json:"numberReturned"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.NumberReturned != 1 || len(body.Links) != 5 || backend.query.Limit != 10 || backend.query.Offset != 3 || len(backend.query.Bounds) != 2 {
		t.Fatalf("body=%#v query=%#v", body, backend.query)
	}
	for relation, offset := range map[string]string{"self": "3", "next": "4", "prev": "0"} {
		var href string
		for _, link := range body.Links {
			if link.Rel == relation && link.Type == "application/geo+json" {
				href = link.Href
			}
		}
		if href == "" {
			t.Fatalf("missing %s link", relation)
		}
		parsed, err := url.Parse(href)
		if err != nil {
			t.Fatal(err)
		}
		values := parsed.Query()
		if values.Get("f") != "json" || values.Get("offset") != offset || values.Get("limit") != "10" || values.Get("bbox") != "170,-10,-170,10" || values.Get("datetime") != "2020-01-01T00:00:00+02:00" {
			t.Fatalf("lost query semantics: %s", href)
		}
	}
	if strings.Contains(response.Body.String(), "HasMore") || strings.Contains(response.Body.String(), "hasMore") {
		t.Fatal("internal page metadata leaked")
	}
	for _, tc := range []struct {
		err    error
		status int
	}{
		{provider.ErrUnsupported, 501}, {context.Canceled, 408}, {context.DeadlineExceeded, 408},
		{provider.InvalidFeatureQueryError{Field: "secret", Reason: "private backend data"}, 400},
	} {
		backend.err = tc.err
		response = httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/features/collections/alpha/items", nil))
		if response.Code != tc.status || strings.Contains(response.Body.String(), "private") {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	}
}

func TestFeatureDatetimeExactBoundaries(t *testing.T) {
	for _, tc := range []struct {
		raw, tail string
		leap      bool
		nano      int
	}{
		{"2020-01-01T00:00:00.0000000001Z", "1", false, 0},
		{"2020-01-01t00:00:00.123456789000001000z", "000001", false, 123456789},
		{"1969-12-31T23:59:59.9999999991Z", "1", false, 999999999},
		{"2016-12-31T23:59:60.2500000001Z", "1", true, 250000000},
		{"2017-01-01T00:59:60.25+01:00", "", true, 250000000},
		{"2020-01-01T00:00:00.000000000000000000000000000Z", "", false, 0},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			q, err := parseFeatureDatetime(tc.raw)
			if err != nil {
				t.Fatal(err)
			}
			if q.Start.Nanosecond() != tc.nano || q.StartSubNanosecond != tc.tail || q.EndSubNanosecond != tc.tail || q.StartLeapSecond != tc.leap || q.EndLeapSecond != tc.leap || !q.Start.Equal(*q.End) {
				t.Fatalf("lost precision: %#v", q)
			}
		})
	}
	for _, raw := range []string{
		"2020-01-01T00:00:00.0000000002Z/2020-01-01T00:00:00.0000000001Z",
		"2016-12-31T23:59:60Z/2016-12-31T23:59:59.999999999Z",
		"2017-01-01T00:00:00Z/2016-12-31T23:59:60.999999999Z",
		"2016-12-30T23:59:60Z", "2020-01-01T23:59:60Z", "2016-12-31T23:59:61Z",
	} {
		if _, err := parseFeatureDatetime(raw); err == nil {
			t.Fatalf("invalid exact boundary accepted: %s", raw)
		}
	}
	for _, raw := range []string{
		"2016-12-31T23:59:59.999999999999999999Z/2016-12-31T23:59:60Z",
		"2016-12-31T23:59:60.999999999999999999Z/2017-01-01T00:00:00Z",
		"../2016-12-31T23:59:60.0000000001Z", "2016-12-31T23:59:60.0000000001Z/..",
	} {
		if _, err := parseFeatureDatetime(raw); err != nil {
			t.Fatalf("valid exact boundary rejected: %s: %v", raw, err)
		}
	}
}

func TestFeatureSourceErrorClassification(t *testing.T) {
	api := discoveryAPI(t)
	for _, tc := range []struct {
		err    error
		status int
	}{
		{provider.FeatureDataError{Err: provider.InvalidFeatureQueryError{Field: "private-source", Reason: "secret-row-value"}}, 500},
		{provider.FeatureDataError{Err: provider.ErrUnsupported}, 500},
		{provider.FeatureDataError{Err: context.Canceled}, 408},
		{provider.FeatureDataError{Err: context.DeadlineExceeded}, 408},
		{provider.InvalidFeatureQueryError{Field: "limit", Reason: "secret-client-value"}, 400},
		{provider.ErrUnsupported, 501},
	} {
		for _, method := range []string{"GET", "HEAD"} {
			response := httptest.NewRecorder()
			api.writeQueryError(response, httptest.NewRequest(method, "/features/collections/alpha/items", nil), fmt.Errorf("outer-context: %w", tc.err))
			if response.Code != tc.status || strings.Contains(response.Body.String(), "secret") || strings.Contains(response.Body.String(), "private-source") || strings.Contains(response.Body.String(), "outer-context") {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if method == "HEAD" && response.Body.Len() != 0 {
				t.Fatal("error HEAD has body")
			}
		}
	}
}
