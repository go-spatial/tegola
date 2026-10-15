//go:build cgo

package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/ogc/features"
	pa "github.com/alexeydott/tegola/provider/audit"
	"github.com/alexeydott/tegola/provider/gpkg"
)

func TestNativeMOSHTTPRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mos.sqlite")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := db.Exec("CREATE TABLE sites(id INTEGER PRIMARY KEY,geom BLOB,name TEXT)"); err != nil {
		t.Fatal(err)
	}
	if err := pa.Migrate(context.Background(), db, "sqlite"); err != nil {
		t.Fatal(err)
	}
	// Native ten-byte MapplGIS Point, independent of the writer encoder.
	source := []byte{2, 0, 0, 0, 1, 0, 1, 0, 0, 0}
	for _, word := range []uint32{1, 1113, 2226} {
		source = binary.LittleEndian.AppendUint32(source, word)
	}
	source = append(source, []byte("preserve-source-annotation")...)
	if _, err := db.Exec("INSERT INTO sites VALUES(1,?,'source')", source); err != nil {
		t.Fatal(err)
	}
	p0, err := gpkg.NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]interface{}{{
		"name": "sites", "tablename": "sites", "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "mos",
		"geometry_type": "point", "srid": 3857, "mos_precision": 2, "mos_units": "m", "fields": []string{"name"},
	}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := p0.(*gpkg.Provider)
	defer func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	}()
	layers, err := p.Layers()
	if err != nil {
		t.Fatal(err)
	}
	service, err := features.NewService([]features.CollectionSource{{ID: "sites", Title: "MOS sites", Layer: layers[0], Querier: p}})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(part4Router(t, part4API(t, service)))
	defer srv.Close()
	request := func(method, body, etag string, want int) string {
		t.Helper()
		req, err := http.NewRequest(method, srv.URL+"/features/collections/sites/items/1", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/merge-patch+json")
		if etag != "" {
			req.Header.Set("If-Match", etag)
		}
		res, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != want {
			t.Fatalf("%s returned %d want%d: %s", method, res.StatusCode, want, data)
		}
		return res.Header.Get("ETag")
	}
	etag := request("GET", "", "", 200)
	next := request("PATCH", `{"properties":{"name":"patched"}}`, etag, 200)
	var raw []byte
	if err := db.QueryRow("SELECT geom FROM sites WHERE id=1").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, source) {
		t.Fatal("HTTP attribute PATCH changed native MOS source bytes")
	}
	request("PATCH", `{"properties":{"name":"stale"}}`, etag, 412)
	next = request("PATCH", `{"geometry":{"type":"Point","coordinates":[0.0002,0.0003]}}`, next, 200)
	if err := db.QueryRow("SELECT geom FROM sites WHERE id=1").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	geometry, err := mos.Decode(raw, mos.Options{Precision: 2, UnitFactor: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(geometry, geom.Point{22.26, 33.4}) {
		t.Fatalf("HTTP CRS/quantization wrong: %#v", geometry)
	}
	request("DELETE", "", next, 204)
	request("GET", "", "", 404)
	var count int
	if err := db.QueryRow("SELECT count(*) FROM sites").Scan(&count); err != nil || count != 0 {
		t.Fatalf("delete left row: %d %v", count, err)
	}
}
