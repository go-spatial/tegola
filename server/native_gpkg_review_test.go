//go:build cgo

package server

import (
	"bytes"
	"database/sql"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider/gpkg"
)

// This acceptance test uses an independently generated GDAL fixture,
// always copied into a disposable directory before the HTTP server opens it.
func TestNativeGeoPackageHTTPPreservation(t *testing.T) {
	source := os.Getenv("TEGOLA_REVIEW_GPKG")
	if source == "" {
		source = filepath.Join("..", "testdata", "wfs", "editing-fixture.gpkg")
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "editing.gpkg")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	untouched := make(map[string][]byte)
	for _, table := range []string{"areas", "routes", "sites_xyz"} {
		var geometry []byte
		if err := db.QueryRow("SELECT geom FROM " + table + " WHERE fid=1").Scan(&geometry); err != nil {
			t.Fatal(err)
		}
		untouched[table] = geometry
	}
	var before []byte
	if err := db.QueryRow("SELECT geom FROM sites WHERE fid=1").Scan(&before); err != nil {
		t.Fatal(err)
	}
	p, err := gpkg.NewTileProvider(dict.Dict{
		"filepath": path,
		"layers": []map[string]interface{}{{
			"name": "sites", "tablename": "sites", "id_fieldname": "fid",
			"geometry_fieldname": "geom", "srid": 4326,
			"fields": []string{"fixture_key", "tenant_id", "name", "note", "amount_text", "counter", "server_tag"},
		}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(gpkg.Cleanup)
	gp := p.(*gpkg.Provider)
	layers, err := gp.Layers()
	if err != nil {
		t.Fatal(err)
	}
	service, err := features.NewService([]features.CollectionSource{{ID: "sites", Title: "Sites", Layer: layers[0], Querier: gp}})
	if err != nil {
		t.Fatal(err)
	}
	api := part4API(t, service)
	preserveDiscoveryGlobals(t)
	router, err := NewRouterWithOptions(nil, RouterOptions{
		Features: api,
		WFS:      &WFSHandler{Service: service, Config: config.WFSConfig{Enabled: true}.Resolved(), WriteConfig: api.cfg.Write},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(router)
	defer srv.Close()
	request := func(method, body, etag string, want int) (string, []byte) {
		t.Helper()
		req, err := http.NewRequest(method, srv.URL+"/features/collections/sites/items/1", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/merge-patch+json")
		if method == "PUT" {
			req.Header.Set("Content-Type", "application/geo+json")
		}
		if etag != "" {
			req.Header.Set("If-Match", etag)
		}
		res, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != want {
			t.Fatalf("%s status %d, want %d: %s", method, res.StatusCode, want, b)
		}
		return res.Header.Get("ETag"), b
	}
	etag, _ := request("GET", "", "", 200)
	next, _ := request("PATCH", `{"properties":{"name":"Updated native fixture"}}`, etag, 200)
	if etag == "" || next == etag {
		t.Fatal("missing or unchanged revision")
	}
	request("PATCH", `{"properties":{"name":"stale overwrite"}}`, etag, 412)
	var after []byte
	var name, amount string
	if err := db.QueryRow("SELECT geom,name,amount_text FROM sites WHERE fid=1").Scan(&after, &name, &amount); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || name != "Updated native fixture" || amount != "9007199254740993.125" {
		t.Fatalf("preservation failed: name=%q amount=%q geometryEqual=%v", name, amount, bytes.Equal(before, after))
	}
	var children int
	if err := db.QueryRow("SELECT count(*) FROM review_children").Scan(&children); err != nil {
		t.Fatal(err)
	}
	if children != 2 {
		t.Fatalf("children=%d, want 2", children)
	}
	// PUT must update in place, preserving foreign-key children and avoiding
	// the fixture's DELETE trigger. Move the geometry and inspect the RTree.
	replacement := `{"type":"Feature","geometry":{"type":"Point","coordinates":[38,56]},
		"properties":{"fixture_key":"sites-1","tenant_id":1,"name":"Replacement","note":"",
		"amount_text":"9007199254740993.125","counter":0,"server_tag":"immutable-seed"}}`
	next, _ = request("PUT", replacement, next, 200)
	var deletes int
	if err := db.QueryRow("SELECT count(*) FROM review_events WHERE operation='DELETE'").Scan(&deletes); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT count(*) FROM review_children").Scan(&children); err != nil {
		t.Fatal(err)
	}
	if deletes != 0 || children != 2 {
		t.Fatalf("PUT deleted existing row: deletes=%d children=%d", deletes, children)
	}
	var minx, maxx, miny, maxy float64
	if err := db.QueryRow("SELECT minx,maxx,miny,maxy FROM rtree_sites_geom WHERE id=1").Scan(&minx, &maxx, &miny, &maxy); err != nil {
		t.Fatal(err)
	}
	if minx != 38 || maxx != 38 || miny != 56 || maxy != 56 {
		t.Fatalf("incorrect RTree: %v", []float64{minx, maxx, miny, maxy})
	}
	if err := db.QueryRow("SELECT max_x,max_y FROM gpkg_contents WHERE table_name='sites'").Scan(&maxx, &maxy); err != nil {
		t.Fatal(err)
	}
	if maxx < 38 || maxy < 56 {
		t.Fatalf("gpkg_contents excludes moved geometry: max_x=%v max_y=%v", maxx, maxy)
	}
	// A trigger failure must roll back the data and its revision together.
	if _, err := db.Exec("CREATE TRIGGER review_injected_failure AFTER UPDATE ON sites WHEN NEW.name='rollback' BEGIN SELECT RAISE(ABORT,'injected'); END"); err != nil {
		t.Fatal(err)
	}
	request("PATCH", `{"properties":{"name":"rollback"}}`, next, 400)
	current, _ := request("GET", "", "", 200)
	if current != next {
		t.Fatalf("failed update changed revision: %q -> %q", next, current)
	}
	if err := db.QueryRow("SELECT name FROM sites WHERE fid=1").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "Replacement" {
		t.Fatalf("failed update changed data: %q", name)
	}
	request("DELETE", "", next, 204)
	request("GET", "", "", 404)
	if err := db.QueryRow("SELECT count(*) FROM review_children").Scan(&children); err != nil {
		t.Fatal(err)
	}
	if children != 0 {
		t.Fatalf("foreign-key cascade not enforced: %d children remain", children)
	}
	var remaining int
	if err := db.QueryRow("SELECT count(*) FROM rtree_sites_geom WHERE id=1").Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatal("deleted geometry remains in RTree")
	}
	// Exercise WFS-T through the same live HTTP router and native file.
	wfsBody := `<wfs:Transaction version="2.0.0" service="WFS"
	 xmlns:wfs="http://www.opengis.net/wfs/2.0" xmlns:fes="http://www.opengis.net/fes/2.0">
	 <wfs:Update typeName="sites"><wfs:Property><wfs:ValueReference>name</wfs:ValueReference>
	 <wfs:Value>WFS native</wfs:Value></wfs:Property><fes:Filter><fes:ResourceId rid="sites.2"/></fes:Filter></wfs:Update>
	 </wfs:Transaction>`
	wfsResponse, err := srv.Client().Post(srv.URL+"/wfs", "application/xml", strings.NewReader(wfsBody))
	if err != nil {
		t.Fatal(err)
	}
	defer wfsResponse.Body.Close()
	wfsData, err := io.ReadAll(wfsResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	var transaction struct {
		XMLName xml.Name
		Updated int `xml:"TransactionSummary>totalUpdated"`
	}
	if err := xml.Unmarshal(wfsData, &transaction); err != nil {
		t.Fatal(err)
	}
	if wfsResponse.StatusCode != 200 || transaction.XMLName.Space != "http://www.opengis.net/wfs/2.0" || transaction.Updated != 1 {
		t.Fatalf("WFS transaction status=%d response=%s", wfsResponse.StatusCode, wfsData)
	}
	if err := db.QueryRow("SELECT name FROM sites WHERE fid=2").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "WFS native" {
		t.Fatalf("WFS changed wrong row: %q", name)
	}
	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity=%q error=%v", integrity, err)
	}
	for table, before := range untouched {
		var after []byte
		if err := db.QueryRow("SELECT geom FROM " + table + " WHERE fid=1").Scan(&after); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("untouched polygon/multipart/XYZ geometry changed: %s", table)
		}
	}
	sourceAfter, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sourceAfter, data) {
		t.Fatal("original fixture was modified")
	}
}
