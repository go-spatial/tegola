//go:build cgo

package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/feature"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/gpkg"
)

const part4DDL = `CREATE TABLE sites (
	fid INTEGER PRIMARY KEY,
	geom BLOB,
	name TEXT,
	seats INTEGER
)`

func part4Service(t *testing.T) (*features.Service, *gpkg.Provider) {
	t.Helper()
	return part4ServiceDDL(t, part4DDL)
}

func part4ServiceDDL(t *testing.T, ddl string) (*features.Service, *gpkg.Provider) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "part4.gpkg")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(ddl); err != nil {
		t.Fatalf("ddl: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	conf := dict.Dict{
		"filepath": path,
		"layers": []map[string]interface{}{
			{
				"name":               "sites",
				"tablename":          "sites",
				"id_fieldname":       "fid",
				"geometry_fieldname": "geom",
				"geometry_format":    "wkb",
				"srid":               4326,
				"fields":             []string{"name", "seats"},
			},
		},
	}
	tiler, err := gpkg.NewTileProvider(conf, nil)
	if err != nil {
		t.Fatalf("NewTileProvider: %v", err)
	}
	t.Cleanup(gpkg.Cleanup)
	gp, ok := tiler.(*gpkg.Provider)
	if !ok {
		t.Fatalf("not a *gpkg.Provider")
	}
	layers, err := gp.Layers()
	if err != nil {
		t.Fatalf("Layers: %v", err)
	}
	var layer provider.LayerInfo
	for _, l := range layers {
		if l.Name() == "sites" {
			layer = l
		}
	}
	if layer == nil {
		t.Fatalf("layer not found")
	}
	service, err := features.NewService([]features.CollectionSource{
		{ID: "sites", Title: "Sites", Layer: layer, Querier: gp},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return service, gp
}

func part4API(t *testing.T, service *features.Service) *FeatureAPI {
	t.Helper()
	writeCfg := config.FeaturesWriteConfig{Enabled: true, AuthMode: "dev"}
	writeCfg.Collections = []config.WriteCollectionConfig{
		{ID: "sites", Operations: []string{"create", "replace", "update", "delete"}},
	}
	api, err := NewFeatureAPI(service, FeatureAPIConfig{
		BasePath:     "/features",
		DefaultLimit: 100,
		MaxLimit:     10000,
		Title:        "t",
		Write:        writeCfg,
	})
	if err != nil {
		t.Fatalf("NewFeatureAPI: %v", err)
	}
	return api
}

func part4Router(t *testing.T, api *FeatureAPI) http.Handler {
	t.Helper()
	preserveDiscoveryGlobals(t)
	router, err := NewRouterWithOptions(nil, RouterOptions{Features: api})
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	return router
}

func doRequest(t *testing.T, router http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *strings.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	} else {
		rdr = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, rdr)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestPart4CRUD(t *testing.T) {
	service, _ := part4Service(t)
	router := part4Router(t, part4API(t, service))
	ct := map[string]string{"Content-Type": "application/geo+json"}

	// Create.
	createBody := `{"type":"Feature","geometry":{"type":"Point","coordinates":[10,20]},"properties":{"name":"hq","seats":42}}`
	rec := doRequest(t, router, http.MethodPost, "/features/collections/sites/items", createBody, ct)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, body: %s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if loc == "" {
		t.Fatal("missing Location header")
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag header")
	}
	var created map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id := int(created["id"].(float64))

	// GET the item by parsed id path.
	getPath := "/features/collections/sites/items/" + strconv.Itoa(id)
	rec = doRequest(t, router, http.MethodGet, getPath, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d", rec.Code)
	}
	getETag := rec.Header().Get("ETag")
	if getETag == "" {
		t.Fatal("GET missing ETag")
	}

	// PUT with stale If-Match -> 412.
	rec = doRequest(t, router, http.MethodPut, getPath,
		`{"type":"Feature","geometry":{"type":"Point","coordinates":[11,21]},"properties":{"name":"hq2","seats":43}}`,
		map[string]string{"Content-Type": "application/geo+json", "If-Match": `"stale"`})
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("PUT stale If-Match status = %d, want 412", rec.Code)
	}

	// PUT with correct If-Match -> 200.
	rec = doRequest(t, router, http.MethodPut, getPath,
		`{"type":"Feature","geometry":{"type":"Point","coordinates":[11,21]},"properties":{"name":"hq2","seats":43}}`,
		map[string]string{"Content-Type": "application/geo+json", "If-Match": getETag})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body: %s", rec.Code, rec.Body.String())
	}
	newETag := rec.Header().Get("ETag")
	if newETag == "" || newETag == getETag {
		t.Fatal("PUT should return a new ETag")
	}

	// PATCH merge-patch: change seats only.
	rec = doRequest(t, router, http.MethodPatch, getPath, `{"properties":{"seats":100}}`,
		map[string]string{"Content-Type": "application/merge-patch+json", "If-Match": newETag})
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d, body: %s", rec.Code, rec.Body.String())
	}
	var patched map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	props := patched["properties"].(map[string]interface{})
	if props["seats"].(float64) != 100 {
		t.Fatalf("seats = %v, want 100", props["seats"])
	}
	if props["name"].(string) != "hq2" {
		t.Fatalf("name = %v, want hq2 (preserved)", props["name"])
	}
	patchETag := rec.Header().Get("ETag")

	// DELETE with If-Match -> 204.
	rec = doRequest(t, router, http.MethodDelete, getPath, "",
		map[string]string{"If-Match": patchETag})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, body: %s", rec.Code, rec.Body.String())
	}

	// GET after delete -> 404.
	rec = doRequest(t, router, http.MethodGet, getPath, "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET after delete status = %d, want 404", rec.Code)
	}
}

