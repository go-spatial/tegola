//go:build cgo

package server

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/gpkg"
)

type protocolDBRow struct {
	id       int64
	geometry any
	instant  any
}

func TestFeatureProtocolGPKGSourceErrors(t *testing.T) {
	for _, tc := range []struct {
		name, format string
		geometry     any
		instant      any
		temporal     bool
		status       int
	}{
		{"malformed native", "gpkg", []byte{'G', 'P', 0, 1, 230, 16, 0, 0, 1, 1, 0, 0, 0, 255}, nil, false, 500},
		{"malformed temporal", "wkt", "POINT Z (1 2 3)", "source-secret-invalid-time", true, 500},
		{"unsupported measured geometry", "wkt", "POINT M (1 2 3)", nil, false, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := protocolDBProvider(t, tc.format, []protocolDBRow{{1, tc.geometry, tc.instant}}, tc.temporal)
			layers, err := p.Layers()
			if err != nil {
				t.Fatal(err)
			}
			srv := protocolServer(t, p, layers[0], "/")
			r := protocolRequest(t, srv, "GET", "/features/collections/public/items/1")
			if r.status != tc.status || strings.Contains(string(r.body), "source-secret") || r.header.Get("Cache-Control") != "no-store" {
				t.Fatalf("source error: %d %s", r.status, r.body)
			}
		})
	}
}

