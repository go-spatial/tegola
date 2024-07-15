package server_test

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/dimfeld/httptreemux"
	"github.com/go-spatial/geom"

	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/provider/test"
	"github.com/go-spatial/tegola/server"
)

// test server config
const (
	httpPort       = ":8080"
	serverVersion  = "0.10.0"
	serverHostName = "tegola.io"
	serverCert     = "testcert/cert.pem"
	serverKey      = "testcert/key.pem"
)

var (
	testMapName        = "test-map"
	testMapAttribution = "test attribution"
	testMapCenter      = [3]float64{1.0, 2.0, 3.0}
)

var testLayer1 = atlas.Layer{
	Name:              "test-layer",
	ProviderLayerName: "test-layer-1",
	MinZoom:           4,
	MaxZoom:           9,
	Provider:          &test.TileProvider{},
	GeomType:          geom.Point{},
	DefaultTags: map[string]any{
		"foo": "bar",
	},
}

var testLayer2 = atlas.Layer{
	Name:              "test-layer-2-name",
	ProviderLayerName: "test-layer-2-provider-layer-name",
	MinZoom:           10,
	MaxZoom:           15,
	Provider:          &test.TileProvider{},
	GeomType:          geom.Line{},
	DefaultTags: map[string]any{
		"foo": "bar",
	},
}

var testLayer3 = atlas.Layer{
	Name:              "test-layer",
	ProviderLayerName: "test-layer-3",
	MinZoom:           10,
	MaxZoom:           20,
	Provider:          &test.TileProvider{},
	GeomType:          geom.Point{},
	DefaultTags:       map[string]any{},
}

func newTestMapWithLayers(layers ...atlas.Layer) *atlas.Atlas {

	testMap := atlas.NewWebMercatorMap(testMapName)
	testMap.Attribution = testMapAttribution
	testMap.Center = testMapCenter
	testMap.Layers = append(testMap.Layers, layers...)

	a := &atlas.Atlas{}
	a.AddMap(testMap)

	return a
}

func newTestMapWithBounds(minx, miny, maxx, maxy float64) *atlas.Atlas {

	testMap := atlas.NewWebMercatorMap(testMapName)
	testMap.Attribution = testMapAttribution
	testMap.Center = testMapCenter
	testMap.Layers = append(testMap.Layers, testLayer1)
	testMap.Bounds = &geom.Extent{minx, miny, maxx, maxy}

	a := &atlas.Atlas{}
	a.AddMap(testMap)

	return a
}

func doRequest(t *testing.T, a *atlas.Atlas, method string, uri string, body io.Reader) (*httptest.ResponseRecorder, *httptreemux.TreeMux, error) {
	t.Helper()

	// Default Method to GET
	if method == "" {
		method = http.MethodGet
	}

	r, err := http.NewRequest(method, uri, body)
	if err != nil {
		return nil, nil, err
	}

	router := server.NewRouter(a)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	return w, router, nil
}

// pre test setup phase
func TestMain(m *testing.M) {
	server.Version = serverVersion
	server.HostName = &url.URL{
		Host: serverHostName,
	}

	testMap := atlas.NewWebMercatorMap(testMapName)
	testMap.Attribution = testMapAttribution
	testMap.Center = testMapCenter
	testMap.Layers = append(testMap.Layers,
		testLayer1,
		testLayer2,
		testLayer3,
	)

	// register a map with atlas
	atlas.AddMap(testMap)

	os.Exit(m.Run())
}

func TestURLRoot(t *testing.T) {
	type tcase struct {
		request  http.Request
		hostName *url.URL
		expected *url.URL
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {

			server.HostName = tc.hostName

			output := server.URLRoot(&tc.request)
			if !reflect.DeepEqual(output, tc.expected) {
				t.Errorf("expected (%+v) got (%+v)", tc.expected, output)
			}
		}
	}

	tests := map[string]tcase{
		"http": {
			request: http.Request{},
			hostName: &url.URL{
				Host: serverHostName,
			},
			expected: &url.URL{
				Scheme: "http",
				Host:   serverHostName,
			},
		},
		"https": {
			request: http.Request{
				TLS: &tls.ConnectionState{},
			},
			hostName: &url.URL{
				Host: serverHostName,
			},
			expected: &url.URL{
				Scheme: "https",
				Host:   serverHostName,
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestHTTPS(t *testing.T) {
	server.SSLCert = serverCert
	server.SSLKey = serverKey

	// start server
	srv := server.Start(nil, ":8123")
	time.Sleep(time.Second)

	// set de-secure the tls client
	hc := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// don't run other tests on https
	defer func() {
		server.SSLCert = ""
		server.SSLKey = ""
		srv.Shutdown(context.Background())
	}()

	type tcase struct {
		url  string
		code int
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			_, err := hc.Get(tc.url)
			if err != nil {
				t.Errorf("unexpected error %v", err)
				return
			}

			// TODO(ear7h): there seems to be a race condition on setting the
			// routes so this is not tested
			//if res.StatusCode != tc.code {
			//	t.Errorf("incorrect status code %v, expected %v", res.StatusCode, tc.code)
			//}
		}
	}

	testcases := map[string]tcase{
		"root": {
			url:  "https://localhost:8123/",
			code: http.StatusOK,
		},
		"capabilities": {
			url:  "https://localhost:8123/capabilities",
			code: http.StatusOK,
		},
	}

	for k, v := range testcases {
		t.Run(k, fn(v))
	}
}