func TestPart4OPTIONS(t *testing.T) {
	service, _ := part4Service(t)
	router := part4Router(t, part4API(t, service))

	rec := doRequest(t, router, http.MethodOptions, "/features/collections/sites/items", "", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d", rec.Code)
	}
	allow := rec.Header().Get("Allow")
	if !strings.Contains(allow, "POST") {
		t.Fatalf("Allow = %q, want POST", allow)
	}

	rec = doRequest(t, router, http.MethodOptions, "/features/collections/sites/items/1", "", nil)
	allow = rec.Header().Get("Allow")
	for _, m := range []string{"PUT", "PATCH", "DELETE"} {
		if !strings.Contains(allow, m) {
			t.Fatalf("Allow = %q, want %s", allow, m)
		}
	}
	if ap := rec.Header().Get("Accept-Patch"); ap != "application/merge-patch+json, application/json-patch+json" {
		t.Fatalf("Accept-Patch = %q", ap)
	}
}

func TestPart4WriteDisabled(t *testing.T) {
	service, _ := part4Service(t)
	// No write config: all mutations must be unavailable.
	api, err := NewFeatureAPI(service, FeatureAPIConfig{
		BasePath: "/features", DefaultLimit: 100, MaxLimit: 10000, Title: "t",
	})
	if err != nil {
		t.Fatal(err)
	}
	router := part4Router(t, api)
	rec := doRequest(t, router, http.MethodPost, "/features/collections/sites/items",
		`{"type":"Feature","geometry":null,"properties":{}}`,
		map[string]string{"Content-Type": "application/geo+json"})
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST with write disabled status = %d, want 404/405", rec.Code)
	}
}

func TestPart4DuplicateKeysRejected(t *testing.T) {
	service, _ := part4Service(t)
	router := part4Router(t, part4API(t, service))
	ct := map[string]string{"Content-Type": "application/geo+json"}

	// Duplicate top-level key.
	rec := doRequest(t, router, http.MethodPost, "/features/collections/sites/items",
		`{"type":"Feature","type":"Feature","geometry":{"type":"Point","coordinates":[10,20]},"properties":{"name":"dup"}}`, ct)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("duplicate top-level key status = %d, want 400", rec.Code)
	}

	// Duplicate nested key in properties.
	rec = doRequest(t, router, http.MethodPost, "/features/collections/sites/items",
		`{"type":"Feature","geometry":{"type":"Point","coordinates":[10,20]},"properties":{"name":"a","name":"b"}}`, ct)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("duplicate nested key status = %d, want 400", rec.Code)
	}

	// Duplicate key in geometry object.
	rec = doRequest(t, router, http.MethodPost, "/features/collections/sites/items",
		`{"type":"Feature","geometry":{"type":"Point","type":"Point","coordinates":[10,20]},"properties":{}}`, ct)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("duplicate geometry key status = %d, want 400", rec.Code)
	}
}

