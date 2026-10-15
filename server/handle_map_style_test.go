package server_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"testing"

	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/mapbox/style"
	"github.com/alexeydott/tegola/server"
	"github.com/go-test/deep"
)

func TestHandleMapStyle(t *testing.T) {
	type tcase struct {
		handler        http.Handler
		uriPrefix      string
		uri            string
		uriPattern     string
		serverHostName string
		expected       style.Root
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			var err error

			// config params this test relies on
			server.HostName = nil
			if tc.serverHostName != "" {
				server.HostName = &url.URL{
					Host: tc.serverHostName,
				}
			}

			if tc.uriPrefix != "" {
				server.URIPrefix = tc.uriPrefix
			} else {
				server.URIPrefix = "/"
			}

			resp, _, err := doRequest(t, nil, http.MethodGet, tc.uri, nil)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if resp.Code != http.StatusOK {
				t.Fatalf("handler returned wrong status code: got (%d) expected (%d)", resp.Code, http.StatusOK)
			}

			// read the response body
			var output style.Root
			if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
				t.Fatalf("unable to unmarshal JSON response body: %s", err)
			}

			if diff := deep.Equal(output, tc.expected); diff != nil {
				t.Fatalf("output does not match expected. diff %s", diff)
			}
		}
	}

	tests := map[string]tcase{
		"default": {
			handler:        server.HandleMapStyle{},
			uri:            path.Join("/maps", testMapName, "style.json"),
			uriPattern:     "/maps/:map_name/style.json",
			serverHostName: serverHostName,
			expected: style.Root{
				Name:    testMapName,
				Version: style.Version,
				Center:  [2]float64{testMapCenter[0], testMapCenter[1]},
				Zoom:    testMapCenter[2],
				Sources: map[string]style.Source{
					testMapName: {
						Type: style.SourceTypeVector,
						URL: (&url.URL{
							Scheme: "http",
							Host:   serverHostName,
							Path:   path.Join(server.URIPrefix, "capabilities", testMapName+".json"),
						}).String(),
					},
				},
				Layers: []style.Layer{
					{
						ID:          testLayer1.MVTName(),
						Source:      testMapName,
						SourceLayer: testLayer1.MVTName(),
						Type:        style.LayerTypeCircle,
						Layout: &style.LayerLayout{
							Visibility: "visible",
						},
						Paint: &style.LayerPaint{
							CircleRadius: 3,
							CircleColor:  "#56f8aa",
						},
					},
					{
						ID:          testLayer2.MVTName(),
						Source:      testMapName,
						SourceLayer: testLayer2.MVTName(),
						Type:        style.LayerTypeLine,
						Layout: &style.LayerLayout{
							Visibility: "visible",
						},
						Paint: &style.LayerPaint{
							LineColor: "#9d70ab",
						},
					},
				},
			},
		},
		"uri prefix set": {
			handler:        server.HandleMapStyle{},
			uriPrefix:      "/tegola",
			uri:            path.Join("/tegola", "maps", testMapName, "style.json"),
			uriPattern:     "/tegola/maps/:map_name/style.json",
			serverHostName: serverHostName,
			expected: style.Root{
				Name:    testMapName,
				Version: style.Version,
				Center:  [2]float64{testMapCenter[0], testMapCenter[1]},
				Zoom:    testMapCenter[2],
				Sources: map[string]style.Source{
					testMapName: {
						Type: style.SourceTypeVector,
						URL: (&url.URL{
							Scheme: "http",
							Host:   serverHostName,
							Path:   path.Join(server.URIPrefix, "tegola", "capabilities", testMapName+".json"),
						}).String(),
					},
				},
				Layers: []style.Layer{
					{
						ID:          testLayer1.MVTName(),
						Source:      testMapName,
						SourceLayer: testLayer1.MVTName(),
						Type:        style.LayerTypeCircle,
						Layout: &style.LayerLayout{
							Visibility: "visible",
						},
						Paint: &style.LayerPaint{
							CircleRadius: 3,
							CircleColor:  "#56f8aa",
						},
					},
					{
						ID:          testLayer2.MVTName(),
						Source:      testMapName,
						SourceLayer: testLayer2.MVTName(),
						Type:        style.LayerTypeLine,
						Layout: &style.LayerLayout{
							Visibility: "visible",
						},
						Paint: &style.LayerPaint{
							LineColor: "#9d70ab",
						},
					},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestHandleMapStylePolygonLayer(t *testing.T) {
	server.HostName = &url.URL{
		Host: serverHostName,
	}
	server.URIPrefix = "/"

	// the style handler looks maps up in the package level atlas, so
	// register the polygon test map there under a unique name.
	polyMapName := "polygon-style-test-map"
	polyMap := atlas.NewWebMercatorMap(polyMapName)
	polyMap.Attribution = testMapAttribution
	polyMap.Center = testMapCenter
	polyMap.Layers = append(polyMap.Layers, testLayer4)
	atlas.AddMap(polyMap)

	resp, _, err := doRequest(t, nil, http.MethodGet, path.Join("/maps", polyMapName, "style.json"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if resp.Code != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got (%d) expected (%d)", resp.Code, http.StatusOK)
	}

	var output style.Root
	if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
		t.Fatalf("unable to unmarshal JSON response body: %s", err)
	}

	expected := style.Root{
		Name:    polyMapName,
		Version: style.Version,
		Center:  [2]float64{testMapCenter[0], testMapCenter[1]},
		Zoom:    testMapCenter[2],
		Sources: map[string]style.Source{
			polyMapName: {
				Type: style.SourceTypeVector,
				URL: (&url.URL{
					Scheme: "http",
					Host:   serverHostName,
					Path:   path.Join(server.URIPrefix, "capabilities", polyMapName+".json"),
				}).String(),
			},
		},
		Layers: polygonLayerStyleLayers(testLayer4.MVTName(), polyMapName),
	}

	if diff := deep.Equal(output, expected); diff != nil {
		t.Fatalf("output does not match expected. diff %s", diff)
	}
}

func TestHandleMapStyleCORS(t *testing.T) {
	tests := map[string]CORSTestCase{
		"1": {
			uri: path.Join("maps", testMapName, "style.json"),
		},
	}

	for name, tc := range tests {
		t.Run(name, CORSTest(tc))
	}
}
