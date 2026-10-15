package server

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/config"
)

func TestWFSCapabilitiesMountedOperationURLs(t *testing.T) {
	for _, version := range []string{"1.1.0", "2.0.0", "2.0.2"} {
		t.Run(version, func(t *testing.T) {
			preserveDiscoveryGlobals(t)
			URIPrefix = "/v1"
			HostName = &url.URL{Host: "public.example"}
			ProxyProtocol = "https"
			handler := &WFSHandler{
				Service: discoveryService(t, "sites"),
				Config:  config.WFSConfig{Enabled: true, BasePath: "/wfs", Versions: []string{version}}.Resolved(),
			}
			router, err := NewRouterWithOptions(nil, RouterOptions{WFS: handler})
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet,
				"/v1/wfs?service=WFS&request=GetCapabilities&version="+version, nil)
			request.Host = "internal.example"
			router.ServeHTTP(rec, request)
			if rec.Code != http.StatusOK {
				t.Fatalf("capabilities status %d: %s", rec.Code, rec.Body.String())
			}
			decoder := xml.NewDecoder(strings.NewReader(rec.Body.String()))
			links := 0
			for {
				token, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				start, ok := token.(xml.StartElement)
				if !ok || (start.Name.Local != "Get" && start.Name.Local != "Post") {
					continue
				}
				for _, attr := range start.Attr {
					if attr.Name.Local == "href" {
						links++
						if attr.Value != "https://public.example/v1/wfs" {
							t.Errorf("operation URL = %q", attr.Value)
						}
					}
				}
			}
			if links == 0 {
				t.Fatal("no advertised operation links")
			}
		})
	}
}

func TestWFSBaseURLUsesURLRootOverride(t *testing.T) {
	preserveDiscoveryGlobals(t)
	URIPrefix = "/v1"
	root := &url.URL{Scheme: "https", Host: "gateway.example", Path: "/stage"}
	URLRoot = func(*http.Request) *url.URL { return root }
	got := wfsBaseURL(httptest.NewRequest(http.MethodGet, "/v1/wfs", nil), config.WFSConfig{})
	if got != "https://gateway.example/stage/v1/wfs" {
		t.Fatalf("operation URL = %q", got)
	}
	if root.Path != "/stage" {
		t.Fatal("mutated URLRoot result")
	}
}
