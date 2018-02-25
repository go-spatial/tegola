package server_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/dimfeld/httptreemux"
	"github.com/golang/protobuf/proto"

	"github.com/go-spatial/tegola/mvt/vector_tile"
	"github.com/go-spatial/tegola/server"
)

func TestHandleMapZXY(t *testing.T) {
	//	setup a new provider
	type tc struct {
		uri            string
		uriPattern     string
		reqMethod      string
		expected       []byte
		expectedCode   int
		expectedLayers []string
	}

	testcases := map[string]tc{
		"0": {
			uri:            "/maps/test-map/10/2/3.pbf",
			uriPattern:     "/maps/:map_name/:z/:x/:y",
			reqMethod:      "GET",
			expectedCode:   http.StatusOK,
			expectedLayers: []string{"test-layer-2-name", "test-layer"},
		},
		"1": {
			uri:            "/maps/test-map/10/2/3.pbf?debug=true",
			uriPattern:     "/maps/:map_name/:z/:x/:y",
			reqMethod:      "GET",
			expectedCode:   http.StatusOK,
			expectedLayers: []string{"test-layer-2-name", "test-layer", "debug-tile-outline", "debug-tile-center"},
		},
		"2": { // issue-163
			uri:          "/maps/test-map/-1/0/0.pbf",
			uriPattern:   "/maps/:map_name/:z/:x/:y",
			reqMethod:    "GET",
			expectedCode: http.StatusBadRequest,
			expected:     []byte("invalid Z value (-1)"),
		},
		"3": { // Check that values outside expected zoom result in 404.
			uri:          "/maps/test-map/-1/2/3.pbf",
			uriPattern:   "/maps/:map_name/:z/:x/:y",
			reqMethod:    "GET",
			expectedCode: http.StatusBadRequest,
		},
		"4": { // Check that negative y value results in 404. (issue-229)
			uri:          "/maps/test-map/1/2/-1.pbf",
			uriPattern:   "/maps/:map_name/:z/:x/:y",
			reqMethod:    "GET",
			expectedCode: http.StatusBadRequest,
		},
		"5": { // Check that nagative x value results in 404.
			uri:          "/maps/test-map/1/-1/3.pbf",
			uriPattern:   "/maps/:map_name/:z/:x/:y",
			reqMethod:    "GET",
			expectedCode: http.StatusBadRequest,
		},
	}

	for i, test := range testcases {
		var err error

		//	setup a new router. this handles parsing our URL wildcards (i.e. :map_name, :z, :x, :y)
		router := httptreemux.New()
		//	setup a new router group
		group := router.NewGroup("/")
		group.UsingContext().Handler(test.reqMethod, test.uriPattern, server.HandleMapZXY{})

		r, err := http.NewRequest(test.reqMethod, test.uri, nil)
		if err != nil {
			t.Fatal(err)
			continue
		}

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		if w.Code != test.expectedCode {
			t.Errorf("[%v] status code, expected %v got %v", i, test.expectedCode, w.Code)
		}

		// Only try to decode as string for errors.
		if len(test.expected) > 0 && test.expectedCode >= 400 {
			wbody := strings.TrimSpace(w.Body.String())

			if string(test.expected) != wbody {
				t.Errorf("[%v] body, expected %v got %v", i, string(test.expected), wbody)
			}
		}

		//	success check
		if len(test.expectedLayers) > 0 {
			var tile vectorTile.Tile
			var responseBodyBytes []byte

			responseBodyBytes, err = ioutil.ReadAll(w.Body)
			if err != nil {
				t.Errorf("[%v] error reading response body, %v", i, err)
				continue
			}

			if err = proto.Unmarshal(responseBodyBytes, &tile); err != nil {
				t.Errorf("[%v] error unmarshalling response body, %v", i, err)
				continue
			}

			var tileLayers []string
			//	extract all the layers names in the response
			for i := range tile.Layers {
				tileLayers = append(tileLayers, *tile.Layers[i].Name)
			}

			if !reflect.DeepEqual(test.expectedLayers, tileLayers) {
				t.Errorf("[%v] layers, expected %v got %v", i, test.expectedLayers, tileLayers)
				continue
			}
		}
	}
}
