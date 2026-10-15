package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
)

type protocolLayer struct{}

func (protocolLayer) Name() string            { return "source" }
func (protocolLayer) SRID() uint64            { return 4326 }
func (protocolLayer) GeomType() geom.Geometry { return geom.Point{} }
func (protocolLayer) TemporalMapping() (provider.TemporalMapping, error) {
	return provider.TemporalMapping{}, nil
}

type protocolTemporalLayer struct{ protocolLayer }

func (protocolTemporalLayer) TemporalMapping() (provider.TemporalMapping, error) {
	return provider.TemporalMapping{InstantField: "at"}, nil
}
func (protocolLayer) FeatureQuerySupported() error { return nil }
func (protocolLayer) SpatialMetadata() (provider.SpatialMetadata, error) {
	return provider.SpatialMetadata{Dimension: provider.DimensionXY}, nil
}

// This adapter records translation and supplies explicit paging evidence. It is
// not a spatial or temporal oracle; those checks use the real GPKG fixture.
type protocolQuerier struct {
	mu      sync.Mutex
	queries []provider.FeatureQuery
	err     error
	unknown bool
}

func (p *protocolQuerier) QueryFeatures(ctx context.Context, _ string, q provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	p.mu.Lock()
	p.queries = append(p.queries, q)
	p.mu.Unlock()
	if p.err != nil {
		return provider.FeatureQueryResult{}, p.err
	}
	if err := ctx.Err(); err != nil {
		return provider.FeatureQueryResult{}, err
	}
	ids := []uint64{10, 20, 30}
	if q.IDs != nil {
		ids = nil
		for _, id := range q.IDs {
			if id == 10 {
				ids = append(ids, id)
			}
		}
	}
	count := uint64(len(ids))
	result := provider.FeatureQueryResult{}
	if !p.unknown {
		result.NumberMatched = &count
	}
	for i, id := range ids {
		if uint64(i) < q.Offset {
			continue
		}
		if result.NumberReturned == uint64(q.Limit) {
			result.HasMore = true
			break
		}
		if err := fn(&provider.Feature{ID: id, SRID: 4326, Geometry: geom.Point{15, 30}, Tags: map[string]any{"name": "public"}}); err != nil {
			return result, err
		}
		result.NumberReturned++
	}
	return result, nil
}
func protocolServer(t *testing.T, q provider.FeatureQuerier, layer provider.LayerInfo, prefix string) *httptest.Server {
	t.Helper()
	oldPrefix, oldHeaders := URIPrefix, Headers
	URIPrefix = prefix
	Headers = map[string]string{"Cache-Control": "public,max-age=100", "X-Protocol": "yes"}
	t.Cleanup(func() { URIPrefix, Headers = oldPrefix, oldHeaders })
	svc, err := features.NewService([]features.CollectionSource{{ID: "public", Layer: layer, Querier: q}})
	if err != nil {
		t.Fatal(err)
	}
	api, err := NewFeatureAPI(svc, FeatureAPIConfig{BasePath: "/features", DefaultLimit: 1, MaxLimit: 2})
	if err != nil {
		t.Fatal(err)
	}
	router, err := NewRouterWithOptions(nil, RouterOptions{Features: api})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv
}

type protocolResponse struct {
	status int
	header http.Header
	body   []byte
}

func protocolRequest(t *testing.T, srv *httptest.Server, method, path string, accept ...string) protocolResponse {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range accept {
		req.Header.Add("Accept", value)
	}
	client := *srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return protocolResponse{resp.StatusCode, resp.Header, body}
}

