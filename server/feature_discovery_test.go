package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
	"github.com/dimfeld/httptreemux"
)

type discoveryLayer struct{}

func (discoveryLayer) Name() string            { return "source" }
func (discoveryLayer) SRID() uint64            { return 4326 }
func (discoveryLayer) GeomType() geom.Geometry { return geom.Point{} }
func (discoveryLayer) TemporalMapping() (provider.TemporalMapping, error) {
	return provider.TemporalMapping{}, nil
}
func (discoveryLayer) FeatureQuerySupported() error { return nil }
func (discoveryLayer) SpatialMetadata() (provider.SpatialMetadata, error) {
	return provider.SpatialMetadata{Dimension: provider.DimensionXY}, nil
}

type discoveryQuerier struct{}

func (discoveryQuerier) QueryFeatures(context.Context, string, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	return provider.FeatureQueryResult{}, errors.New("discovery must not query data")
}

func discoveryService(t *testing.T, ids ...string) *features.Service {
	t.Helper()
	sources := make([]features.CollectionSource, 0, len(ids))
	for _, id := range ids {
		sources = append(sources, features.CollectionSource{ID: id, Layer: discoveryLayer{}, Querier: discoveryQuerier{}, Title: "Title " + id, Description: "Description " + id})
	}
	service, err := features.NewService(sources)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func discoveryAPI(t *testing.T) *FeatureAPI {
	t.Helper()
	api, err := NewFeatureAPI(discoveryService(t, "zeta", "alpha"), FeatureAPIConfig{BasePath: "/features", DefaultLimit: 100, MaxLimit: 10000, Title: "Published data", Description: "Public collections"})
	if err != nil {
		t.Fatal(err)
	}
	return api
}

func preserveDiscoveryGlobals(t *testing.T) {
	t.Helper()
	prefix, headers, host, proxy, root := URIPrefix, Headers, HostName, ProxyProtocol, URLRoot
	t.Cleanup(func() { URIPrefix, Headers, HostName, ProxyProtocol, URLRoot = prefix, headers, host, proxy, root })
	URIPrefix = "/"
	Headers = map[string]string{}
	HostName = nil
	ProxyProtocol = ""
}

func TestFeatureDiscoveryRuntimeValidation(t *testing.T) {
	preserveDiscoveryGlobals(t)
	service := discoveryService(t, "alpha")
	for name, cfg := range map[string]FeatureAPIConfig{
		"zero":            {},
		"root":            {BasePath: "/", DefaultLimit: 1, MaxLimit: 1},
		"reserved":        {BasePath: "/maps/data", DefaultLimit: 1, MaxLimit: 1},
		"dynamic":         {BasePath: "/:features", DefaultLimit: 1, MaxLimit: 1},
		"inverted limits": {BasePath: "/features", DefaultLimit: 2, MaxLimit: 1},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewFeatureAPI(service, cfg); err == nil {
				t.Fatal("invalid API accepted")
			}
		})
	}
	cfg := FeatureAPIConfig{BasePath: "/features", DefaultLimit: 1, MaxLimit: 1}
	if _, err := NewFeatureAPI(nil, cfg); err == nil {
		t.Fatal("nil service accepted")
	}
	if _, err := NewFeatureAPI(discoveryService(t, "bad/id"), cfg); err == nil {
		t.Fatal("unsafe direct catalog ID accepted")
	}
	if _, err := NewFeatureAPI(discoveryService(t), cfg); err == nil {
		t.Fatal("empty publication accepted")
	}
	if router, err := NewRouterWithOptions(nil, RouterOptions{Features: &FeatureAPI{}}); err == nil || router != nil {
		t.Fatal("uninitialized runtime accepted")
	}
	if srv, err := StartWithOptions(nil, ":0", RouterOptions{Features: &FeatureAPI{}}); err == nil || srv != nil {
		t.Fatal("invalid runtime reached listener")
	}
}

