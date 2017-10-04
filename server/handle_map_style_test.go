package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/dimfeld/httptreemux"
	"github.com/terranodo/tegola/mapbox/style"
	"github.com/terranodo/tegola/server"
)

func TestHandleMapStyle(t *testing.T) {
	//	config params this test relies on
	server.HostName = serverHostName

	//	setup a new provider
	testcases := []struct {
		handler    http.Handler
		uri        string
		uriPattern string
		reqMethod  string
		expected   style.Root
	}{
		{
			handler:    server.HandleMapStyle{},
			uri:        fmt.Sprintf("/maps/%v/style.json", testMap.Name),
			uriPattern: "/maps/:map_name/style.json",
			reqMethod:  "GET",
			expected: style.Root{
				Name:    testMap.Name,
				Version: style.Version,
				Center:  [2]float64{testMap.Center[0], testMap.Center[1]},
				Zoom:    testMap.Center[2],
				Sources: map[string]style.Source{
					testMap.Name: style.Source{
						Type: style.SourceTypeVector,
						URL:  fmt.Sprintf("http://%v/capabilities/%v.json", serverHostName, testMap.Name),
					},
				},
				Layers: []style.Layer{
					{
						ID:          testLayer1.MVTName(),
						Source:      testMap.Name,
						SourceLayer: testLayer1.Name,
						Type:        style.LayerTypeCircle,
						Layout: &style.LayerLayout{
							Visibility: "visible",
						},
						Paint: &style.LayerPaint{
							CircleRadius: 3,
							CircleColor:  "#7a40ce",
						},
					},
					{
						ID:          testLayer2.MVTName(),
						Source:      testMap.Name,
						SourceLayer: testLayer2.Name,
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

	for i, tc := range testcases {
		var err error

		//	setup a new router. this handles parsing our URL wildcards (i.e. :map_name, :z, :x, :y)
		router := httptreemux.New()
		//	setup a new router group
		group := router.NewGroup("/")
		group.UsingContext().Handler(tc.reqMethod, tc.uriPattern, tc.handler)

		r, err := http.NewRequest(tc.reqMethod, tc.uri, nil)
		if err != nil {
			t.Errorf("test case (%v) failed: %v", i, err)
			continue
		}

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("test case (%v). handler returned wrong status code: got (%v) expected (%v)", i, w.Code, http.StatusOK)
		}

		var output style.Root
		if err = json.NewDecoder(w.Body).Decode(&output); err != nil {
			t.Errorf("test case (%v) failed: %v", i, err)
			continue
		}

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("test case (%v) failed. output \n\n %+v \n\n does not match expected \n\n %+v", i, output, tc.expected)
		}
	}
}