func TestFeatureProtocolAcceptanceNegotiation(t *testing.T) {
	srv := protocolServer(t, &protocolQuerier{}, protocolLayer{}, "/")
	for _, tc := range []struct {
		path   string
		accept []string
		status int
		media  string
	}{
		{"/features/collections/public/items", nil, 200, "application/geo+json"},
		{"/features/collections/public/items", []string{"application/geo+json"}, 200, "application/geo+json"},
		{"/features/collections/public/items", []string{"*/*"}, 200, "application/geo+json"},
		{"/features/collections/public/items", []string{"application/xml"}, 406, "application/json"},
		{"/features/collections/public/items", []string{"application/json"}, 406, "application/json"},
		{"/features/collections/public/items", []string{",, application/geo+json,,"}, 200, "application/geo+json"},
		{"/features/collections/public/items", []string{"application/geo+json;q=1;note=\"a,b\""}, 200, "application/geo+json"},
		{"/features/collections/public/items", []string{"application/geo+json;q=0, text/html;q=0, */*;q=1"}, 406, "application/json"},
		{"/features/collections/public/items", []string{"text/html", "application/geo+json"}, 200, "application/geo+json"},
		{"/features/collections", []string{"application/json"}, 200, "application/json"},
		{"/features/collections", []string{"application/json;q=0, application/*;q=1"}, 406, "application/json"},
		{"/features/api", []string{"application/vnd.oai.openapi+json;version=3.0"}, 200, "application/vnd.oai.openapi+json;version=3.0"},
		{"/features/api", []string{"application/vnd.oai.openapi+json;version=9"}, 406, "application/json"},
	} {
		r := protocolRequest(t, srv, "GET", tc.path, tc.accept...)
		if r.status != tc.status || r.header.Get("Content-Type") != tc.media || r.header.Get("Cache-Control") != "no-store" {
			t.Errorf("%s Accept%v: %d %v %s", tc.path, tc.accept, r.status, r.header, r.body)
		}
	}
	get := protocolRequest(t, srv, "GET", "/features/collections/public/items", "application/xml")
	head := protocolRequest(t, srv, "HEAD", "/features/collections/public/items", "application/xml")
	if head.status != 406 || len(head.body) != 0 || head.header.Get("Content-Length") != get.header.Get("Content-Length") {
		t.Fatal("HEAD 406 parity", head)
	}
}
func protocolIDs(t *testing.T, r protocolResponse) []uint64 {
	t.Helper()
	var body struct{ Features []struct{ ID uint64 } }
	if err := json.Unmarshal(r.body, &body); err != nil {
		t.Fatal(err)
	}
	ids := []uint64{}
	for _, f := range body.Features {
		ids = append(ids, f.ID)
	}
	return ids
}
func TestFeatureProtocolAcceptancePagingAndHEAD(t *testing.T) {
	q := &protocolQuerier{}
	srv := protocolServer(t, q, protocolTemporalLayer{}, "/stage")
	path := "/stage/features/collections/public/items?limit=1&datetime=2020-01-01T00%3A00%3A00Z&bbox=10,20,20,40"
	get := protocolRequest(t, srv, http.MethodGet, path)
	if get.status != 200 || !reflect.DeepEqual(protocolIDs(t, get), []uint64{10}) {
		t.Fatalf("first page: %d %s", get.status, get.body)
	}
	var page struct {
		NumberReturned uint64
		NumberMatched  *uint64
		Links          []struct{ Href, Rel string }
	}
	if err := json.Unmarshal(get.body, &page); err != nil {
		t.Fatal(err)
	}
	if page.NumberReturned != 1 || page.NumberMatched == nil || *page.NumberMatched != 3 {
		t.Fatal("count evidence", string(get.body))
	}
	for _, link := range page.Links {
		u, err := url.Parse(link.Href)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(u.Path, "/stage/features/") {
			t.Fatal("stage lost", link)
		}
		if link.Rel == "item" {
			if len(u.Query()) != 1 || u.Query().Get("f") != "json" {
				t.Fatal("item link invalid constraints", link)
			}
			continue
		}
		if u.Query().Get("datetime") != "2020-01-01T00:00:00Z" || u.Query().Get("bbox") != "10,20,20,40" {
			t.Fatal("filters lost", link)
		}
		if link.Rel == "next" {
			next := protocolRequest(t, srv, http.MethodGet, u.RequestURI())
			if !reflect.DeepEqual(protocolIDs(t, next), []uint64{20}) {
				t.Fatal("next", string(next.body))
			}
		}
	}
	head := protocolRequest(t, srv, http.MethodHead, path)
	if head.status != get.status || len(head.body) != 0 || head.header.Get("Content-Length") != get.header.Get("Content-Length") {
		t.Fatal("HEAD parity")
	}
	if get.header.Get("Cache-Control") != "no-store" || get.header.Get("Access-Control-Allow-Origin") == "" || get.header.Get("Content-Type") != "application/geo+json" {
		t.Fatal(get.header)
	}
	q.mu.Lock()
	first := q.queries[0]
	q.mu.Unlock()
	if first.Limit != 1 || first.BoundsSRID != 4326 || len(first.Bounds) != 1 || first.Temporal == nil {
		t.Fatal("neutral translation", first)
	}
}
func TestFeatureProtocolAcceptanceLimitsAndInvalid(t *testing.T) {
	q := &protocolQuerier{}
	srv := protocolServer(t, q, protocolLayer{}, "/")
	base := "/features/collections/public/items"
	for _, tc := range []struct {
		raw  string
		want int
	}{{"limit=1", 1}, {"limit=2", 2}} {
		r := protocolRequest(t, srv, "GET", base+"?"+tc.raw)
		if r.status != 200 || len(protocolIDs(t, r)) != tc.want {
			t.Fatalf("limit %s: %d %s", tc.raw, r.status, r.body)
		}
	}
	for _, raw := range []string{"limit=0", "limit=-1", "limit=1.5", "limit=", "limit=1&limit=2", "limit=3", "limit=99999999999999999999999999999999999", "offset=-1", "offset=18446744073709551615&limit=2", "other=x", "bbox=1,2,3", "bbox=NaN,0,1,1", "bbox=0,2,1,1", "bbox=0,0,2,1,1,1", "datetime=../..", "datetime=2020-01-02T00:00:00Z/2020-01-01T00:00:00Z", "datetime=garbage", "datetime=2015-01-01T23:59:60Z"} {
		r := protocolRequest(t, srv, "GET", base+"?"+raw)
		if r.status != 400 {
			t.Errorf("%s: %d %s", raw, r.status, r.body)
		}
	}
	for _, path := range []string{"/features/collections/missing/items", "/features/collections/public/items/999"} {
		r := protocolRequest(t, srv, "GET", path)
		if r.status != 404 {
			t.Errorf("%s: %d", path, r.status)
		}
	}
	r := protocolRequest(t, srv, "GET", "/features/collections/public/items/10")
	if r.status != 200 {
		t.Fatal(string(r.body))
	}
}
func TestFeatureProtocolAcceptanceErrorsAndUnknownCount(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{{"unsupported", provider.ErrUnsupported, 501}, {"invalid", provider.InvalidFeatureQueryError{Field: "bbox", Reason: "secret-source"}, 400}, {"cancel", context.Canceled, 408}, {"failure", errors.New("secret-source credential"), 500}} {
		t.Run(tc.name, func(t *testing.T) {
			srv := protocolServer(t, &protocolQuerier{err: tc.err}, protocolLayer{}, "/")
			r := protocolRequest(t, srv, "GET", "/features/collections/public/items")
			if r.status != tc.status || strings.Contains(string(r.body), "secret-source") || r.header.Get("Cache-Control") != "no-store" {
				t.Fatalf("%d %s", r.status, r.body)
			}
		})
	}
	srv := protocolServer(t, &protocolQuerier{unknown: true}, protocolLayer{}, "/")
	r := protocolRequest(t, srv, "GET", "/features/collections/public/items")
	var body map[string]json.RawMessage
	if err := json.Unmarshal(r.body, &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["numberMatched"]; ok {
		t.Fatal("unknown count published")
	}
}

func TestFeatureProtocolAcceptanceNamespaceProxyAndMethods(t *testing.T) {
	oldRoot := URLRoot
	t.Cleanup(func() { URLRoot = oldRoot })
	URLRoot = func(*http.Request) *url.URL {
		return &url.URL{Scheme: "https", Host: "public.example", Path: "/gateway"}
	}
	srv := protocolServer(t, &protocolQuerier{}, protocolLayer{}, "/stage")
	r := protocolRequest(t, srv, "GET", "/stage/features/collections/public/items")
	var body struct{ Links []struct{ Href string } }
	if err := json.Unmarshal(r.body, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Links) < 2 {
		t.Fatal("paging links absent")
	}
	for _, link := range body.Links {
		if !strings.HasPrefix(link.Href, "https://public.example/gateway/stage/features/") {
			t.Fatal("public proxy/stage lost", link)
		}
	}
	for _, path := range []string{"/features/collections/public/items", "/stage/features/unknown"} {
		r := protocolRequest(t, srv, "GET", path)
		if r.status != 404 {
			t.Errorf("namespace %s: %d", path, r.status)
		}
	}
	for _, method := range []string{"POST", "OPTIONS"} {
		r := protocolRequest(t, srv, method, "/stage/features/collections/public/items")
		if r.header.Get("Cache-Control") != "no-store" {
			t.Fatal("method cache policy", r.header)
		}
		if method == "POST" && r.status != 405 {
			t.Fatal("method status", r.status)
		}
	}
	redirect := protocolRequest(t, srv, "GET", "/stage/features/collections/public/items/")
	if redirect.status < 300 || redirect.status > 399 || redirect.header.Get("Cache-Control") != "no-store" {
		t.Fatal("canonical redirect", redirect.status, redirect.header)
	}
	router, err := NewRouterWithOptions(nil, RouterOptions{})
	if err != nil {
		t.Fatal(err)
	}
	disabled := httptest.NewServer(router)
	t.Cleanup(disabled.Close)
	for _, path := range []string{"/stage/features", "/stage/features/collections/public/items"} {
		r := protocolRequest(t, disabled, "GET", path)
		if r.status != 404 {
			t.Fatal("disabled API exposed", path, r.status)
		}
	}
}

func TestFeatureProtocolAcceptanceConcurrentRequests(t *testing.T) {
	srv := protocolServer(t, &protocolQuerier{}, protocolLayer{}, "/")
	var wg sync.WaitGroup
	issues := make(chan string, 8)
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := srv.Client().Get(srv.URL + "/features/collections/public/items?limit=1&offset=1")
			if err != nil {
				issues <- err.Error()
				return
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					issues <- err.Error()
				}
			}()
			var body struct{ Features []struct{ ID uint64 } }
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				issues <- err.Error()
				return
			}
			if resp.StatusCode != 200 || len(body.Features) != 1 || body.Features[0].ID != 20 {
				issues <- "concurrent response differs"
			}
		}()
	}
	// Consume concurrently: even multiple diagnostics from a worker cannot block
	// it before Wait completes.
	go func() { wg.Wait(); close(issues) }()
	for issue := range issues {
		t.Error(issue)
	}
}