// Geometry bytes are built directly from literal ordinates, without invoking
// the production encoder/decoder as an oracle.
func protocolWKBLine(points ...[3]float64) []byte {
	b := []byte{1}
	b = binary.LittleEndian.AppendUint32(b, 1002)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(points)))
	for _, point := range points {
		for _, v := range point {
			b = binary.LittleEndian.AppendUint64(b, math.Float64bits(v))
		}
	}
	return b
}
func protocolDBProvider(t *testing.T, format string, rows []protocolDBRow, temporal bool) *gpkg.Provider {
	t.Helper()
	path := filepath.Join(t.TempDir(), "protocol.gpkg")
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
		for _, ddl := range []string{"CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,at INTEGER)", "CREATE TABLE gpkg_geometry_columns(table_name TEXT,column_name TEXT,geometry_type_name TEXT,srs_id INTEGER,z INTEGER,m INTEGER)", "CREATE TABLE gpkg_contents(table_name TEXT,data_type TEXT,min_x REAL,min_y REAL,max_x REAL,max_y REAL,srs_id INTEGER)"} {
			if _, err := db.Exec(ddl); err != nil {
				t.Fatal(err)
			}
		}
		if format == "gpkg" {
			if _, err := db.Exec("INSERT INTO gpkg_geometry_columns VALUES('items','geom','GEOMETRY',4326,2,0); INSERT INTO gpkg_contents VALUES('items','features',-180,-90,180,90,4326)"); err != nil {
				t.Fatal(err)
			}
		}
		for _, row := range rows {
			if _, err := db.Exec("INSERT INTO items VALUES(?,?,?)", row.id, row.geometry, row.instant); err != nil {
				t.Fatal(err)
			}
		}
	}()
	layer := map[string]any{"name": "source", "tablename": "items", "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": format, "geometry_type": "linestring", "srid": 4326, "spatial_dimension": "mixed_xy_xyz", "vertical_crs": provider.CRS84h}
	if temporal {
		layer["temporal_field"] = "at"
		layer["temporal_storage"] = "unix_seconds"
	}
	tiler, err := gpkg.NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]any{layer}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := tiler.(*gpkg.Provider)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	})
	return p
}
func TestFeatureProtocolGPKGExact3DAndDatetime(t *testing.T) {
	// ID10 has overlapping XYZ ranges but cannot intersect the box: its x<=1
	// portion has z<=1, while the box requires z>=9. No numeric tolerance.
	line := protocolWKBLine([3]float64{0, 0, 0}, [3]float64{10, 0, 10})
	hit := protocolWKBLine([3]float64{0, 0, 9}, [3]float64{1, 0, 10})
	rows := []protocolDBRow{{10, line, int64(0)}, {20, hit, int64(0)}, {30, hit, int64(1)}, {40, nil, nil}, {50, protocolWKBLine([3]float64{1, 0, 9}, [3]float64{1, 0, 10}), int64(0)}}
	p := protocolDBProvider(t, "wkb", rows, true)
	layers, err := p.Layers()
	if err != nil {
		t.Fatal(err)
	}
	srv := protocolServer(t, p, layers[0], "/stage")
	base := "/stage/features/collections/public/items?limit=2&bbox=0,-1,9,1,1,10&datetime=1970-01-01T00:00:00Z"
	r := protocolRequest(t, srv, "GET", base)
	if r.status != 200 || !reflect.DeepEqual(protocolIDs(t, r), []uint64{20, 40}) {
		t.Fatalf("combined static oracle: %d %s", r.status, r.body)
	}
	var page struct {
		NumberMatched *uint64
		Features      []struct {
			ID       uint64
			Geometry json.RawMessage
		}
		Links []struct{ Href, Rel string }
	}
	if err := json.Unmarshal(r.body, &page); err != nil {
		t.Fatal(err)
	}
	if page.NumberMatched != nil && *page.NumberMatched != 3 {
		t.Fatal("exact count", string(r.body))
	}
	var g struct {
		Type        string
		Coordinates [][]float64
	}
	if err := json.Unmarshal(page.Features[0].Geometry, &g); err != nil {
		t.Fatal(err)
	}
	if g.Type != "LineString" || !reflect.DeepEqual(g.Coordinates, [][]float64{{0, 0, 9}, {1, 0, 10}}) {
		t.Fatal("Z lost or changed", g)
	}
	second := protocolRequest(t, srv, "GET", base+"&offset=2")
	if !reflect.DeepEqual(protocolIDs(t, second), []uint64{50}) {
		t.Fatal("page after exact predicates", string(second.body))
	}
	for _, tc := range []struct {
		query string
		ids   []uint64
	}{
		{"limit=2&offset=1&bbox=0,-1,9,1,1,10&datetime=1970-01-01T00:00:00Z", []uint64{40, 50}},
		{"limit=2&bbox=0,-1,1,1&datetime=1970-01-01T00:00:00Z", []uint64{10, 20}},
		{"limit=2&bbox=0,-1,9,1,1,10&datetime=1970-01-01T01:00:00%2B01:00", []uint64{20, 40}},
		{"limit=2&datetime=1970-01-01T00:00:00.0000000001Z", []uint64{40}},
		{"limit=2&datetime=../1969-12-31T23:59:59.9999999999Z", []uint64{40}},
		{"limit=2&datetime=2016-12-31T23:59:60Z", []uint64{40}},
	} {
		r := protocolRequest(t, srv, "GET", "/stage/features/collections/public/items?"+tc.query)
		if r.status != 200 || !reflect.DeepEqual(protocolIDs(t, r), tc.ids) {
			t.Errorf("%s: %d %s want %v", tc.query, r.status, r.body, tc.ids)
		}
	}
}
func TestFeatureProtocolGPKGDimensionalFormatsAndHole(t *testing.T) {
	for _, format := range []string{"wkb", "wkt", "gpkg"} {
		t.Run(format, func(t *testing.T) {
			var value any = protocolWKBLine([3]float64{15, 30, 7}, [3]float64{16, 31, 8})
			if format == "wkt" {
				value = "LINESTRING Z (15 30 7,16 31 8)"
			}
			if format == "gpkg" {
				header := []byte{'G', 'P', 0, 1, 0, 0, 0, 0}
				binary.LittleEndian.PutUint32(header[4:], 4326)
				value = append(header, value.([]byte)...)
			}
			p := protocolDBProvider(t, format, []protocolDBRow{{7, value, nil}}, false)
			layers, err := p.Layers()
			if err != nil {
				t.Fatal(err)
			}
			srv := protocolServer(t, p, layers[0], "/")
			r := protocolRequest(t, srv, "GET", "/features/collections/public/items/7")
			var f struct {
				Geometry struct{ Coordinates [][]float64 }
			}
			if r.status != 200 {
				t.Fatal(string(r.body))
			}
			if err := json.Unmarshal(r.body, &f); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(f.Geometry.Coordinates, [][]float64{{15, 30, 7}, {16, 31, 8}}) {
				t.Fatal(f)
			}
		})
	}
	p := protocolDBProvider(t, "wkt", []protocolDBRow{{10, "POLYGON Z ((0 0 0,10 0 10,10 10 10,0 10 0,0 0 0),(4 4 4,6 4 6,6 6 6,4 6 4,4 4 4))", nil}, {20, "POINT (5 5)", nil}, {30, nil, nil}}, false)
	layers, err := p.Layers()
	if err != nil {
		t.Fatal(err)
	}
	srv := protocolServer(t, p, layers[0], "/")
	for _, tc := range []struct {
		bbox string
		ids  []uint64
	}{{"4.5,4.5,4.5,5.5,5.5,5.5", []uint64{20, 30}}, {"4,5,4,4,5,4", []uint64{10, 30}}, {"1,1,8,2,2,9", []uint64{30}}} {
		r := protocolRequest(t, srv, "GET", "/features/collections/public/items?limit=2&bbox="+tc.bbox)
		if r.status != 200 || !reflect.DeepEqual(protocolIDs(t, r), tc.ids) {
			t.Errorf("hole/XY-height %s: %d %s", tc.bbox, r.status, r.body)
		}
	}
}

