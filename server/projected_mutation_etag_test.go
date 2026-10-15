//go:build cgo

package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider/gpkg"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
)

func projectedMutationFixture(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "projected.gpkg")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := db.Exec(part4DDL); err != nil {
		t.Fatal(err)
	}
	raw, err := gpkg.NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]any{{"name": "sites", "tablename": "sites", "id_fieldname": "fid", "geometry_fieldname": "geom", "geometry_format": "wkb", "srid": 4326, "fields": []string{"name", "seats"}}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*gpkg.Provider)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	})
	layers, err := p.Layers()
	if err != nil {
		t.Fatal(err)
	}
	service, err := features.NewService([]features.CollectionSource{{ID: "sites", Layer: layers[0], Querier: p}})
	if err != nil {
		t.Fatal(err)
	}
	return part4Router(t, part4API(t, service)), db
}

func TestProjectedMutationETagUsesSelectedRepresentation(t *testing.T) {
	for _, crs := range []string{features.CRS84, "http://www.opengis.net/def/crs/EPSG/0/3857", "http://www.opengis.net/def/crs/EPSG/0/4326"} {
		for _, format := range []string{"json", "html"} {
			t.Run(crs+"/"+format, func(t *testing.T) {
				router, db := projectedMutationFixture(t)
				created := doRequest(t, router, http.MethodPost, "/features/collections/sites/items", `{"type":"Feature","geometry":{"type":"Point","coordinates":[37.6,55.7]},"properties":{"name":"before","seats":1}}`, map[string]string{"Content-Type": mediaGeoJSON})
				if created.Code != 201 {
					t.Fatalf("create=%d %s", created.Code, created.Body.String())
				}
				target := "/features/collections/sites/items/1?" + url.Values{"crs": {crs}, "f": {format}}.Encode()
				get := doRequest(t, router, http.MethodGet, target, "", nil)
				if get.Code != 200 {
					t.Fatalf("get=%d %s", get.Code, get.Body.String())
				}
				var before, after []byte
				if err := db.QueryRow("SELECT geom FROM sites WHERE fid=1").Scan(&before); err != nil {
					t.Fatal(err)
				}
				patched := doRequest(t, router, http.MethodPatch, target, `{"properties":{"name":"patched"}}`, map[string]string{"Content-Type": mediaMergePatch, "If-Match": get.Header().Get("ETag")})
				if patched.Code != 200 {
					t.Fatalf("patch=%d %s", patched.Code, patched.Body.String())
				}
				if err := db.QueryRow("SELECT geom FROM sites WHERE fid=1").Scan(&after); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(before, after) {
					t.Fatalf("attribute patch rewrote geometry %x -> %x", before, after)
				}
				fresh := doRequest(t, router, http.MethodGet, target, "", nil)
				if fresh.Header().Get("ETag") != patched.Header().Get("ETag") {
					t.Fatal("mutation ETag disagrees with selected GET")
				}
				stale := doRequest(t, router, http.MethodPatch, target, `{"properties":{"name":"stale"}}`, map[string]string{"Content-Type": mediaMergePatch, "If-Match": get.Header().Get("ETag")})
				if stale.Code != 412 {
					t.Fatalf("stale=%d", stale.Code)
				}
				for _, method := range []string{http.MethodPut, http.MethodDelete} {
					stale := doRequest(t, router, method, target,
						`{"type":"Feature","geometry":null,"properties":{"name":"stale"}}`,
						map[string]string{"Content-Type": mediaGeoJSON, "If-Match": get.Header().Get("ETag")})
					if stale.Code != http.StatusPreconditionFailed {
						t.Fatalf("stale %s=%d", method, stale.Code)
					}
				}
				replaced := doRequest(t, router, http.MethodPut, target, `{"type":"Feature","geometry":{"type":"Point","coordinates":[38,56]},"properties":{"name":"replaced","seats":2}}`, map[string]string{"Content-Type": mediaGeoJSON, "Content-Crs": "<" + features.CRS84 + ">", "If-Match": fresh.Header().Get("ETag")})
				if replaced.Code != 200 {
					t.Fatalf("replace=%d %s", replaced.Code, replaced.Body.String())
				}
				canonical := doRequest(t, router, http.MethodGet, "/features/collections/sites/items/1", "", nil)
				var feature features.Feature
				if err := json.Unmarshal(canonical.Body.Bytes(), &feature); err != nil {
					t.Fatal(err)
				}
				if !jsonEqual(jsonRawToMap(feature.Geometry), jsonRawToMap(json.RawMessage(`{"type":"Point","coordinates":[38,56]}`))) {
					t.Fatalf("output CRS changed input interpretation: %s", feature.Geometry)
				}
				fresh = doRequest(t, router, http.MethodGet, target, "", nil)
				deleted := doRequest(t, router, http.MethodDelete, target, "", map[string]string{"If-Match": fresh.Header().Get("ETag")})
				if deleted.Code != 204 {
					t.Fatalf("delete=%d %s", deleted.Code, deleted.Body.String())
				}
			})
		}
	}
}

