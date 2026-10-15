package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
)

type transportQueryableLayer struct {
	discoveryLayer
	mapping provider.TemporalMapping
	fields  []provider.FeatureQueryable
	err     error
}

func (l *transportQueryableLayer) TemporalMapping() (provider.TemporalMapping, error) {
	return l.mapping, nil
}

func (l *transportQueryableLayer) FeatureQueryables() (provider.FeatureQueryables, error) {
	if l.err != nil {
		return provider.FeatureQueryables{}, l.err
	}
	return provider.NewFeatureQueryables(l.fields)
}

type transportFilterQuerier struct {
	calls atomic.Int32
	query provider.FeatureQuery
}

func (q *transportFilterQuerier) QueryFeatures(ctx context.Context, _ string, query provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	q.calls.Add(1)
	q.query = query
	if err := ctx.Err(); err != nil {
		return provider.FeatureQueryResult{}, err
	}
	if err := fn(&provider.Feature{ID: 7, SRID: 4326, Geometry: geom.Point{15, 0}, Tags: map[string]any{"s": "selected"}}); err != nil {
		return provider.FeatureQueryResult{}, err
	}
	return provider.FeatureQueryResult{NumberReturned: 1, HasMore: true}, nil
}
func transportFilterRouter(t *testing.T) (*Router, *transportFilterQuerier) {
	t.Helper()
	backend := &transportFilterQuerier{}
	sources := []features.CollectionSource{
		{ID: "available", Title: "Available", Layer: &transportQueryableLayer{mapping: provider.TemporalMapping{InstantField: "time"}, fields: []provider.FeatureQueryable{{Name: "s", Type: provider.QueryableString, Nullable: true}, {Name: "n", Type: provider.QueryableInteger}, {Name: "b", Type: provider.QueryableBoolean, Nullable: true}, {Name: "date", Type: provider.QueryableDate}, {Name: "stamp", Type: provider.QueryableTimestamp}, {Name: "bad name", Type: provider.QueryableString}}}, Querier: backend},
		{ID: "empty", Layer: &transportQueryableLayer{}, Querier: backend},
		{ID: "missingcap", Layer: discoveryLayer{}, Querier: backend},
		{ID: "unsupported", Layer: &transportQueryableLayer{err: provider.ErrUnsupported}, Querier: backend},
	}
	service, err := features.NewService(sources)
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
	return router, backend
}
func TestFeatureQueryablesSchemaHEADAndPrefixes(t *testing.T) {
	preserveDiscoveryGlobals(t)
	URIPrefix = "/proxy"
	URLRoot = func(*http.Request) *url.URL {
		return &url.URL{Scheme: "https", Host: "gateway.example", Path: "stage", RawQuery: "ignored=1"}
	}
	router, backend := transportFilterRouter(t)
	path := "/proxy/features/collections/available/queryables"
	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, path, nil))
	if get.Code != 200 || get.Header().Get("Content-Type") != "application/schema+json" || get.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(get.Code, get.Header())
	}
	var doc map[string]any
	if err := json.Unmarshal(get.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["$id"] != "https://gateway.example/stage/proxy/features/collections/available/queryables" || doc["$schema"] != "https://json-schema.org/draft/2020-12/schema" || doc["additionalProperties"] != false || doc["type"] != "object" {
		t.Fatal(doc)
	}
	properties := doc["properties"].(map[string]any)
	if len(properties) != 5 || properties["bad name"] != nil {
		t.Fatal("unaddressable property advertised", properties)
	}
	if properties["n"].(map[string]any)["type"] != "integer" || properties["date"].(map[string]any)["format"] != "date" || properties["stamp"].(map[string]any)["format"] != "date-time" {
		t.Fatal(properties)
	}
	nullable := properties["s"].(map[string]any)["type"].([]any)
	if len(nullable) != 2 || nullable[0] != "string" || nullable[1] != "null" {
		t.Fatal(nullable)
	}
	head := httptest.NewRecorder()
	router.ServeHTTP(head, httptest.NewRequest(http.MethodHead, path, nil))
	if head.Code != get.Code || head.Body.Len() != 0 || head.Header().Get("Content-Length") != get.Header().Get("Content-Length") || head.Header().Get("Content-Type") != get.Header().Get("Content-Type") {
		t.Fatal(head.Code, head.Header())
	}
	empty := httptest.NewRecorder()
	router.ServeHTTP(empty, httptest.NewRequest(http.MethodGet, "/proxy/features/collections/empty/queryables", nil))
	if empty.Code != 200 || !strings.Contains(empty.Body.String(), `"properties":{}`) {
		t.Fatal(empty.Code, empty.Body.String())
	}
	if backend.calls.Load() != 0 {
		t.Fatal("metadata queried source data")
	}
}
func TestFeatureQueryablesNegotiationErrorsAndLinks(t *testing.T) {
	preserveDiscoveryGlobals(t)
	router, backend := transportFilterRouter(t)
	cases := []struct {
		path, method, accept string
		status               int
	}{
		{"/features/collections/available/queryables", "GET", "application/schema+json", 200},
		{"/features/collections/available/queryables", "GET", "application/json", 406},
		{"/features/collections/available/queryables", "HEAD", "application/schema+json;q=0,text/html;q=0,*/*;q=1", 406},
		{"/features/collections/available/queryables", "GET", "application/*", 200},
		{"/features/collections/available/queryables?filter=TRUE", "GET", "", 400},
		{"/features/collections/missingcap/queryables", "GET", "", 501},
		{"/features/collections/unsupported/queryables", "HEAD", "", 501},
		{"/features/collections/missing/queryables", "GET", "", 404},
		{"/features/collections/available/queryables", "POST", "", 405},
	}
	for _, c := range cases {
		t.Run(c.path+c.method+c.accept, func(t *testing.T) {
			r := httptest.NewRequest(c.method, c.path, nil)
			if c.accept != "" {
				r.Header.Set("Accept", c.accept)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != c.status || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(w.Code, w.Header(), w.Body.String())
			}
			if c.method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("HEAD body")
			}
		})
	}
	for _, id := range []string{"available", "empty", "missingcap", "unsupported"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/features/collections/"+id, nil))
		var doc featureCollectionDescription
		if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, link := range doc.Links {
			if link.Rel == "http://www.opengis.net/def/rel/ogc/1.0/queryables" {
				found = true
				if link.Type != "application/schema+json" || !strings.HasSuffix(link.Href, "/queryables?f=json") {
					t.Fatal(link)
				}
			}
		}
		if found != (id == "available" || id == "empty") {
			t.Fatal("availability link mismatch", id, found)
		}
	}
	redirect := httptest.NewRecorder()
	router.ServeHTTP(redirect, httptest.NewRequest("GET", "/features/collections/available/queryables/", nil))
	if redirect.Code < 300 || redirect.Code >= 400 || redirect.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(redirect.Code, redirect.Header())
	}
	if backend.calls.Load() != 0 {
		t.Fatal("metadata errors queried source")
	}
}
func TestFeatureFilterHTTPRejectsBeforeSource(t *testing.T) {
	preserveDiscoveryGlobals(t)
	router, backend := transportFilterRouter(t)
	invalid := []string{"filter=", "filter=%20", "filter=TRUE&filter=FALSE", "filter-lang=cql2-text", "filter=TRUE&filter-lang=", "filter=TRUE&filter-lang=cql2-json", "filter=TRUE&filter-lang=cql2-text&filter-lang=cql2-text", "filter=TRUE&filter-crs=CRS84", "filter=n%20%3D%20%27wrong%27", "filter=private%20%3D%201", "filter=%22bad%20name%22%20%3D%20%27x%27", "filter=n%20IN%20(1,2)", "filter=%zz"}
	for _, raw := range invalid {
		for _, method := range []string{"GET", "HEAD"} {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(method, "/features/collections/available/items?"+raw, nil))
			if w.Code != 400 || w.Header().Get("Cache-Control") != "no-store" || backend.calls.Load() != 0 {
				t.Fatal(raw, w.Code, backend.calls.Load())
			}
			if method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("HEAD body")
			}
		}
	}
	for _, c := range []struct {
		id     string
		status int
	}{{"missingcap", 501}, {"unsupported", 501}, {"missing", 404}} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/features/collections/"+c.id+"/items?filter=TRUE", nil))
		if w.Code != c.status || backend.calls.Load() != 0 {
			t.Fatal(c, w.Code)
		}
	}
}
func TestFeatureFilterHTTPPreservesQueryAndPaging(t *testing.T) {
	preserveDiscoveryGlobals(t)
	router, backend := transportFilterRouter(t)
	filter := `n > 0.5 AND s IS NOT NULL`
	datetime := "2016-12-31T23:59:60.123456789123Z/.."
	values := url.Values{"filter": {filter}, "filter-lang": {"cql2-text"}, "bbox": {"170,-10,-170,10"}, "datetime": {datetime}, "offset": {"2"}, "limit": {"1"}}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/features/collections/available/items?"+values.Encode(), nil))
	if w.Code != 200 || backend.calls.Load() != 1 || backend.query.Filter == nil || len(backend.query.Bounds) != 2 || backend.query.Temporal == nil || !backend.query.Temporal.StartLeapSecond || backend.query.Temporal.StartSubNanosecond != "123" {
		t.Fatal(w.Code, w.Body.String(), backend.query)
	}
	var doc struct{ Links []featureLink }
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Links) != 5 {
		t.Fatal(doc.Links)
	}
	for _, link := range doc.Links {
		target, err := url.Parse(link.Href)
		if err != nil {
			t.Fatal(err)
		}
		q := target.Query()
		if link.Rel == "item" {
			if len(q) != 1 || q.Get("f") != "json" {
				t.Fatal("collection-only constraints leaked into item link", link)
			}
			continue
		}
		if q.Get("filter") != filter || q.Get("filter-lang") != "cql2-text" || q.Get("bbox") != values.Get("bbox") || q.Get("datetime") != datetime {
			t.Fatal("paging constraints changed", link)
		}
	}
	// Missing language uses cql2-text, with no language manufactured in links.
	plain := httptest.NewRecorder()
	router.ServeHTTP(plain, httptest.NewRequest("GET", "/features/collections/available/items?filter=TRUE", nil))
	if plain.Code != 200 || strings.Contains(plain.Body.String(), "filter-lang=") {
		t.Fatal(plain.Code, plain.Body.String())
	}
}
func TestFeatureFilterHTTPBoundedLiteralAndOpenAPI(t *testing.T) {
	preserveDiscoveryGlobals(t)
	router, backend := transportFilterRouter(t)
	literal := strings.Repeat("a", 16384)
	raw := url.Values{"filter": {"s = '" + literal + "'"}}.Encode()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/features/collections/available/items?"+raw, nil))
	if w.Code != 200 || backend.query.Filter.Root().Literal.Text() != literal {
		t.Fatal(w.Code)
	}
	oversized := url.Values{"filter": {"s = '" + literal + "b'"}}.Encode()
	bad := httptest.NewRecorder()
	router.ServeHTTP(bad, httptest.NewRequest("GET", "/features/collections/available/items?"+oversized, nil))
	if bad.Code != 400 || backend.calls.Load() != 1 {
		t.Fatal("limit ignored", bad.Code)
	}
	// Annex B text grammar rejects a literal NUL even though the neutral API can represent it.
	nul := httptest.NewRecorder()
	router.ServeHTTP(nul, httptest.NewRequest("GET", "/features/collections/available/items?"+url.Values{"filter": {"s = '\x00'"}}.Encode(), nil))
	if nul.Code != 400 || backend.calls.Load() != 1 {
		t.Fatal("invalid text literal reached provider", nul.Code)
	}
	api := httptest.NewRecorder()
	router.ServeHTTP(api, httptest.NewRequest("GET", "/features/api", nil))
	var doc map[string]any
	if err := json.Unmarshal(api.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	paths := doc["paths"].(map[string]any)
	schema := paths["/collections/available/queryables"].(map[string]any)
	for _, method := range []string{"get", "head"} {
		op := schema[method].(map[string]any)
		response := op["responses"].(map[string]any)["200"].(map[string]any)
		if method == "head" {
			if response["content"] != nil {
				t.Fatal("HEAD schema declares body")
			}
			continue
		}
		content := response["content"].(map[string]any)
		if content["application/schema+json"] == nil {
			t.Fatal("schema media missing")
		}
	}
	items := paths["/collections/available/items"].(map[string]any)["get"].(map[string]any)
	found := map[string]bool{}
	for _, parameter := range items["parameters"].([]any) {
		found[parameter.(map[string]any)["name"].(string)] = true
	}
	if !found["filter"] || !found["filter-lang"] || found["filter-crs"] {
		t.Fatal(found)
	}
	conformance := httptest.NewRecorder()
	router.ServeHTTP(conformance, httptest.NewRequest("GET", "/features/conformance", nil))
	var declaration struct {
		ConformsTo []string `json:"conformsTo"`
	}
	if err := json.Unmarshal(conformance.Body.Bytes(), &declaration); err != nil {
		t.Fatal(err)
	}
	wantClasses := []string{features.ConformanceCore, features.ConformanceGeoJSON, features.ConformanceHTML, features.ConformanceOpenAPI}
	if len(declaration.ConformsTo) != len(wantClasses) {
		t.Fatal("unreviewed optional conformance class", declaration)
	}
	for i, class := range wantClasses {
		if declaration.ConformsTo[i] != class {
			t.Fatal("conformance class changed", declaration)
		}
	}
}
