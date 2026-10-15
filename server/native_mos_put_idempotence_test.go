//go:build cgo

package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
	pa "github.com/alexeydott/tegola/provider/audit"
	"github.com/alexeydott/tegola/provider/gpkg"
)

func TestNativeMOSPutPreservesIdenticalReadGeometry(t *testing.T) {
	t.Run("properties", func(t *testing.T) { testNativeMOSPutPreservation(t, []string{"name"}) })
	t.Run("geometry only", func(t *testing.T) { testNativeMOSPutPreservation(t, []string{"id"}) })
}

func testNativeMOSPutPreservation(t *testing.T, fields []string) {
	path := filepath.Join(t.TempDir(), "put.sqlite")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	_, err = db.Exec("CREATE TABLE sites(id INTEGER PRIMARY KEY,geom BLOB NOT NULL,name TEXT,MINX INTEGER,MAXX INTEGER,MINY INTEGER,MAXY INTEGER)")
	if err != nil {
		t.Fatal(err)
	}
	if err = pa.Migrate(context.Background(), db, "sqlite"); err != nil {
		t.Fatal(err)
	}
	source, err := mos.Encode(geom.Point{1000, 2000}, mos.Options{Precision: 0, UnitFactor: 0.001})
	if err != nil {
		t.Fatal(err)
	}
	source = append(source, []byte("opaque annotation")...)
	_, err = db.Exec("INSERT INTO sites VALUES(1,?,'source',1000000,1000000,2000000,2000000)", source)
	if err != nil {
		t.Fatal(err)
	}
	p0, err := gpkg.NewTileProvider(dict.Dict{
		"filepath": path,
		"layers": []map[string]interface{}{{
			"name": "sites", "tablename": "sites", "id_fieldname": "id",
			"geometry_fieldname": "geom", "geometry_format": "mos", "geometry_type": "point",
			"crs_defn": "+proj=etmerc +ellps=bessel +towgs84=10,20,30,0.1,-0.2,0.3,1 " +
				"+lon_0=30 +lat_0=50 +k_0=1 +x_0=1000 +y_0=2000 +units=m",
			"mos_precision": 0, "mos_units": "mm", "fields": fields,
		}},
	}, nil)
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
	service, err := features.NewService([]features.CollectionSource{{ID: "sites", Layer: layers[0], Querier: p}})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(part4Router(t, part4API(t, service)))
	defer srv.Close()
	request := func(method string, body []byte, tag string) ([]byte, string, int) {
		t.Helper()
		r, err := http.NewRequest(method, srv.URL+"/features/collections/sites/items/1", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Content-Type", "application/geo+json")
		if tag != "" {
			r.Header.Set("If-Match", tag)
		}
		response, err := srv.Client().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return data, response.Header.Get("ETag"), response.StatusCode
	}
	body, tag, status := request("GET", nil, "")
	if status != 200 {
		t.Fatalf("GET %d %s", status, body)
	}
	initialTag := tag
	for i := 0; i < 3; i++ {
		var feature map[string]json.RawMessage
		if err = json.Unmarshal(body, &feature); err != nil {
			t.Fatal(err)
		}
		feature["properties"] = json.RawMessage(`{"name":"replaced"}`)
		if fields[0] == "id" {
			feature["properties"] = json.RawMessage(`{}`)
		}
		input, err := json.Marshal(feature)
		if err != nil {
			t.Fatal(err)
		}
		condition := tag
		if i == 1 {
			condition = "*" // The preservation read supplies its own exact CAS.
		}
		body, tag, status = request("PUT", input, condition)
		if status != 200 {
			t.Fatalf("PUT %d %s", status, body)
		}
		var stored []byte
		var minx, maxx, miny, maxy int
		if err = db.QueryRow("SELECT geom,MINX,MAXX,MINY,MAXY FROM sites WHERE id=1").Scan(&stored, &minx, &maxx, &miny, &maxy); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(stored, source) || minx != 1000000 || maxx != 1000000 || miny != 2000000 || maxy != 2000000 {
			t.Fatalf("identical geometry PUT changed native bytes/bounds: %d %d %d %d", minx, maxx, miny, maxy)
		}
	}
	if _, _, status = request("PUT", body, initialTag); status != 412 {
		t.Fatalf("stale PUT %d", status)
	}
	// Even after an adapter has decided to preserve geometry, the provider must
	// reject a revision that changed before Apply, not preserve a newer row.
	tx, err := p.BeginFeatureTx(context.Background(), provider.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Apply(context.Background(), provider.Mutation{Op: provider.MutationReplace, Collection: "sites", FeatureID: 1, GeometryUnchanged: true, IfRevision: revisionFromETag(initialTag), Properties: map[string]provider.MutationValue{"name": {Kind: provider.MutationValueString, String: "must not write"}}})
	var mutationError *provider.MutationError
	if !errors.As(err, &mutationError) || mutationError.Kind != provider.MutationErrPreconditionFailed {
		t.Fatalf("native CAS: %v", err)
	}
	if err = tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	var replacement map[string]json.RawMessage
	if err = json.Unmarshal(body, &replacement); err != nil {
		t.Fatal(err)
	}
	replacement["geometry"] = json.RawMessage(`{"type":"Point","coordinates":[30.001,50.001]}`)
	changed, err := json.Marshal(replacement)
	if err != nil {
		t.Fatal(err)
	}
	if result, _, code := request("PUT", changed, tag); code != 200 {
		t.Fatalf("changed PUT %d %s", code, result)
	}
	var stored []byte
	if err = db.QueryRow("SELECT geom FROM sites WHERE id=1").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(stored, source) {
		t.Fatal("explicit geometry change was ignored")
	}
}
