package server_test

import (
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"

	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/mvt/vector_tile"
)

type MapHandlerTCase struct {
	method string
	uri    string
	atlas  *atlas.Atlas

	expectedBody   string
	expectedCode   int
	expectedLayers []string
}

func MapHandlerTester(t *testing.T, tc MapHandlerTCase) {
	// setup a new router. This handles parsing our URL wildcards.

	a := tc.atlas
	if a == nil {
		a = newTestMapWithLayers(testLayer1, testLayer2, testLayer3)
	}
	w, _, err := doRequest(a, tc.method, tc.uri, nil)

	if w.Code != tc.expectedCode {
		wbody := strings.TrimSpace(w.Body.String())
		t.Log("wbody", wbody)
		t.Errorf("status code, expected %v got %v", tc.expectedCode, w.Code)
	}
	// Only try and decode as string for errors.
	if len(tc.expectedBody) > 0 && tc.expectedCode >= 400 {
		wbody := strings.TrimSpace(w.Body.String())
		if string(tc.expectedBody) != wbody {
			t.Errorf("body, expected %v got %v", string(tc.expectedBody), wbody)
		}
	}

	// success check
	if len(tc.expectedLayers) > 0 {
		var tile vectorTile.Tile
		var responseBodyBytes []byte

		responseBodyBytes, err = ioutil.ReadAll(w.Body)
		if err != nil {
			t.Errorf("reading response body, expected nil got %v", err)
			return
		}

		if err = proto.Unmarshal(responseBodyBytes, &tile); err != nil {
			t.Errorf("unmarshalling response body, expected nil got %v", err)
			return
		}

		var tileLayers []string
		// extract all the layers names in the response
		for i := range tile.Layers {
			tileLayers = append(tileLayers, *tile.Layers[i].Name)
		}

		if !reflect.DeepEqual(tc.expectedLayers, tileLayers) {
			t.Errorf("layers, expected %v got %v", tc.expectedLayers, tileLayers)
			return
		}
	}
}

func TestHandleMapLayerZXY(t *testing.T) {
	tests := map[string]MapHandlerTCase{
		"Max Zoom, no layers left issue-375": {
			uri:          "/maps/test-map/test-layer/10/2/3.pbf",
			atlas:        newTestMapWithLayers(testLayer1), // Max Zoom on Layer1 is 9.
			expectedCode: http.StatusNotFound,
		},
		"std": {
			uri:            "/maps/test-map/test-layer/4/2/3.pbf",
			expectedCode:   http.StatusOK,
			expectedLayers: []string{"test-layer"},
		},
		"std debug": {
			uri:            "/maps/test-map/test-layer/10/2/3.pbf?debug=true",
			expectedCode:   http.StatusOK,
			expectedLayers: []string{"test-layer", "debug-tile-outline", "debug-tile-center"},
		},
		"neg row(y) not allowed issue-229": {
			uri:          "/maps/test-map/test-layer/1/1/-1.pbf",
			expectedCode: http.StatusBadRequest,
			expectedBody: "invalid Y value (-1)",
		},
		"neg col(x) not allowed issue-229": {
			uri:          "/maps/test-map/test-layer/1/-1/3.pbf",
			expectedCode: http.StatusBadRequest,
			expectedBody: "invalid X value (-1)",
		},
		"neg zoom(z) not allowed issue-163": {
			uri:          "/maps/test-map/test-layer/-1/0/0.pbf",
			expectedCode: http.StatusBadRequest,
			expectedBody: "invalid Z value (-1)",
		},
		"invalid x issue-334": {
			uri:          "/maps/test-map/test-layer/1/4/0.pbf",
			expectedCode: http.StatusBadRequest,
			expectedBody: "invalid X value (4)",
		},
		"invalid y issue-334": {
			uri:          "/maps/test-map/test-layer/1/0/4.pbf",
			expectedCode: http.StatusBadRequest,
			expectedBody: "invalid Y value (4)",
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { MapHandlerTester(t, tc) })
	}
}

func TestHandleMapZXY(t *testing.T) {
	tests := map[string]MapHandlerTCase{
		"Max Zoom, no layers left issue-375": {
			uri:          "/maps/test-map/10/2/3.pbf",
			atlas:        newTestMapWithLayers(testLayer1), // Max Zoom on Layer1 is 9.
			expectedCode: http.StatusNotFound,
		},
		"std 4_2_3": {
			uri:            "/maps/test-map/4/2/3.pbf",
			expectedCode:   http.StatusOK,
			expectedLayers: []string{"test-layer"},
		},
		"std 4_2_3 debug": {
			uri:            "/maps/test-map/4/2/3.pbf?debug=true",
			expectedCode:   http.StatusOK,
			expectedLayers: []string{"test-layer", "debug-tile-outline", "debug-tile-center"},
		},
		"std": {
			uri:            "/maps/test-map/10/2/3.pbf",
			expectedCode:   http.StatusOK,
			expectedLayers: []string{"test-layer-2-name", "test-layer"},
		},
		"std debug": {
			uri:            "/maps/test-map/10/2/3.pbf?debug=true",
			expectedCode:   http.StatusOK,
			expectedLayers: []string{"test-layer-2-name", "test-layer", "debug-tile-outline", "debug-tile-center"},
		},
		"neg row(y) not allowed issue-229": {
			uri:          "/maps/test-map/1/1/-1.pbf",
			expectedCode: http.StatusBadRequest,
			expectedBody: "invalid Y value (-1)",
		},
		"neg col(x) not allowed issue-229": {
			uri:          "/maps/test-map/1/-1/3.pbf",
			expectedCode: http.StatusBadRequest,
			expectedBody: "invalid X value (-1)",
		},
		"neg zoom(z) not allowed issue-163": {
			uri:          "/maps/test-map/-1/0/0.pbf",
			expectedCode: http.StatusBadRequest,
			expectedBody: "invalid Z value (-1)",
		},
		"invalid x issue-334": {
			uri:          "/maps/test-map/1/4/0.pbf",
			expectedCode: http.StatusBadRequest,
			expectedBody: "invalid X value (4)",
		},
		"invalid y issue-334": {
			uri:          "/maps/test-map/1/0/4.pbf",
			expectedCode: http.StatusBadRequest,
			expectedBody: "invalid Y value (4)",
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { MapHandlerTester(t, tc) })
	}
}
