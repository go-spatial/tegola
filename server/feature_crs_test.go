package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
)

type transportCRSLayer struct{ discoveryLayer }

func (transportCRSLayer) FeatureCRSDefinition() (provider.FeatureCRSDefinition, error) {
	descriptor, err := features.ResolveCRS(features.CRS84)
	return provider.FeatureCRSDefinition{HorizontalSRID: 4326, Definition: descriptor.Definition().Definition, CanonicalAuthority: "EPSG", CanonicalCode: "4326", Spatial: provider.SpatialMetadata{Dimension: provider.DimensionXY}}, err
}
func transportCRSRouter(t *testing.T) (*Router, *transportFilterQuerier) {
	t.Helper()
	backend := &transportFilterQuerier{}
	service, err := features.NewService([]features.CollectionSource{{ID: "public", Layer: transportCRSLayer{}, Querier: backend}, {ID: "core", Layer: discoveryLayer{}, Querier: backend}})
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
func TestFeatureCRSAxesHeadersAndOriginalBounds(t *testing.T) {
	preserveDiscoveryGlobals(t)
	router, backend := transportCRSRouter(t)
	uri := "http://www.opengis.net/def/crs/EPSG/0/4326"
	parameters := url.Values{"crs": {uri}, "bbox-crs": {uri}, "bbox": {"-1,14,1,16"}}
	path := "/features/collections/public/items?" + parameters.Encode()
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(method, path, nil))
		if response.Code != 200 || response.Header().Get("Content-Crs") != "<"+uri+">" || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s %d %v", method, response.Code, response.Header())
		}
		if !reflect.DeepEqual(backend.query.Bounds, []geom.Extent{{14, -1, 16, 1}}) || backend.query.BoundsSRID != 4326 || backend.query.BoundsCRSDefinition == "" {
			t.Fatalf("query frame changed %+v", backend.query)
		}
		if method == http.MethodHead {
			if response.Body.Len() != 0 {
				t.Fatal("HEAD body")
			}
			continue
		}
		var page struct {
			Features []struct {
				Geometry struct{ Coordinates [2]float64 }
			}
			Links []featureLink
		}
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if page.Features[0].Geometry.Coordinates != [2]float64{0, 15} {
			t.Fatalf("axes %s", response.Body.String())
		}
		for _, link := range page.Links {
			parsed, err := url.Parse(link.Href)
			if err != nil {
				t.Fatal(err)
			}
			if link.Rel == "item" {
				if parsed.Query().Get("crs") != uri || parsed.Query().Get("bbox-crs") != "" {
					t.Fatal("item CRS changed")
				}
				continue
			}
			if parsed.Query().Get("crs") != uri || parsed.Query().Get("bbox-crs") != uri {
				t.Fatal("paging lost CRS")
			}
		}
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/features/collections/public/items/7?crs="+url.QueryEscape(uri), nil))
	if response.Code != 200 || response.Header().Get("Content-Crs") != "<"+uri+">" {
		t.Fatalf("item %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "crs=") {
		t.Fatal("item self lost CRS")
	}
}
func TestFeatureCRSRejectsBeforeIOAndCoreDefaults(t *testing.T) {
	preserveDiscoveryGlobals(t)
	router, backend := transportCRSRouter(t)
	for _, suffix := range []string{"?crs=", "?crs=bad", "?crs=bad&crs=bad", "?bbox-crs=bad", "?bbox=0,0,1,1&bbox-crs=bad", "?bbox=0,0,1,1&bbox-crs=" + url.QueryEscape(provider.CRS84h), "?bbox=2,0,1,1&bbox-crs=" + url.QueryEscape("http://www.opengis.net/def/crs/EPSG/0/3857")} {
		before := backend.calls.Load()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/features/collections/public/items"+suffix, nil))
		if response.Code != 400 || backend.calls.Load() != before {
			t.Fatalf("%s status%d calls%d", suffix, response.Code, backend.calls.Load()-before)
		}
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/features/collections/core/items", nil))
	if response.Code != 200 || response.Header().Get("Content-Crs") != "<"+features.CRS84+">" {
		t.Fatalf("Core default %d %v", response.Code, response.Header())
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/features/collections/core/items?crs="+url.QueryEscape(features.CRS84), nil))
	if response.Code != 400 {
		t.Fatalf("unavailable explicit %d", response.Code)
	}
}
func TestFeatureCRSCollectionAndOpenAPI(t *testing.T) {
	preserveDiscoveryGlobals(t)
	router, _ := transportCRSRouter(t)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/features/collections/public", nil))
	var collection featureCollectionDescription
	if err := json.Unmarshal(response.Body.Bytes(), &collection); err != nil {
		t.Fatal(err)
	}
	if len(collection.CRS) == 0 || collection.CRS[0] != features.CRS84 || collection.StorageCRS != features.CRS84 {
		t.Fatalf("metadata %s", response.Body.String())
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/features/api", nil))
	for _, required := range []string{"bbox-crs", "Content-Crs", "\"crs\""} {
		if !strings.Contains(response.Body.String(), required) {
			t.Fatalf("missing %s", required)
		}
	}
}

func TestFeatureCRSHeaderRemovedOnEncodingFailure(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		response := httptest.NewRecorder()
		response.Header().Set("Content-Crs", "<"+features.CRS84+">")
		(&FeatureAPI{}).writeJSON(response, httptest.NewRequest(method, "/", nil), 200, "application/geo+json", make(chan int))
		if response.Code != 500 || response.Header().Get("Content-Crs") != "" {
			t.Fatalf("encoding failure %d %v", response.Code, response.Header())
		}
		if method == http.MethodHead && response.Body.Len() != 0 {
			t.Fatal("HEAD error body")
		}
	}
}

func TestFeatureCRSOpenAPICoreParameterDefinitions(t *testing.T) {
	preserveDiscoveryGlobals(t)
	router, _ := transportCRSRouter(t)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/features/api", nil))
	var document struct {
		Paths map[string]map[string]struct{ Parameters []map[string]any }
	}
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"get", "head"} {
		byName := map[string]map[string]any{}
		for _, parameter := range document.Paths["/collections/public/items"][method].Parameters {
			byName[parameter["name"].(string)] = parameter
		}
		for _, name := range []string{"limit", "bbox", "datetime"} {
			parameter := byName[name]
			if parameter["style"] != "form" || parameter["explode"] != false || parameter["required"] != false {
				t.Fatalf("%s %s serialization %v", method, name, parameter)
			}
		}
		bbox := byName["bbox"]["schema"].(map[string]any)
		if bbox["type"] != "array" || bbox["minItems"] != float64(4) || bbox["maxItems"] != float64(6) || bbox["items"].(map[string]any)["type"] != "number" {
			t.Fatalf("bbox root schema %v", bbox)
		}
		choices := bbox["oneOf"].([]any)
		if len(choices) != 2 {
			t.Fatalf("bbox choices %v", choices)
		}
		for i, length := range []float64{4, 6} {
			choice := choices[i].(map[string]any)
			if choice["minItems"] != length || choice["maxItems"] != length {
				t.Fatalf("bbox arity %v", choice)
			}
		}
		limit := byName["limit"]["schema"].(map[string]any)
		if limit["maximum"] != float64(10) || limit["minimum"] != float64(1) || limit["default"] != float64(1) {
			t.Fatalf("publication limits %v", limit)
		}
	}
}
