package server

import (
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFeatureAPIDefinitionRepresentationsAndHEAD(t *testing.T) {
	preserveDiscoveryGlobals(t)
	api := discoveryAPI(t)
	router, err := NewRouterWithOptions(nil, RouterOptions{Features: api})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ query, accept, media string }{
		{"", "", "application/vnd.oai.openapi+json;version=3.0"},
		{"?f=json", "text/html", "application/vnd.oai.openapi+json;version=3.0"},
		{"?f=html", "application/json", "text/html; charset=utf-8"},
		{"", "text/html", "text/html; charset=utf-8"},
	} {
		get := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/features/api"+tc.query, nil)
		if tc.accept != "" {
			request.Header.Set("Accept", tc.accept)
		}
		router.ServeHTTP(get, request)
		if get.Code != 200 || get.Header().Get("Content-Type") != tc.media || get.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(get.Code, get.Header())
		}
		head := httptest.NewRecorder()
		request = httptest.NewRequest(http.MethodHead, "/features/api"+tc.query, nil)
		if tc.accept != "" {
			request.Header.Set("Accept", tc.accept)
		}
		router.ServeHTTP(head, request)
		if head.Code != get.Code || head.Body.Len() != 0 || head.Header().Get("Content-Length") != get.Header().Get("Content-Length") || head.Header().Get("Content-Type") != tc.media {
			t.Fatal("HEAD mismatch", head.Code, head.Header())
		}
		if strings.HasPrefix(tc.media, "text/html") {
			for _, marker := range []string{"3.0.3", "/collections/alpha/items", "getItems_616c706861", "application/geo+json"} {
				if !strings.Contains(html.UnescapeString(get.Body.String()), marker) {
					t.Error("incomplete API HTML", marker)
				}
			}
		} else {
			var document map[string]any
			if err := json.Unmarshal(get.Body.Bytes(), &document); err != nil {
				t.Fatal(err)
			}
			if document["openapi"] != "3.0.3" || document["components"] == nil || document["x-tegola-max-response-bytes"] != float64(16<<20) {
				t.Fatal("missing API metadata", document)
			}
			paths := document["paths"].(map[string]any)
			if paths["/collections/alpha/items"] == nil || paths["/collections/alpha/queryables"] != nil {
				t.Fatal("wrong installed capabilities")
			}
		}
	}
}

func TestFeatureAPIDefinitionErrorsRemainJSON(t *testing.T) {
	preserveDiscoveryGlobals(t)
	router, err := NewRouterWithOptions(nil, RouterOptions{Features: discoveryAPI(t)})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/features/api?f=unknown", "/features/api?f=html&extra=1", "/features/api?f=html&f=json"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Accept", "text/html")
		router.ServeHTTP(response, request)
		if response.Code != 400 || response.Header().Get("Content-Type") != "application/json" {
			t.Fatal(path, response.Code, response.Header())
		}
		var value struct{ Code, Description string }
		if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil || value.Code == "" || value.Description == "" {
			t.Fatal("invalid error schema", err)
		}
	}
}
