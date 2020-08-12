package server_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/mapbox/style"
	"github.com/go-spatial/tegola/server"
)

func TestHandleMapStyle(t *testing.T) {
	type tcase struct {
		handler    http.Handler
		uriPrefix  string
		hostName   string
		port       string
		uri        string
		uriPattern string
		reqMethod  string
		expected   style.Root
	}

	// config params this test relies on
	server.HostName = serverHostName

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			var err error

			if tc.uriPrefix != "" {
				server.URIPrefix = tc.uriPrefix
			} else {
				server.URIPrefix = "/"
			}

			w, _, err := doRequest(nil, tc.reqMethod, tc.uri, nil)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if w.Code != http.StatusOK {
				t.Errorf("handler returned wrong status code: got (%v) expected (%v)", w.Code, http.StatusOK)
				return
			}

			bytes, err := ioutil.ReadAll(w.Body)
			if err != nil {
				t.Errorf("err reading response body: %v", err)
				return
			}

			var output style.Root
			// read the response body
			if err := json.Unmarshal(bytes, &output); err != nil {
				t.Errorf("unable to unmarshal JSON response body: %v", err)
				return

			}

			if !reflect.DeepEqual(output, tc.expected) {
				t.Errorf("failed. output \n\n %+v \n\n does not match expected \n\n %+v", output, tc.expected)
				return
			}
		}
	}

	tests := map[string]tcase{
		"default": {
			handler:    server.HandleMapStyle{},
			uri:        fmt.Sprintf("/maps/%v/style.json", testMapName),
			uriPattern: "/maps/:map_name/style.json",
			reqMethod:  "GET",
			expected: style.Root{
				Name:    testMapName,
				Version: style.Version,
				Center:  [2]float64{testMapCenter[0], testMapCenter[1]},
				Zoom:    testMapCenter[2],
				Sources: map[string]style.Source{
					testMapName: {
						Type: style.SourceTypeVector,
						URL:  fmt.Sprintf("http://%v/capabilities/%v.json", serverHostName, testMapName),
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
			handler:    server.HandleMapStyle{},
			uriPrefix:  "/tegola",
			uri:        fmt.Sprintf("/tegola/maps/%v/style.json", testMapName),
			uriPattern: "/tegola/maps/:map_name/style.json",
			reqMethod:  "GET",
			expected: style.Root{
				Name:    testMapName,
				Version: style.Version,
				Center:  [2]float64{testMapCenter[0], testMapCenter[1]},
				Zoom:    testMapCenter[2],
				Sources: map[string]style.Source{
					testMapName: {
						Type: style.SourceTypeVector,
						URL:  fmt.Sprintf("http://%v/tegola/capabilities/%v.json", serverHostName, testMapName),
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

func TestHandleMapStyleCORS(t *testing.T) {
	tests := map[string]CORSTestCase{
		"1": {
			uri: fmt.Sprintf("/maps/%v/style.json", testMapName),
		},
	}

	for name, tc := range tests {
		t.Run(name, CORSTest(tc))
	}
}
