package features

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexeydott/tegola/provider"
)

func openAPITestOptions() OpenAPIOptions {
	return OpenAPIOptions{Title: "Fixture", Version: "test", ServerURL: "https://example.test/stage/features", DefaultLimit: 3, MaxLimit: 17, HTML: true, MaxResponseBytes: 16 << 20, QueryTimeout: 30 * time.Second}
}

func openAPITestService(t *testing.T) *Service {
	t.Helper()
	xy, _ := ResolveCRS(CRS84)
	xyz, _ := ResolveCRS(CRS84h)
	xyCatalog, err := NewCollectionCRS(xy, provider.SpatialMetadata{Dimension: provider.DimensionXY})
	if err != nil {
		t.Fatal(err)
	}
	xyzCatalog, err := NewCollectionCRS(xyz, provider.SpatialMetadata{Dimension: provider.DimensionMixedXYXYZ, VerticalCRS: provider.CRS84h})
	if err != nil {
		t.Fatal(err)
	}
	return &Service{collections: map[string]resolvedCollection{
		"core":             {metadata: CollectionMetadata{ID: "core"}},
		"temporal":         {metadata: CollectionMetadata{ID: "temporal"}, temporal: provider.TemporalMapping{InstantField: "private_time"}},
		"empty-queryables": {metadata: CollectionMetadata{ID: "empty-queryables"}, queryablesAvailable: true},
		"xy":               {metadata: CollectionMetadata{ID: "xy"}, crsAvailable: true, crs: xyCatalog},
		"mixed.~":          {metadata: CollectionMetadata{ID: "mixed.~"}, crsAvailable: true, crs: xyzCatalog, queryablesAvailable: true},
	}}
}

func TestOpenAPICapabilityPaths(t *testing.T) {
	s := openAPITestService(t)
	doc, err := s.OpenAPI(openAPITestOptions())
	if err != nil {
		t.Fatal(err)
	}
	paths := doc["paths"].(map[string]any)
	if _, exists := paths["/collections/{collection}/items"]; exists {
		t.Fatal("generic capability union")
	}
	if _, exists := paths["/collections/core/queryables"]; exists {
		t.Fatal("unsupported queryables advertised")
	}
	if _, exists := paths["/collections/empty-queryables/queryables"]; !exists {
		t.Fatal("valid empty catalog lost")
	}
	for id, want := range map[string][]string{
		"core":             {"limit", "offset", "bbox", "datetime", "f"},
		"temporal":         {"limit", "offset", "bbox", "datetime", "f"},
		"empty-queryables": {"limit", "offset", "bbox", "datetime", "filter", "filter-lang", "f"},
		"xy":               {"limit", "offset", "bbox", "datetime", "bbox-crs", "crs", "f"},
		"mixed.~":          {"limit", "offset", "bbox", "datetime", "filter", "filter-lang", "bbox-crs", "crs", "f"},
	} {
		operation := paths["/collections/"+id+"/items"].(map[string]any)["get"].(map[string]any)
		var got []string
		for _, p := range operation["parameters"].([]any) {
			got = append(got, p.(map[string]any)["name"].(string))
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s params %v want %v", id, got, want)
		}
	}
	core := paths["/collections/core/items"].(map[string]any)["get"].(map[string]any)
	if !reflect.DeepEqual(core["x-tegola-unsupported-parameters"], []string{"filter", "filter-lang", "bbox-crs", "crs"}) {
		t.Fatal("error compatibility undocumented", core)
	}
	limit := core["parameters"].([]any)[0].(map[string]any)["schema"].(map[string]any)
	if limit["default"] != uint(3) || limit["maximum"] != uint(17) {
		t.Fatal(limit)
	}
	if _, clamped := limit["x-tegola-limit-clamping"]; clamped {
		t.Fatal("limit clamping still advertised", limit)
	}
	for _, id := range []string{"xy", "mixed.~"} {
		operation := paths["/collections/"+id+"/items"].(map[string]any)["get"].(map[string]any)
		for _, p := range operation["parameters"].([]any) {
			parameter := p.(map[string]any)
			if parameter["name"] == "crs" {
				schema := parameter["schema"].(map[string]any)
				want := s.collections[id].crs
				if !reflect.DeepEqual(schema["enum"], want.URIs()) || schema["default"] != want.DefaultURI() {
					t.Fatal(schema)
				}
			}
		}
	}
}

