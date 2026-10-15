//go:build cgo

package lambda_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider/gpkg"
	"github.com/alexeydott/tegola/server"
	lambdaserver "github.com/alexeydott/tegola/server/lambda"
)

func migrationRouter(t *testing.T) http.Handler {
	t.Helper()
	path := filepath.Join(t.TempDir(), "migration.gpkg")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		}()
		wire, err := os.ReadFile("../../testdata/migration/jivan/fixture.sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(wire)); err != nil {
			t.Fatal(err)
		}
	}()
	layers := []map[string]any{}
	for _, name := range []string{"xy", "xyz", "bare"} {
		layer := map[string]any{"name": name, "tablename": name, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 4326}
		if name != "bare" {
			layer["fields"] = []string{"n", "s", "b"}
			layer["temporal_field"], layer["temporal_storage"] = "at", "unix_seconds"
		}
		if name == "xyz" {
			layer["spatial_dimension"], layer["vertical_crs"] = "xyz", "http://www.opengis.net/def/crs/OGC/0/CRS84h"
		}
		layers = append(layers, layer)
	}
	tiler, err := gpkg.NewTileProvider(dict.Dict{"filepath": path, "layers": layers}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := tiler.(*gpkg.Provider)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	})
	info, err := p.Layers()
	if err != nil {
		t.Fatal(err)
	}
	sources := []features.CollectionSource{}
	for _, layer := range info {
		sources = append(sources, features.CollectionSource{ID: layer.Name(), Layer: layer, Querier: p})
	}
	svc, err := features.NewService(sources)
	if err != nil {
		t.Fatal(err)
	}
	api, err := server.NewFeatureAPI(svc, server.FeatureAPIConfig{BasePath: "/features", DefaultLimit: 2, MaxLimit: 100})
	if err != nil {
		t.Fatal(err)
	}
	router, err := server.NewRouterWithOptions(nil, server.RouterOptions{Features: api})
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func migrationEvent(t *testing.T, mode, method, target string) []byte {
	t.Helper()
	uri, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	query := uri.Query()
	headers := map[string]string{"Accept": "application/json", "Host": "attacker.invalid", "X-Forwarded-Host": "attacker.invalid", "X-Forwarded-Proto": "http"}
	event := map[string]any{"body": "", "isBase64Encoded": false}
	switch mode {
	case "rest-v1":
		event["httpMethod"], event["path"], event["headers"] = method, uri.Path, headers
		event["multiValueQueryStringParameters"] = query
		event["requestContext"] = map[string]any{"accountId": "owned", "domainName": "gateway.example", "stage": "prod"}
	case "http-v2", "function-url-v2":
		event["version"], event["rawPath"], event["rawQueryString"], event["headers"] = "2.0", uri.Path, uri.RawQuery, headers
		stage := "prod"
		if mode == "function-url-v2" {
			stage = "$default"
		}
		event["requestContext"] = map[string]any{"domainName": "gateway.example", "stage": stage, "http": map[string]string{"method": method, "path": uri.Path}}
	case "alb-multi":
		event["httpMethod"], event["path"] = method, uri.Path
		event["multiValueHeaders"] = map[string][]string{"Accept": {"application/json"}, "Host": {"attacker.invalid"}}
		encoded := map[string][]string{}
		for key, values := range query {
			for _, value := range values {
				encoded[url.QueryEscape(key)] = append(encoded[url.QueryEscape(key)], url.QueryEscape(value))
			}
		}
		event["multiValueQueryStringParameters"] = encoded
		event["requestContext"] = map[string]any{"elb": map[string]string{"targetGroupArn": "arn:owned:test"}}
	default:
		t.Fatal("Unknown owned event mode")
	}
	wire, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return wire
}

