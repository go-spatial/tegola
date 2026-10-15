package server_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/dimfeld/httptreemux"

	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/mapbox/style"
	"github.com/alexeydott/tegola/provider/test"
	"github.com/alexeydott/tegola/server"
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

var testLayer4 = atlas.Layer{
	Name:              "test-layer-polygon",
	ProviderLayerName: "test-layer-4",
	MinZoom:           0,
	MaxZoom:           22,
	Provider:          &test.TileProvider{},
	GeomType:          geom.Polygon{},
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

// polygonLayerStyleLayers returns the expected style layers for a polygon
// geometry layer: a fill layer restricted to polygons plus a sibling line
// layer for polylines served by the same source layer.
func polygonLayerStyleLayers(name, mapName string) []style.Layer {
	return []style.Layer{
		{
			ID:          name + "-line",
			Source:      mapName,
			SourceLayer: name,
			Type:        style.LayerTypeLine,
			Filter:      []interface{}{"==", "$type", "LineString"},
			Layout: &style.LayerLayout{
				Visibility: "visible",
			},
			Paint: &style.LayerPaint{
				LineColor: "#c37a13",
			},
		},
		{
			ID:          name,
			Source:      mapName,
			SourceLayer: name,
			Type:        style.LayerTypeFill,
			Filter:      []interface{}{"==", "$type", "Polygon"},
			Layout: &style.LayerLayout{
				Visibility: "visible",
			},
			Paint: &style.LayerPaint{
				FillColor:        "rgba(195,122,19,0.1)",
				FillOutlineColor: "#c37a13",
			},
		},
	}
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

// waitForPort dials addr until it accepts a TCP connection or the timeout
// elapses. It lets tests wait for a listener to become ready instead of
// relying on a fixed sleep.
func waitForPort(addr string) error {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 250*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s", addr)
}

func TestHTTPS(t *testing.T) {
	server.SSLCert = serverCert
	server.SSLKey = serverKey

	// start server
	srv := server.Start(nil, ":8123")

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
		if err := srv.Shutdown(context.Background()); err != nil {
			t.Errorf("error shutting down test server: %v", err)
		}
	}()

	// Routes are registered synchronously inside Start before the listener is
	// started, so once the listener accepts connections the routes are ready.
	// Wait for readiness instead of a fixed sleep so assertions are deterministic.
	if err := waitForPort("localhost:8123"); err != nil {
		t.Fatalf("server did not start: %v", err)
	}

	type tcase struct {
		url  string
		code int
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			res, err := hc.Get(tc.url)
			if err != nil {
				t.Errorf("unexpected error %v", err)
				return
			}
			defer res.Body.Close()

			if res.StatusCode != tc.code {
				t.Errorf("incorrect status code %v, expected %v", res.StatusCode, tc.code)
			}
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
