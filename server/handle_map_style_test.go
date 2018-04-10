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
	// config params this test relies on
	server.HostName = serverHostName

	testcases := []struct {
		handler    http.Handler
		hostName   string
		port       string
		uri        string
		uriPattern string
		reqMethod  string
		expected   style.Root
	}{
		{
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
	}

	for i, tc := range testcases {
		var err error

		w, _, err := doRequest(nil, tc.reqMethod, tc.uri, nil)
		if err != nil {
			t.Errorf("[%v] failed: %v", i, err)
			continue
		}

		if w.Code != http.StatusOK {
			t.Errorf("[%v] handler returned wrong status code: got (%v) expected (%v)", i, w.Code, http.StatusOK)
			continue
		}

		bytes, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Errorf("[%v] err reading response body: %v", i, err)
			continue
		}

		var output style.Root
		// read the response body
		if err := json.Unmarshal(bytes, &output); err != nil {
			t.Errorf("[%v] unable to unmarshal JSON response body: %v", i, err)
			continue
		}

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("[%v] failed. output \n\n %+v \n\n does not match expected \n\n %+v", i, output, tc.expected)
			continue
		}
	}
}