func TestPart4ETagRoundTrip(t *testing.T) {
	service, _ := part4Service(t)
	router := part4Router(t, part4API(t, service))
	ct := map[string]string{"Content-Type": "application/geo+json"}

	// Create.
	rec := doRequest(t, router, http.MethodPost, "/features/collections/sites/items",
		`{"type":"Feature","geometry":{"type":"Point","coordinates":[10,20]},"properties":{"name":"etag"}}`, ct)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d", rec.Code)
	}
	var created map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id := int(created["id"].(float64))
	getPath := "/features/collections/sites/items/" + strconv.Itoa(id)

	// GET ETag must be accepted by PUT If-Match (same validator).
	rec = doRequest(t, router, http.MethodGet, getPath, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d", rec.Code)
	}
	getETag := rec.Header().Get("ETag")
	rec = doRequest(t, router, http.MethodPut, getPath,
		`{"type":"Feature","geometry":{"type":"Point","coordinates":[11,21]},"properties":{"name":"etag2"}}`,
		map[string]string{"Content-Type": "application/geo+json", "If-Match": getETag})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT with GET ETag status = %d, want 200 (validator mismatch)", rec.Code)
	}
	putETag := rec.Header().Get("ETag")

	// PUT response ETag must be accepted by PATCH If-Match.
	rec = doRequest(t, router, http.MethodPatch, getPath, `{"properties":{"name":"etag3"}}`,
		map[string]string{"Content-Type": "application/merge-patch+json", "If-Match": putETag})
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH with PUT ETag status = %d, want 200 (validator mismatch)", rec.Code)
	}

	// ETag must be the strong validator of the exact GET bytes.
	rec = doRequest(t, router, http.MethodGet, getPath, "", nil)
	body := rec.Body.Bytes()
	if got, want := rec.Header().Get("ETag"), revisionETag(body, revisionFromETag(rec.Header().Get("ETag"))); got != want {
		t.Fatalf("GET ETag = %s, want strong validator of exact bytes %s", got, want)
	}
}

func TestPart4SchemaEndpoint(t *testing.T) {
	service, _ := part4Service(t)
	router := part4Router(t, part4API(t, service))

	rec := doRequest(t, router, http.MethodGet, "/features/collections/sites/schema", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET schema status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/schema+json" {
		t.Fatalf("schema Content-Type = %q", ct)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &schema); err != nil {
		t.Fatal(err)
	}
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("schema missing properties")
	}
	if _, ok := props["properties"]; !ok {
		t.Fatal("schema missing properties.properties")
	}
}

func TestPart4OpenAPIWriteOps(t *testing.T) {
	service, _ := part4Service(t)
	api := part4API(t, service)
	router := part4Router(t, api)

	rec := doRequest(t, router, http.MethodGet, "/features/api", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET api status = %d", rec.Code)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	paths, ok := doc["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("no paths in OpenAPI")
	}
	// POST on /collections/sites/items
	itemsPath, ok := paths["/collections/sites/items"].(map[string]interface{})
	if !ok {
		t.Fatal("no /collections/sites/items path")
	}
	if _, ok := itemsPath["post"]; !ok {
		t.Fatal("POST not documented for writable collection")
	}
	if _, ok := itemsPath["get"]; !ok {
		t.Fatal("GET lost after write-ops merge")
	}
	// PUT/PATCH/DELETE on item path
	itemPath, ok := paths["/collections/sites/items/{feature}"].(map[string]interface{})
	if !ok {
		t.Fatal("no item path")
	}
	for _, m := range []string{"put", "patch", "delete", "get"} {
		if _, ok := itemPath[m]; !ok {
			t.Fatalf("%s not documented on item path", m)
		}
	}
	// /schema path
	if _, ok := paths["/collections/sites/schema"]; !ok {
		t.Fatal("no /schema path in OpenAPI")
	}
}

