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

	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/mvt/vector_tile"
	"github.com/go-spatial/tegola/server"
)

func TestMapWithNoLayersLeftErrors(t *testing.T) {
	type tcase struct {
		Method string
		uri    string
		atlas  *atlas.Atlas

		expectedBody   string
		expectedCode   int
		expectedLayers []string
	}
	fn := func(t *testing.T, tc tcase) {
		// setup a new router. This handles parsing our URL wildcards.
		router := httptreemux.New()

		// Default Method to GET
		if tc.Method == "" {
			tc.Method = "GET"
		}

		// setup a router group
		group := router.NewGroup("/")
		{
			hmzxy := server.HandleMapZXY{Atlas: tc.atlas}
			group.UsingContext().Handler(
				tc.Method, hmzxy.Scheme(),
				hmzxy,
			)
		}
		r, err := http.NewRequest(tc.Method, tc.uri, nil)
		if err != nil {
			t.Fatal(err)
			return
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
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
	tests := map[string]tcase{
		"Max Zoom, no layers left issue-375": {
			uri:          "/maps/test-map/10/2/3.pbf",
			atlas:        newTestMapWithLayers(testLayer1), // Max Zoom on Layer1 is 9.
			expectedCode: http.StatusBadRequest,
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}