func TestMigrationLambdaRealGPKGParity(t *testing.T) {
	oldRoot, oldPrefix := server.URLRoot, server.URIPrefix
	public, _ := url.Parse("https://migration.example/mapping")
	server.URIPrefix = "/nested"
	server.URLRoot = func(r *http.Request) *url.URL {
		if root := lambdaserver.URLRoot(r); root.Host != "" {
			return root
		}
		copy := *public
		return &copy
	}
	t.Cleanup(func() { server.URLRoot, server.URIPrefix = oldRoot, oldPrefix })
	router := migrationRouter(t)
	direct := httptest.NewServer(router)
	t.Cleanup(direct.Close)
	adapter, err := lambdaserver.New(router, lambdaserver.Options{PublicURL: public})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"rest-v1", "http-v2", "function-url-v2", "alb-multi"} {
		t.Run(mode, func(t *testing.T) {
			for _, tc := range []struct {
				name, method, path string
				status             int
				ids                []uint64
			}{
				{"landing", "GET", "?f=json", 200, nil},
				{"collections", "GET", "/collections?f=json", 200, nil},
				{"html", "GET", "/collections/xy/items?f=html", 200, nil},
				{"composed", "GET", "/collections/xy/items?f=json&limit=100&filter=n%3E0&bbox=14,29,17,32&datetime=1970-01-01T00%3A00%3A00Z", 200, []uint64{10, 30, 60}},
				{"untimed", "GET", "/collections/bare/items?f=json&limit=100&datetime=2099-01-01T00%3A00%3A00Z", 200, []uint64{10, 20, 30, 40, 50, 60}},
				{"page", "GET", "/collections/xy/items?f=json&limit=2&offset=2", 200, []uint64{30, 40}},
				{"xyz", "GET", "/collections/xyz/items/10?f=json", 200, nil},
				{"null", "GET", "/collections/xy/items/40?f=json", 200, nil},
				{"head", "HEAD", "/collections/xy/items?f=json", 200, nil},
				{"options", "OPTIONS", "/collections/xy/items?f=json", 200, nil},
				{"missing", "GET", "/collections/xy/items/999?f=json", 404, nil},
				{"bad-id", "GET", "/collections/xy/items/-1?f=json", 400, nil},
				{"repeated", "GET", "/collections/xy/items?f=json&limit=1&limit=2", 400, nil},
			} {
				t.Run(tc.name, func(t *testing.T) {
					target := "/nested/features" + tc.path
					request, err := http.NewRequest(tc.method, direct.URL+target, nil)
					if err != nil {
						t.Fatal(err)
					}
					request.Header.Set("Accept", "application/json")
					response, err := direct.Client().Do(request)
					if err != nil {
						t.Fatal(err)
					}
					body, err := io.ReadAll(response.Body)
					if closeErr := response.Body.Close(); closeErr != nil {
						t.Error(closeErr)
					}
					if err != nil {
						t.Fatal(err)
					}
					wire, err := adapter.Invoke(context.Background(), migrationEvent(t, mode, tc.method, target))
					if err != nil {
						t.Fatal(err)
					}
					var envelope struct {
						StatusCode        int
						Body              string
						IsBase64Encoded   bool
						Headers           map[string]string
						MultiValueHeaders map[string][]string
					}
					if err := json.Unmarshal(wire, &envelope); err != nil {
						t.Fatal(err)
					}
					if response.StatusCode != tc.status || envelope.StatusCode != tc.status || envelope.IsBase64Encoded || envelope.Body != string(body) {
						t.Fatalf("Direct/invoke response differs: direct=%d invoke=%d expected=%d", response.StatusCode, envelope.StatusCode, tc.status)
					}
					for _, name := range []string{"Content-Type", "Content-Crs", "Cache-Control", "Vary", "Content-Length", "Access-Control-Allow-Origin"} {
						actual := ""
						for key, value := range envelope.Headers {
							if strings.EqualFold(key, name) {
								actual = value
							}
						}
						for key, values := range envelope.MultiValueHeaders {
							if strings.EqualFold(key, name) {
								actual = strings.Join(values, ", ")
							}
						}
						if name == "Content-Length" && tc.method != "HEAD" {
							continue
						}
						if actual != response.Header.Get(name) {
							t.Fatalf("Header %s differs", name)
						}
					}
					if tc.ids != nil {
						var page struct{ Features []struct{ ID uint64 } }
						if err := json.Unmarshal(body, &page); err != nil {
							t.Fatal(err)
						}
						ids := []uint64{}
						for _, feature := range page.Features {
							ids = append(ids, feature.ID)
						}
						if !reflect.DeepEqual(ids, tc.ids) {
							t.Fatal("Static migrated membership differs")
						}
					}
					if tc.name == "xyz" || tc.name == "null" {
						var keys map[string]json.RawMessage
						if err := json.Unmarshal(body, &keys); err != nil {
							t.Fatal(err)
						}
						if _, present := keys["geometry"]; !present {
							t.Fatal("Geometry key absent")
						}
						var feature struct {
							ID         uint64
							Geometry   any
							Properties map[string]any
						}
						if err := json.Unmarshal(body, &feature); err != nil {
							t.Fatal(err)
						}
						if tc.name == "xyz" {
							literal := map[string]any{"type": "Point", "coordinates": []any{float64(15), float64(30), float64(23)}}
							if feature.ID != 10 || !reflect.DeepEqual(feature.Geometry, literal) || !reflect.DeepEqual(feature.Properties, map[string]any{"n": float64(1), "s": "height", "b": true}) {
								t.Fatal("Static XYZ feature differs")
							}
						} else if feature.ID != 40 || feature.Geometry != nil || !reflect.DeepEqual(feature.Properties, map[string]any{"n": nil, "s": nil, "b": nil}) {
							t.Fatal("Static NULL feature differs")
						}
					}
					if tc.name == "collections" {
						var catalog struct{ Collections []struct{ ID string } }
						if err := json.Unmarshal(body, &catalog); err != nil {
							t.Fatal(err)
						}
						ids := []string{}
						for _, collection := range catalog.Collections {
							ids = append(ids, collection.ID)
						}
						slices.Sort(ids)
						if !slices.Equal(ids, []string{"bare", "xy", "xyz"}) {
							t.Fatal("Explicit migrated catalog differs")
						}
					}
					if tc.name == "landing" {
						var landing struct{ Links []struct{ Href string } }
						if err := json.Unmarshal(body, &landing); err != nil {
							t.Fatal(err)
						}
						if len(landing.Links) == 0 {
							t.Fatal("Landing links absent")
						}
						for _, link := range landing.Links {
							if !strings.HasPrefix(link.Href, "https://migration.example/mapping/nested/features") {
								t.Fatal("Trusted public mapping/prefix differs")
							}
						}
					}
				})
			}
		})
	}
}