func TestFeatureProtocolGPKGAntimeridianAndExactTime(t *testing.T) {
	p := protocolDBProvider(t, "wkt", []protocolDBRow{{10, "POINT Z (179 0 7)", int64(1483228799)}, {20, "POINT Z (-179 0 7)", int64(1483228800)}, {30, "POINT Z (0 0 7)", nil}, {40, nil, nil}}, true)
	layers, err := p.Layers()
	if err != nil {
		t.Fatal(err)
	}
	srv := protocolServer(t, p, layers[0], "/")
	cases := []struct {
		query string
		ids   []uint64
	}{
		{"bbox=170,-1,8,-170,1,9", []uint64{40}},
		{"bbox=170,-1,6,-170,1,8", []uint64{10, 20}},
		{"bbox=170,-1,-170,1&offset=1", []uint64{20, 40}},
		{"datetime=2016-12-31T23:59:60.5Z", []uint64{30, 40}},
		{"datetime=2017-01-01T00:59:60.5%2B01:00", []uint64{30, 40}},
		{"datetime=2016-12-31T23:59:59Z/2017-01-01T00:00:00Z", []uint64{10, 20}},
		{"datetime=2016-12-31T23:59:59.0000000000000000000000000000Z", []uint64{10, 30}},
		{"datetime=2016-12-31t23:59:59z", []uint64{10, 30}},
		{"datetime=2016-12-31T23:59:59.0000000000000000000000000001Z", []uint64{30, 40}},
	}
	for _, tc := range cases {
		r := protocolRequest(t, srv, "GET", "/features/collections/public/items?limit=2&"+tc.query)
		if r.status != 200 || !reflect.DeepEqual(protocolIDs(t, r), tc.ids) {
			t.Errorf("%s: %d %s want %v", tc.query, r.status, r.body, tc.ids)
		}
	}
	for _, raw := range []string{"2016-12-31T23:59:59.0000000002Z/2016-12-31T23:59:59.0000000001Z", "2016-12-30T23:59:60Z", "2017-01-01T00:00:60Z"} {
		r := protocolRequest(t, srv, "GET", "/features/collections/public/items?datetime="+raw)
		if r.status != 400 {
			t.Errorf("invalid exact time %s: %d %s", raw, r.status, r.body)
		}
	}
}