func TestPart4RequireIfMatch428(t *testing.T) {
	service, _ := part4Service(t)
	writeCfg := config.FeaturesWriteConfig{Enabled: true, RequireIfMatch: true, AuthMode: "dev"}
	writeCfg.Collections = []config.WriteCollectionConfig{
		{ID: "sites", Operations: []string{"create", "replace", "update", "delete"}},
	}
	api, err := NewFeatureAPI(service, FeatureAPIConfig{
		BasePath:     "/features",
		DefaultLimit: 100,
		MaxLimit:     10000,
		Title:        "t",
		Write:        writeCfg,
	})
	if err != nil {
		t.Fatalf("NewFeatureAPI: %v", err)
	}
	router := part4Router(t, api)
	ct := map[string]string{"Content-Type": "application/geo+json"}

	// Create (no If-Match needed for POST).
	rec := doRequest(t, router, http.MethodPost, "/features/collections/sites/items",
		`{"type":"Feature","geometry":{"type":"Point","coordinates":[10,20]},"properties":{"name":"r428"}}`, ct)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d", rec.Code)
	}
	var created map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id := int(created["id"].(float64))
	getPath := "/features/collections/sites/items/" + strconv.Itoa(id)

	// PUT without If-Match -> 428.
	rec = doRequest(t, router, http.MethodPut, getPath,
		`{"type":"Feature","geometry":{"type":"Point","coordinates":[11,21]},"properties":{"name":"r428b"}}`, ct)
	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("PUT without If-Match status = %d, want 428", rec.Code)
	}

	// PUT with If-Match -> 200.
	rec = doRequest(t, router, http.MethodGet, getPath, "", nil)
	etag := rec.Header().Get("ETag")
	rec = doRequest(t, router, http.MethodPut, getPath,
		`{"type":"Feature","geometry":{"type":"Point","coordinates":[11,21]},"properties":{"name":"r428b"}}`,
		map[string]string{"Content-Type": "application/geo+json", "If-Match": etag})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT with If-Match status = %d, want 200", rec.Code)
	}

	// DELETE without If-Match -> 428.
	rec = doRequest(t, router, http.MethodDelete, getPath, "", nil)
	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("DELETE without If-Match status = %d, want 428", rec.Code)
	}
}

