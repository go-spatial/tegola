package server

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/ogc/features"
)

func TestFeatureConformanceDeclarationRepresentations(t *testing.T) {
	preserveDiscoveryGlobals(t)
	service, err := features.NewService([]features.CollectionSource{{ID: "public", Layer: discoveryLayer{}, Querier: &itemsQuerier{}}})
	if err != nil {
		t.Fatal(err)
	}
	api, err := NewFeatureAPI(service, FeatureAPIConfig{BasePath: "/features", DefaultLimit: 1, MaxLimit: 2})
	if err != nil {
		t.Fatal(err)
	}
	router, err := NewRouterWithOptions(nil, RouterOptions{Features: api})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{features.ConformanceCore, features.ConformanceGeoJSON, features.ConformanceHTML, features.ConformanceOpenAPI}
	for _, format := range []string{"json", "html"} {
		get := httptest.NewRecorder()
		path := "/features/conformance?f=" + format
		router.ServeHTTP(get, httptest.NewRequest("GET", path, nil))
		if get.Code != 200 || get.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(get.Code, get.Header())
		}
		var declaration struct {
			ConformsTo []string      `json:"conformsTo"`
			Links      []featureLink `json:"links"`
		}
		if format == "json" {
			if err := json.Unmarshal(get.Body.Bytes(), &declaration); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(declaration.ConformsTo, want) || len(declaration.Links) != 2 || declaration.Links[1].Type != "text/html" {
				t.Fatal(declaration)
			}
		} else {
			for _, class := range want {
				if !strings.Contains(get.Body.String(), class) {
					t.Fatal("HTML omitted class", class)
				}
			}
			if strings.Count(get.Body.String(), "<a ") != 2 {
				t.Fatal("declaration links omitted")
			}
		}
		head := httptest.NewRecorder()
		router.ServeHTTP(head, httptest.NewRequest("HEAD", path, nil))
		if head.Code != get.Code || head.Header().Get("Content-Type") != get.Header().Get("Content-Type") || head.Header().Get("Content-Length") != get.Header().Get("Content-Length") || head.Body.Len() != 0 {
			t.Fatal("conformance HEAD parity")
		}
	}
}