func TestProjectedMutationRejectsWrongRepresentationAndInvalidSelectors(t *testing.T) {
	router, db := projectedMutationFixture(t)
	created := doRequest(t, router, http.MethodPost, "/features/collections/sites/items", `{"type":"Feature","geometry":{"type":"Point","coordinates":[37.6,55.7]},"properties":{"name":"before","seats":1}}`, map[string]string{"Content-Type": mediaGeoJSON})
	if created.Code != 201 {
		t.Fatalf("create=%d %s", created.Code, created.Body.String())
	}
	base := "/features/collections/sites/items/1"
	projected := base + "?" + url.Values{"crs": {"http://www.opengis.net/def/crs/EPSG/0/3857"}}.Encode()
	get := doRequest(t, router, http.MethodGet, projected, "", nil)
	patch := `{"properties":{"name":"invalid"}}`
	// A valid revision with a hash for a different representation is not a match.
	wrong := doRequest(t, router, http.MethodPatch, base, patch, map[string]string{"Content-Type": mediaMergePatch, "If-Match": get.Header().Get("ETag")})
	if wrong.Code != 412 {
		t.Fatalf("cross-representation tag accepted: %d", wrong.Code)
	}
	for _, query := range []string{"?crs=unknown", "?crs=bad&crs=worse", "?crs=%xx", "?unexpected=1"} {
		rec := doRequest(t, router, http.MethodPatch, base+query, patch, map[string]string{"Content-Type": mediaMergePatch})
		if rec.Code != 400 {
			t.Fatalf("selector %s returned%d: %s", query, rec.Code, rec.Body.String())
		}
	}
	unsupported := doRequest(t, router, http.MethodPut, projected, `{"type":"Feature","geometry":{"type":"Point","coordinates":[1000,2000]},"properties":{"name":"invalid"}}`, map[string]string{"Content-Type": mediaGeoJSON, "Content-Crs": "<http://www.opengis.net/def/crs/EPSG/0/3857>", "If-Match": get.Header().Get("ETag")})
	if unsupported.Code != 400 {
		t.Fatalf("unsupported input CRS=%d", unsupported.Code)
	}
	var name string
	if err := db.QueryRow("SELECT name FROM sites WHERE fid=1").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "before" {
		t.Fatalf("rejected request committed: %s", name)
	}
	// Accept-negotiated HTML follows the same hash path as explicit f=html.
	html := doRequest(t, router, http.MethodGet, projected, "", map[string]string{"Accept": "text/html"})
	accepted := doRequest(t, router, http.MethodPatch, projected, `{"properties":{"name":"accepted"}}`, map[string]string{"Accept": "text/html", "Content-Type": mediaMergePatch, "If-Match": html.Header().Get("ETag")})
	if accepted.Code != 200 {
		t.Fatalf("HTML Accept precondition=%d: %s", accepted.Code, accepted.Body.String())
	}
	current := doRequest(t, router, http.MethodGet, projected, "", map[string]string{"Accept": "text/html"})
	if accepted.Header().Get("ETag") != current.Header().Get("ETag") {
		t.Fatal("Accept HTML mutation tag differs from GET")
	}
}