func TestFeatureDiscoveryResourcesAndHEAD(t *testing.T) {
	preserveDiscoveryGlobals(t)
	Headers = map[string]string{"Cache-Control": "public,max-age=3600", "X-Configured": "present"}
	api := discoveryAPI(t)
	router, err := NewRouterWithOptions(nil, RouterOptions{Features: api})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/features", "/features/api", "/features/conformance", "/features/collections", "/features/collections/alpha", "/features/collections/missing", "/features/unknown"} {
		t.Run(path, func(t *testing.T) {
			get := httptest.NewRecorder()
			router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, path, nil))
			head := httptest.NewRecorder()
			router.ServeHTTP(head, httptest.NewRequest(http.MethodHead, path, nil))
			if head.Code != get.Code || head.Body.Len() != 0 || !reflect.DeepEqual(head.Header(), get.Header()) {
				t.Fatalf("HEAD differs: %d/%d %v/%v %s", head.Code, get.Code, head.Header(), get.Header(), head.Body.String())
			}
			if get.Header().Get("Cache-Control") != "no-store" || get.Header().Get("X-Configured") != "present" || get.Header().Get("Access-Control-Allow-Origin") == "" {
				t.Fatal(get.Header())
			}
			if get.Header().Get("Content-Length") != strconv.Itoa(get.Body.Len()) {
				t.Fatal("incorrect content length")
			}
			if !json.Valid(get.Body.Bytes()) {
				t.Fatal(get.Body.String())
			}
		})
	}
	for _, method := range []string{http.MethodOptions, http.MethodPost} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(method, "/features/collections", nil))
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("method response cache policy", response.Header())
		}
		if method == http.MethodPost && response.Code != http.StatusMethodNotAllowed {
			t.Fatal(response.Code)
		}
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/features?unknown=1", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatal("unknown discovery query accepted")
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/features/conformance", nil))
	var declaration struct {
		ConformsTo []string      `json:"conformsTo"`
		Links      []featureLink `json:"links"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &declaration); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(declaration.ConformsTo, []string{features.ConformanceCore, features.ConformanceGeoJSON, features.ConformanceHTML, features.ConformanceOpenAPI}) || len(declaration.Links) != 2 {
		t.Fatal("unexpected admitted conformance classes", response.Body.String())
	}
}

func TestFeatureDiscoveryProxyLinksAndCatalog(t *testing.T) {
	preserveDiscoveryGlobals(t)
	URIPrefix = "/proxy"
	HostName = &url.URL{Host: "public.example"}
	ProxyProtocol = "https"
	api := discoveryAPI(t)
	router, err := NewRouterWithOptions(nil, RouterOptions{Features: api})
	if err != nil {
		t.Fatal(err)
	}
	// Changing the global afterward cannot change the mounted feature prefix.
	URIPrefix = "/changed"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proxy/features", nil))
	var landing struct{ Links []featureLink }
	if err := json.Unmarshal(response.Body.Bytes(), &landing); err != nil {
		t.Fatal(err)
	}
	wanted := map[string]string{"self": "", "alternate": "", "service-doc": "/api", "service-desc": "/api", "conformance": "/conformance", "data": "/collections"}
	if len(landing.Links) != len(wanted) {
		t.Fatal(landing)
	}
	for _, link := range landing.Links {
		if link.Href != "https://public.example/proxy/features"+wanted[link.Rel]+"?f="+map[bool]string{true: "html", false: "json"}[link.Rel == "alternate" || link.Rel == "service-doc"] {
			t.Fatal(link)
		}
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proxy/features/collections", nil))
	var catalog struct {
		Collections []featureCollectionDescription
	}
	if err := json.Unmarshal(response.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Collections) != 2 || catalog.Collections[0].ID != "alpha" || catalog.Collections[0].Title != "Title alpha" || catalog.Collections[1].ID != "zeta" {
		t.Fatal(catalog)
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proxy/features/api", nil))
	var definition struct {
		OpenAPI string
		Servers []struct{ URL string }
		Paths   map[string]map[string]json.RawMessage
	}
	if err := json.Unmarshal(response.Body.Bytes(), &definition); err != nil {
		t.Fatal(err)
	}
	if definition.OpenAPI != "3.0.3" || len(definition.Paths) != 10 || definition.Servers[0].URL != "https://public.example/proxy/features" {
		t.Fatal(definition)
	}
	if _, exists := definition.Paths["/collections/alpha/items"]; !exists {
		t.Fatal("installed items endpoint absent")
	}
	for _, operations := range definition.Paths {
		if _, exists := operations["get"]; !exists {
			t.Fatal("GET absent")
		}
		if _, exists := operations["head"]; !exists {
			t.Fatal("HEAD absent")
		}
	}
}

func TestFeatureDiscoveryDisabledLegacyAndConcurrency(t *testing.T) {
	preserveDiscoveryGlobals(t)
	legacy := NewRouter(nil)
	disabled, err := NewRouterWithOptions(nil, RouterOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/capabilities", "/maps/missing/style.json", "/features"} {
		old, new := httptest.NewRecorder(), httptest.NewRecorder()
		legacy.ServeHTTP(old, httptest.NewRequest(http.MethodGet, path, nil))
		disabled.ServeHTTP(new, httptest.NewRequest(http.MethodGet, path, nil))
		if old.Code != new.Code || old.Body.String() != new.Body.String() || !reflect.DeepEqual(old.Header(), new.Header()) {
			t.Fatalf("disabled behavior changed for %s", path)
		}
	}
	router, err := NewRouterWithOptions(nil, RouterOptions{Features: discoveryAPI(t)})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/features/collections", nil))
			if response.Code != http.StatusOK {
				t.Error(response.Code)
			}
		})
	}
	wg.Wait()
}

func TestFeatureDiscoveryStaticURIPrefixValidation(t *testing.T) {
	preserveDiscoveryGlobals(t)
	api := discoveryAPI(t)
	for _, prefix := range []string{"", "relative", "/:tenant", "/root/*path", "/%61", "/root/../data", "/root/./data", "/root//data", "/root//", "/root?x=1"} {
		URIPrefix = prefix
		if router, err := NewRouterWithOptions(nil, RouterOptions{Features: api}); err == nil || router != nil {
			t.Fatalf("unsafe prefix accepted %q", prefix)
		}
		if listener, err := StartWithOptions(nil, ":0", RouterOptions{Features: api}); err == nil || listener != nil {
			t.Fatalf("unsafe prefix reached listener %q", prefix)
		}
	}
	for _, prefix := range []string{"/", "/proxy", "/proxy/", "/root/proxy"} {
		URIPrefix = prefix
		router, err := NewRouterWithOptions(nil, RouterOptions{Features: api})
		if err != nil {
			t.Fatal(err)
		}
		path := strings.TrimSuffix(prefix, "/") + "/features"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("valid prefix failed %q: %d", prefix, response.Code)
		}
		var landing struct{ Links []featureLink }
		if err := json.Unmarshal(response.Body.Bytes(), &landing); err != nil {
			t.Fatal(err)
		}
		if landing.Links[0].Href != "http://example.com"+path+"?f=json" {
			t.Fatalf("prefix duplicated or normalized: %s", landing.Links[0].Href)
		}
	}
	// Feature-only validation must not change the legacy router's accepted syntax.
	URIPrefix = "/:tenant"
	if _, err := NewRouterWithOptions(nil, RouterOptions{}); err != nil {
		t.Fatal("legacy prefix validation changed", err)
	}
}

func TestFeatureDiscoveryCanonicalRedirectCachePolicy(t *testing.T) {
	preserveDiscoveryGlobals(t)
	Headers = map[string]string{"Cache-Control": "public,max-age=3600"}
	router, err := NewRouterWithOptions(nil, RouterOptions{Features: discoveryAPI(t)})
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(method, "/features/collections/", nil))
		if response.Code != http.StatusMovedPermanently || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("feature redirect unguarded: %d %v", response.Code, response.Header())
		}
		if method == http.MethodHead && response.Body.Len() != 0 {
			t.Fatal("HEAD redirect body")
		}
	}
	// Use a mux without the viewer wildcard to exercise TreeMux clean-path redirects.
	preserve := NewRouter(nil)
	noViewerMux := httptreemux.New()
	noViewerMux.UsingContext().Handler(http.MethodGet, "/features/collections", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	guarded := &Router{TreeMux: noViewerMux, featureBasePath: "/features"}
	response := httptest.NewRecorder()
	guarded.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/other/../features/collections", nil))
	if response.Code != http.StatusMovedPermanently || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cleaned feature redirect unguarded: %d %v", response.Code, response.Header())
	}
	// Legacy redirects must retain the previous policy even when features are enabled.
	old, new := httptest.NewRecorder(), httptest.NewRecorder()
	preserve.ServeHTTP(old, httptest.NewRequest(http.MethodGet, "/capabilities/", nil))
	router.ServeHTTP(new, httptest.NewRequest(http.MethodGet, "/capabilities/", nil))
	if old.Code != new.Code || !reflect.DeepEqual(old.Header(), new.Header()) || old.Body.String() != new.Body.String() {
		t.Fatal("legacy redirect changed")
	}
}

func TestFeatureDiscoveryURLRootStagePath(t *testing.T) {
	preserveDiscoveryGlobals(t)
	URIPrefix = "/proxy"
	api := discoveryAPI(t)
	router, err := NewRouterWithOptions(nil, RouterOptions{Features: api})
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"stage", "/stage", "stage/", "/stage/", "/outer/stage"} {
		root := &url.URL{Scheme: "https", Host: "gateway.example", Path: stage}
		before := *root
		URLRoot = func(*http.Request) *url.URL { return root }
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proxy/features", nil))
		var landing struct{ Links []featureLink }
		if err := json.Unmarshal(response.Body.Bytes(), &landing); err != nil {
			t.Fatal(err)
		}
		expected := "https://gateway.example/" + strings.Trim(stage, "/") + "/proxy/features"
		if landing.Links[0].Href != expected+"?f=json" {
			t.Fatalf("stage omitted/duplicated: %s want %s", landing.Links[0].Href, expected)
		}
		if !reflect.DeepEqual(*root, before) {
			t.Fatal("URLRoot result mutated")
		}
		response = httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proxy/features/api", nil))
		var definition struct{ Servers []struct{ URL string } }
		if err := json.Unmarshal(response.Body.Bytes(), &definition); err != nil {
			t.Fatal(err)
		}
		if definition.Servers[0].URL != expected {
			t.Fatalf("API server stage path lost: %+v", definition)
		}
	}
}