func TestReviewPatchHTTPPreservesStateOnMalformedInput(t *testing.T) {
	service, _ := part4Service(t)
	router := part4Router(t, part4API(t, service))
	created := doRequest(t, router, http.MethodPost, "/features/collections/sites/items", `{"type":"Feature","geometry":{"type":"Point","coordinates":[10,20]},"properties":{"name":"before","seats":42}}`, map[string]string{"Content-Type": mediaGeoJSON})
	if created.Code != 201 {
		t.Fatalf("create %d: %s", created.Code, created.Body.String())
	}
	locationURL, err := url.Parse(created.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	location := locationURL.RequestURI()
	for _, body := range []string{`{"properties":{"name":"bad","name":"worse"}}`, `{"properties":{"name":"bad"}} {}`, `{"geometry":3,"properties":{"name":"bad"}}`, `{"id":999,"properties":{"name":"bad"}}`, `{"properties":3}`} {
		patched := doRequest(t, router, http.MethodPatch, location, body, map[string]string{"Content-Type": mediaMergePatch})
		if patched.Code != 400 {
			t.Fatalf("patch %s = %d: %s", body, patched.Code, patched.Body.String())
		}
		read := doRequest(t, router, http.MethodGet, location, "", nil)
		if !strings.Contains(read.Body.String(), `"name":"before"`) || !strings.Contains(read.Body.String(), `"coordinates":[10,20]`) {
			t.Fatalf("invalid patch changed data: %s", read.Body.String())
		}
	}
	clear := doRequest(t, router, http.MethodPatch, location, `{"geometry":null}`, map[string]string{"Content-Type": mediaMergePatch})
	if clear.Code != 200 || !strings.Contains(clear.Body.String(), `"geometry":null`) {
		t.Fatalf("clear %d: %s", clear.Code, clear.Body.String())
	}
}

func TestReviewMutationRejectsUnsupportedCRS(t *testing.T) {
	service, _ := part4Service(t)
	router := part4Router(t, part4API(t, service))
	rec := doRequest(t, router, http.MethodPost, "/features/collections/sites/items", `{"type":"Feature","geometry":{"type":"Point","coordinates":[10,20]},"properties":{"name":"bad"}}`, map[string]string{"Content-Type": mediaGeoJSON, "Content-Crs": "<http://www.opengis.net/def/crs/EPSG/0/3857>"})
	if rec.Code != 400 {
		t.Fatalf("unsupported CRS accepted: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPR2NonNullableGeometrySchemaAndInput(t *testing.T) {
	service, _ := part4ServiceDDL(t, strings.Replace(part4DDL, "geom BLOB", "geom BLOB NOT NULL", 1))
	router := part4Router(t, part4API(t, service))
	schema := doRequest(t, router, http.MethodGet, "/features/collections/sites/schema", "", nil)
	if schema.Code != 200 {
		t.Fatalf("schema %d: %s", schema.Code, schema.Body.String())
	}
	var document map[string]any
	if err := json.Unmarshal(schema.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	geometry := document["properties"].(map[string]any)["geometry"].(map[string]any)
	if geometry["type"] != "object" {
		t.Fatalf("NOT NULL geometry advertised nullable: %v", geometry)
	}
	rec := doRequest(t, router, http.MethodPost, "/features/collections/sites/items", `{"type":"Feature","geometry":null,"properties":{"name":"bad"}}`, map[string]string{"Content-Type": mediaGeoJSON})
	if rec.Code != 400 {
		t.Fatalf("explicit null must fail validation400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPR2ExplicitNullOnDefaultedNotNullProperty(t *testing.T) {
	service, _ := part4ServiceDDL(t, strings.Replace(part4DDL, "seats INTEGER", "seats INTEGER NOT NULL DEFAULT 7", 1))
	router := part4Router(t, part4API(t, service))
	for _, body := range []string{
		`{"type":"Feature","geometry":null,"properties":{"name":"bad","seats":null}}`,
		`{"type":"Feature","geometry":null,"properties":{"name":"good"}}`,
	} {
		rec := doRequest(t, router, http.MethodPost, "/features/collections/sites/items", body, map[string]string{"Content-Type": mediaGeoJSON})
		if strings.Contains(body, `"seats":null`) {
			if rec.Code != 400 {
				t.Fatalf("explicit null got %d: %s", rec.Code, rec.Body.String())
			}
		} else if rec.Code != 201 || !strings.Contains(rec.Body.String(), `"seats":7`) {
			t.Fatalf("omission lost default %d: %s", rec.Code, rec.Body.String())
		}
	}
}

// pr2Authenticator represents an embedding application's verified identity adapter.
type pr2Authenticator struct{}

func (pr2Authenticator) Principal(*http.Request) feature.Principal {
	return feature.Principal{ID: "verified-editor"}
}

func TestPR2EmbeddedProductionAuthenticatorRemainsSupported(t *testing.T) {
	service, _ := part4Service(t)
	api := part4API(t, service)
	api.cfg.Write.AuthMode = "production"
	api.cfg.Authenticator = pr2Authenticator{}
	router := part4Router(t, api)
	rec := doRequest(t, router, http.MethodPost, "/features/collections/sites/items", `{"type":"Feature","geometry":null,"properties":{"name":"verified"}}`, map[string]string{"Content-Type": mediaGeoJSON})
	if rec.Code != 201 {
		t.Fatalf("embedded authenticator rejected %d: %s", rec.Code, rec.Body.String())
	}
}
