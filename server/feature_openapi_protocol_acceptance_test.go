package server

import (
	"context"
	"encoding/json"
	"math"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
)

type phase09FullLayer struct{ filterAcceptanceLayer }

func (phase09FullLayer) TemporalMapping() (provider.TemporalMapping, error) {
	return provider.TemporalMapping{InstantField: "at"}, nil
}
func (phase09FullLayer) FeatureCRSDefinition() (provider.FeatureCRSDefinition, error) {
	return provider.FeatureCRSDefinition{HorizontalSRID: 4326, Definition: "+proj=longlat +datum=WGS84", CanonicalAuthority: "EPSG", CanonicalCode: "4326", Spatial: provider.SpatialMetadata{Dimension: provider.DimensionXY}}, nil
}

type phase09EmptyLayer struct{ protocolLayer }

func (phase09EmptyLayer) FeatureQueryables() (provider.FeatureQueryables, error) {
	return provider.NewFeatureQueryables(nil)
}

// A translation/transport observer, never a database predicate oracle.
type phase09Query struct {
	calls atomic.Int64
	text  string
}

func (p *phase09Query) QueryFeatures(ctx context.Context, _ string, q provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	p.calls.Add(1)
	if err := ctx.Err(); err != nil {
		return provider.FeatureQueryResult{}, err
	}
	n := uint64(1)
	result := provider.FeatureQueryResult{NumberMatched: &n}
	if q.Offset != 0 {
		return result, nil
	}
	f := &provider.Feature{ID: math.MaxUint64, SRID: 4326, Geometry: geom.Point{15, 30}, Tags: map[string]any{"s": p.text, "n": nil, "b": false, "links": []any{map[string]any{"href": "https://untrusted.invalid/property-link", "rel": "item"}}}}
	if err := fn(f); err != nil {
		return result, err
	}
	result.NumberReturned = 1
	return result, nil
}

func phase09Server(t *testing.T, sources []features.CollectionSource, cfg FeatureAPIConfig, prefix string) *httptest.Server {
	t.Helper()
	oldPrefix, oldHost := URIPrefix, HostName
	URIPrefix, HostName = prefix, nil
	t.Cleanup(func() { URIPrefix, HostName = oldPrefix, oldHost })
	if cfg.BasePath == "" {
		cfg.BasePath = "/features"
	}
	if cfg.DefaultLimit == 0 {
		cfg.DefaultLimit = 1
	}
	if cfg.MaxLimit == 0 {
		cfg.MaxLimit = 2
	}
	svc, err := features.NewService(sources)
	if err != nil {
		t.Fatal(err)
	}
	api, err := NewFeatureAPI(svc, cfg)
	if err != nil {
		t.Fatal(err)
	}
	router, err := NewRouterWithOptions(nil, RouterOptions{Features: api})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv
}

func phase09Object(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected object, got %T", v)
	}
	return m
}

func TestFeatureOpenAPIProtocolAcceptanceCapabilities(t *testing.T) {
	p := &phase09Query{}
	srv := phase09Server(t, []features.CollectionSource{{ID: "core", Layer: protocolLayer{}, Querier: p}, {ID: "full", Layer: phase09FullLayer{}, Querier: p}, {ID: "empty", Layer: phase09EmptyLayer{}, Querier: p}}, FeatureAPIConfig{DefaultLimit: 3, MaxLimit: 7}, "/proxy/nested")
	path := "/proxy/nested/features/api"
	r := protocolRequest(t, srv, "GET", path)
	if r.status != 200 || r.header.Get("Content-Type") != "application/vnd.oai.openapi+json;version=3.0" {
		t.Fatalf("API status/media %d %s", r.status, r.header.Get("Content-Type"))
	}
	var doc map[string]any
	if err := json.Unmarshal(r.body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["openapi"] != "3.0.3" {
		t.Fatal("unexpected API dialect")
	}
	paths := phase09Object(t, doc["paths"])
	seen := map[string]bool{}
	for resource, raw := range paths {
		if strings.Contains(resource, "{collection}") {
			t.Fatal("generic path leaks collection capability union")
		}
		for _, method := range []string{"get", "head", "options"} {
			op := phase09Object(t, phase09Object(t, raw)[method])
			id, ok := op["operationId"].(string)
			if !ok || id == "" || seen[id] {
				t.Fatal("operation ID missing or duplicated")
			}
			seen[id] = true
			if strings.Contains(resource, "{feature}") {
				found := false
				for _, raw := range op["parameters"].([]any) {
					p := phase09Object(t, raw)
					if p["name"] == "feature" && p["in"] == "path" && p["required"] == true {
						found = true
					}
				}
				if !found {
					t.Fatalf("required path parameter missing from%s%s", method, resource)
				}
			}
		}
	}
	for _, id := range []string{"core", "full", "empty"} {
		_, exists := paths["/collections/"+id+"/queryables"]
		if exists != (id != "core") {
			t.Fatalf("wrong Queryables publication for %s", id)
		}
		op := phase09Object(t, phase09Object(t, paths["/collections/"+id+"/items"])["get"])
		params := map[string]map[string]any{}
		for _, raw := range op["parameters"].([]any) {
			v := phase09Object(t, raw)
			params[v["name"].(string)] = v
		}
		for _, name := range []string{"limit", "offset", "bbox", "f"} {
			if params[name] == nil {
				t.Fatalf("missing %s for%s", name, id)
			}
		}
		for _, name := range []string{"filter", "filter-lang"} {
			if (params[name] != nil) != (id != "core") {
				t.Fatalf("filter capability leak %s", id)
			}
		}
		if params["datetime"] == nil {
			t.Fatalf("missing Core datetime for %s", id)
		}
		for _, name := range []string{"crs", "bbox-crs"} {
			if (params[name] != nil) != (id == "full") {
				t.Fatalf("optional capability leak %s/%s", id, name)
			}
		}
		limit := phase09Object(t, params["limit"]["schema"])
		if limit["default"] != float64(3) || limit["maximum"] != float64(7) {
			t.Fatal("configured limits lost")
		}
	}
	if p.calls.Load() != 0 {
		t.Fatal("discovery performed provider I/O")
	}
	again := protocolRequest(t, srv, "GET", path)
	if string(r.body) != string(again.body) {
		t.Fatal("same frozen publication API changed")
	}
	head := protocolRequest(t, srv, "HEAD", path)
	if head.status != r.status || len(head.body) != 0 || head.header.Get("Content-Length") != r.header.Get("Content-Length") {
		t.Fatal("API HEAD parity broken")
	}
	servers := doc["servers"].([]any)
	url := phase09Object(t, servers[0])["url"].(string)
	if url != srv.URL+"/proxy/nested/features" {
		t.Fatalf("nested prefix duplicated/lost: %s", url)
	}
}
