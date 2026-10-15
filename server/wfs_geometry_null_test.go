//go:build cgo

package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider/gpkg"
)

func TestWFSGeometryNullNativeSQLite(t *testing.T) {
	for _, nullable := range []bool{true, false} {
		t.Run(fmt.Sprintf("nullable=%v", nullable), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "null.sqlite")
			db, err := sql.Open("sqlite3", path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			constraint := " DEFAULT 'POINT (3 4)'"
			if !nullable {
				constraint = " NOT NULL DEFAULT 'POINT (3 4)'"
			}
			if _, err = db.Exec(`CREATE TABLE wfs_sites(fid INTEGER PRIMARY KEY,geom TEXT` + constraint + `,name TEXT,seats INTEGER); INSERT INTO wfs_sites VALUES(1,'POINT (1 2)','initial',1)`); err != nil {
				t.Fatal(err)
			}
			tiler, err := gpkg.NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]interface{}{{"name": "wfs_sites", "tablename": "wfs_sites", "id_fieldname": "fid", "geometry_fieldname": "geom", "geometry_format": "wkt", "srid": 4326, "fields": []string{"name", "seats"}}}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(gpkg.Cleanup)
			p := tiler.(*gpkg.Provider)
			layers, err := p.Layers()
			if err != nil {
				t.Fatal(err)
			}
			svc, err := features.NewService([]features.CollectionSource{{ID: "wfs_sites", Layer: layers[0], Querier: p}})
			if err != nil {
				t.Fatal(err)
			}
			handler := &WFSHandler{Service: svc, Config: config.WFSConfig{Enabled: true, BasePath: "/wfs", Versions: []string{"1.1.0", "2.0.0", "2.0.2"}}.Resolved(), WriteConfig: config.FeaturesWriteConfig{Enabled: true, AuthMode: "dev", Collections: []config.WriteCollectionConfig{{ID: "wfs_sites", Operations: []string{"create", "replace", "update", "delete"}}}}}
			preserveDiscoveryGlobals(t)
			router, err := NewRouterWithOptions(nil, RouterOptions{WFS: handler})
			if err != nil {
				t.Fatal(err)
			}
			for _, version := range []string{"1.1.0", "2.0.0", "2.0.2"} {
				namespace := "http://www.opengis.net/wfs/2.0"
				if version == "1.1.0" {
					namespace = "http://www.opengis.net/wfs"
				}
				post := func(action string, want int) {
					t.Helper()
					action = strings.ReplaceAll(action, "<geom", "<geometry")
					action = strings.ReplaceAll(action, "</geom>", "</geometry>")
					action = strings.ReplaceAll(action, ">geom<", ">geometry<")
					action = strings.ReplaceAll(action, "<wfs_sites>", "<app:wfs_sites>")
					action = strings.ReplaceAll(action, "</wfs_sites>", "</app:wfs_sites>")
					for _, property := range []string{"name", "geometry"} {
						action = strings.ReplaceAll(action, "<"+property, "<app:"+property)
						action = strings.ReplaceAll(action, "</"+property+">", "</app:"+property+">")
					}
					if version == "1.1.0" {
						action = strings.ReplaceAll(action, "ValueReference", "Name")
					}
					body := fmt.Sprintf(`<Transaction xmlns="%s" xmlns:app="http://example.com/tegola/wfs_sites" service="WFS" version="%s" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">%s</Transaction>`, namespace, version, action)
					r := httptest.NewRequest(http.MethodPost, "/wfs", strings.NewReader(body))
					r.Header.Set("Content-Type", "application/xml")
					w := httptest.NewRecorder()
					router.ServeHTTP(w, r)
					if w.Code != want {
						t.Fatalf("%s %s: HTTP %d want %d: %s", version, action, w.Code, want, w.Body.String())
					}
				}
				filter := `<fes:Filter xmlns:fes="http://www.opengis.net/fes/2.0"><fes:ResourceId rid="wfs_sites.1"/></fes:Filter>`
				if version == "1.1.0" {
					filter = `<ogc:Filter xmlns:ogc="http://www.opengis.net/ogc"><ogc:FeatureId fid="wfs_sites.1"/></ogc:Filter>`
				}
				for _, op := range []string{"Insert", "Update", "Replace"} {
					if version == "1.1.0" && op == "Replace" {
						continue
					}
					if _, err := db.Exec(`UPDATE wfs_sites SET geom='POINT (1 2)' WHERE fid=1`); err != nil {
						t.Fatal(err)
					}
					if _, err := db.Exec(`DELETE FROM wfs_sites WHERE fid<>1`); err != nil {
						t.Fatal(err)
					}
					action := `<` + op + `><wfs_sites><name>nil-test</name><geom xsi:nil="true"/></wfs_sites>`
					if op == "Update" {
						action = `<Update typeName="wfs_sites"><Property><ValueReference>geom</ValueReference><Value xsi:nil="1"/></Property>`
					}
					if op != "Insert" {
						action += filter
					}
					action += `</` + op + `>`
					want := http.StatusOK
					if !nullable {
						want = http.StatusBadRequest
					}
					post(action, want)
					var count int
					query := `SELECT count(*) FROM wfs_sites WHERE fid=1 AND geom IS NULL`
					if op == "Insert" {
						query = `SELECT count(*) FROM wfs_sites WHERE name='nil-test' AND geom IS NULL`
					}
					if err := db.QueryRow(query).Scan(&count); err != nil {
						t.Fatal(err)
					}
					if nullable && count == 0 || !nullable && count != 0 {
						t.Fatalf("%s %s native NULL count=%d", version, op, count)
					}
				}
				if _, err := db.Exec(`UPDATE wfs_sites SET geom='POINT (1 2)' WHERE fid=1`); err != nil {
					t.Fatal(err)
				}
				post(`<Update typeName="wfs_sites"><Property><ValueReference>name</ValueReference><Value>renamed</Value></Property>`+filter+`</Update>`, http.StatusOK)
				var geometry string
				if err := db.QueryRow(`SELECT geom FROM wfs_sites WHERE fid=1`).Scan(&geometry); err != nil || geometry != "POINT (1 2)" {
					t.Fatalf("omitted Update changed geometry: %q %v", geometry, err)
				}
				post(`<Update typeName="wfs_sites"><Property><ValueReference>geom</ValueReference><Value/></Property>`+filter+`</Update>`, http.StatusBadRequest)
				if nullable {
					post(`<Insert><wfs_sites><name>omitted-geometry</name></wfs_sites></Insert>`, http.StatusOK)
					var defaultGeometry string
					if err := db.QueryRow(`SELECT geom FROM wfs_sites WHERE name='omitted-geometry'`).Scan(&defaultGeometry); err != nil || defaultGeometry != "POINT (3 4)" {
						t.Fatalf("omitted Insert did not retain native default: %q %v", defaultGeometry, err)
					}
					if _, err := db.Exec(`DELETE FROM wfs_sites WHERE name='omitted-geometry'`); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}