func TestOpenAPISchemasMethodsAndLocalReferences(t *testing.T) {
	doc, err := openAPITestService(t).OpenAPI(openAPITestOptions())
	if err != nil {
		t.Fatal(err)
	}
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	ids := map[string]bool{}
	for path, value := range doc["paths"].(map[string]any) {
		for method, value := range value.(map[string]any) {
			operation := value.(map[string]any)
			id := operation["operationId"].(string)
			if ids[id] {
				t.Fatal("duplicate operation", id)
			}
			ids[id] = true
			for suffix := path; strings.Contains(suffix, "{"); {
				_, remainder, _ := strings.Cut(suffix, "{")
				name, rest, closed := strings.Cut(remainder, "}")
				if !closed {
					t.Fatal("unclosed path placeholder", path)
				}
				found := false
				if parameters, ok := operation["parameters"].([]any); ok {
					for _, value := range parameters {
						parameter := value.(map[string]any)
						found = found || parameter["name"] == name && parameter["in"] == "path" && parameter["required"] == true
					}
				}
				if !found {
					t.Fatal("operation missing required placeholder", path, method, name)
				}
				suffix = rest
			}
			responses := operation["responses"].(map[string]any)
			if method == "options" {
				if strings.Contains(path, "{feature}") {
					parameters := operation["parameters"].([]any)
					if len(parameters) != 1 || parameters[0].(map[string]any)["name"] != "feature" || parameters[0].(map[string]any)["required"] != true {
						t.Fatal("OPTIONS missing required path parameter", path)
					}
				}
				continue
			}
			for _, status := range []string{"200", "400", "404", "405", "406", "408", "414", "431", "500", "501"} {
				response := responses[status].(map[string]any)
				if method == "head" && response["content"] != nil {
					t.Fatal(path, "HEAD declares body")
				}
			}
			if method == "get" {
				content := responses["200"].(map[string]any)["content"].(map[string]any)
				if content["text/html"] == nil {
					t.Fatal(path, "HTML missing")
				}
			}
		}
	}
	var walk func(any)
	walk = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if ref, ok := v["$ref"].(string); ok {
				name := strings.TrimPrefix(ref, "#/components/schemas/")
				if name == ref || schemas[name] == nil {
					t.Fatal("unresolved/nonlocal ref", ref)
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(doc)
	feature := schemas["Feature"].(map[string]any)["properties"].(map[string]any)
	if feature["properties"].(map[string]any)["additionalProperties"] != true {
		t.Fatal("properties wrongly closed to queryables")
	}
	if feature["id"].(map[string]any)["maximum"] != json.Number("18446744073709551615") {
		t.Fatal("uint64 narrowed")
	}
	raw, err := json.Marshal(doc)
	if err != nil || !bytes.Contains(raw, []byte(`"maximum":18446744073709551615`)) {
		t.Fatal("uint64 JSON changed", err)
	}
	if strings.Contains(string(raw), `"type":[`) {
		t.Fatal("Draft2020-12 union embedded into OAS3")
	}
}

func TestOpenAPIDetachedDeterministicConcurrent(t *testing.T) {
	s := openAPITestService(t)
	options := openAPITestOptions()
	baseline, _ := s.OpenAPI(options)
	want, _ := json.Marshal(baseline)
	baseline["paths"].(map[string]any)["/collections/xy/items"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any)["headers"] = nil
	baseline["components"].(map[string]any)["schemas"].(map[string]any)["Feature"] = nil
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			doc, err := s.OpenAPI(options)
			if err != nil {
				t.Error(err)
				return
			}
			raw, err := json.Marshal(doc)
			if err != nil || !bytes.Equal(raw, want) {
				t.Error("unstable/shared document", err)
			}
		}()
	}
	wg.Wait()
}

func TestOpenAPIRejectsInvalidOptionsAndPaths(t *testing.T) {
	s := openAPITestService(t)
	for _, invalid := range []string{"", "//evil.test", "https://user:pass@example.test", "https://example.test/{variable}", "https://example.test?q=1", "https://example.test#fragment", "file:///tmp/api", "relative"} {
		options := openAPITestOptions()
		options.ServerURL = invalid
		if _, err := s.OpenAPI(options); err == nil {
			t.Errorf("accepted URL %q", invalid)
		}
	}
	for _, change := range []func(*OpenAPIOptions){func(o *OpenAPIOptions) { o.DefaultLimit = 0 }, func(o *OpenAPIOptions) { o.MaxLimit = 1 }, func(o *OpenAPIOptions) { o.QueryTimeout = 0 }, func(o *OpenAPIOptions) { o.MaxResponseBytes = 1023 }, func(o *OpenAPIOptions) { o.Version = "" }} {
		options := openAPITestOptions()
		change(&options)
		if _, err := s.OpenAPI(options); err == nil {
			t.Fatal("accepted invalid options")
		}
	}
	if _, err := (*Service)(nil).OpenAPI(openAPITestOptions()); err == nil {
		t.Fatal("nil service accepted")
	}
	s.collections["bad"] = resolvedCollection{metadata: CollectionMetadata{ID: "bad/path"}}
	if _, err := s.OpenAPI(openAPITestOptions()); err == nil {
		t.Fatal("path injection accepted")
	}
}

func TestOpenAPIWriteContracts(t *testing.T) {
	options := openAPITestOptions()
	options.Write = OpenAPIWriteConfig{Enabled: true, Collections: map[string][]string{"core": {"create", "update", "delete"}}}
	doc, err := openAPITestService(t).OpenAPI(options)
	if err != nil {
		t.Fatal(err)
	}
	paths := doc["paths"].(map[string]any)
	items := paths["/collections/core/items"].(map[string]any)
	post := items["post"].(map[string]any)
	body := post["requestBody"].(map[string]any)["content"].(map[string]any)["application/geo+json"].(map[string]any)["schema"].(map[string]any)
	if body["$ref"] != "#/components/schemas/FeatureInput" {
		t.Fatal("create requires response Feature with server ID")
	}
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	required := schemas["FeatureInput"].(map[string]any)["required"].([]string)
	for _, name := range required {
		if name == "id" {
			t.Fatal("create must not require generated ID")
		}
	}
	item := paths["/collections/core/items/{feature}"].(map[string]any)
	patch := item["patch"].(map[string]any)
	content := patch["requestBody"].(map[string]any)["content"].(map[string]any)
	if content["application/json-patch+json"] == nil || content["application/merge-patch+json"] == nil {
		t.Fatal("missing supported patch media")
	}
	options.Write.Collections["core"] = []string{"update"}
	doc, err = openAPITestService(t).OpenAPI(options)
	if err != nil {
		t.Fatal(err)
	}
	if doc["paths"].(map[string]any)["/collections/core/schema"] == nil {
		t.Fatal("update-only collection needs mutation schema")
	}
}
