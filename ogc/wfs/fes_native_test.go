//go:build cgo

package wfs

import (
	"context"
	"database/sql"
	"encoding/xml"
	"path/filepath"
	"testing"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider/gpkg"
)

// Executes FES through the real SQLite-backed GPKG provider (WKT profile),
// checking persisted row identities rather than just the constructed AST.
func TestPR203NativeSQLiteTypedFES(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fes.sqlite")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE sites(fid INTEGER PRIMARY KEY,geom TEXT,code TEXT,active BOOLEAN,n INTEGER);
 INSERT INTO sites VALUES(1,'POINT (1 2)','123',1,9007199254740993),(2,'POINT (1 2)','00123',0,9007199254740992),(3,'POINT (1 2)','1e3',1,3);`)
	if err != nil {
		t.Fatal(err)
	}
	tiler, err := gpkg.NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]interface{}{{"name": "sites", "tablename": "sites", "id_fieldname": "fid", "geometry_fieldname": "geom", "geometry_format": "wkt", "srid": 4326, "fields": []string{"code", "active", "n"}}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(gpkg.Cleanup)
	p := tiler.(*gpkg.Provider)
	layers, err := p.Layers()
	if err != nil {
		t.Fatal(err)
	}
	service, err := features.NewService([]features.CollectionSource{{ID: "sites", Layer: layers[0], Querier: p}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		property, literal string
		ids               []string
	}{
		{"code", "123", []string{"sites.1"}},
		{"code", "00123", []string{"sites.2"}},
		{"code", "1e3", []string{"sites.3"}},
		{"active", "false", []string{"sites.2"}},
		{"active", "true", []string{"sites.1", "sites.3"}},
		{"n", "9007199254740993", []string{"sites.1"}},
	} {
		t.Run(tc.property+"="+tc.literal, func(t *testing.T) {
			filter := `<fes:Filter xmlns:fes="http://www.opengis.net/fes/2.0" xmlns:app="http://example.com/tegola/sites"><fes:PropertyIsEqualTo><fes:ValueReference>app:` + tc.property + `</fes:ValueReference><fes:Literal>` + tc.literal + `</fes:Literal></fes:PropertyIsEqualTo></fes:Filter>`
			req, ex := ParseGetFeatureKVP(V202, map[string]string{"typename": "app:sites", "filter": filter})
			if len(ex) > 0 {
				t.Fatal(ex)
			}
			raw, ex := ExecuteGetFeature(context.Background(), service, req)
			if len(ex) > 0 {
				t.Fatal(ex)
			}
			var collection struct {
				Members []struct {
					Feature struct {
						ID string `xml:"id,attr"`
					} `xml:",any"`
				} `xml:"member"`
			}
			if err := xml.Unmarshal([]byte(raw), &collection); err != nil {
				t.Fatal(err)
			}
			if len(collection.Members) != len(tc.ids) {
				t.Fatalf("want %v got %s", tc.ids, raw)
			}
			for i, m := range collection.Members {
				if m.Feature.ID != tc.ids[i] {
					t.Fatalf("want %v got %s", tc.ids, raw)
				}
			}
		})
	}
}
