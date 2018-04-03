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

func TestHandleMapLayerZXY(t *testing.T) {
	// setup a new provider
	testcases := []struct {
		uri            string
		uriPattern     string
		reqMethod      string
		expectedCode   int
		expectedBody   []byte
		expectedLayers []string
	}{
		{
			uri:            "/maps/test-map/test-layer/4/2/3.pbf",
			uriPattern:     "/maps/:map_name/:layer_name/:z/:x/:y",
			reqMethod:      "GET",
			expectedCode:   http.StatusOK,
			expectedLayers: []string{"test-layer"},
		},
		{
			uri:            "/maps/test-map/test-layer/10/2/3.pbf?debug=true",
			uriPattern:     "/maps/:map_name/:layer_name/:z/:x/:y",
			reqMethod:      "GET",
			expectedCode:   http.StatusOK,
			expectedLayers: []string{"test-layer", "debug-tile-outline", "debug-tile-center"},
		},
		{ // Negative row (y) not allowed (issue-229)
			uri:          "/maps/test-map/test-layer/1/1/-1.pbf",
			uriPattern:   "/maps/:map_name/:layer_name/:z/:x/:y",
			reqMethod:    "GET",
			expectedCode: http.StatusBadRequest,
			expectedBody: []byte("invalid Y value (-1.pbf)"),
		},
		{ // Negative column (x) not allowed
			uri:          "/maps/test-map/test-layer/1/-1/3.pbf",
			uriPattern:   "/maps/:map_name/:layer_name/:z/:x/:y",
			reqMethod:    "GET",
			expectedCode: http.StatusBadRequest,
			expectedBody: []byte("invalid X value (-1)"),
		},
		{ // issue-163
			uri:          "/maps/test-map/test-layer/-1/0/0.pbf",
			uriPattern:   "/maps/:map_name/:layer_name/:z/:x/:y",
			reqMethod:    "GET",
			expectedCode: http.StatusBadRequest,
			expectedBody: []byte("invalid Z value (-1)"),
		},
		{ // issue-334
			uri:          "/maps/test-map/test-layer/1/4/0.pbf",
			uriPattern:   "/maps/:map_name/:layer_name/:z/:x/:y",
			reqMethod:    "GET",
			expectedCode: http.StatusBadRequest,
			expectedBody: []byte("invalid X value (4)"),
		},
		{ // issue-334
			uri:          "/maps/test-map/test-layer/1/0/4.pbf",
			uriPattern:   "/maps/:map_name/:layer_name/:z/:x/:y",
			reqMethod:    "GET",
			expectedCode: http.StatusBadRequest,
			expectedBody: []byte("invalid Y value (4.pbf)"),
		},
	}

	for i, test := range testcases {
		var err error

		// setup a new router. this handles parsing our URL wildcards (i.e. :map_name, :z, :x, :y)
		router := httptreemux.New()
		// setup a new router group
		group := router.NewGroup("/")
		group.UsingContext().Handler(test.reqMethod, test.uriPattern, server.HandleMapLayerZXY{})

		r, err := http.NewRequest(test.reqMethod, test.uri, nil)
		if err != nil {
			t.Errorf("[%v] new request, expected nil got %v", i, err)
			continue
		}

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		if w.Code != test.expectedCode {
			t.Errorf("[%v] status code, expected %v got %v", i, test.expectedCode, w.Code)
			continue
		}

		// error checking
		if len(test.expectedBody) > 0 && test.expectedCode >= 400 {
			wbody := strings.TrimSpace(w.Body.String())

			if string(test.expectedBody) != wbody {
				t.Errorf("[%v] body,  expected %v got %v", i, string(test.expectedBody), wbody)
				continue
			}
			continue
		}

		// success check
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
			// extract all the layers names in the response
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
