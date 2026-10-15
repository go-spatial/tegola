//go:build cgo

package features

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/gpkg"
)

// Frozen before acceptance runtime using independent scalar Python math. UTM
// central-meridian meridional arc uses Snyder's e6 series. 1e-6 degrees (~0.11m)
// exceeds the series residual and MOS centimetre quantization. No production
// projection routine generates these source coordinates or the expected values.
func TestGPKGAcceptanceIndependentCRS84(t *testing.T) {
	basic.RegisterBuiltinProj4SRIDs()
	tests := []struct {
		name, format, definition string
		srid                     int
		xy, expected             [2]float64
		systemInfo               bool
	}{
		{name: "native4326", format: "gpkg", srid: 4326, xy: [2]float64{15, 30}, expected: [2]float64{15, 30}},
		{name: "rawWKB3857", format: "wkb", srid: 3857, xy: [2]float64{1669792.3618991037, 3503549.843504374}, expected: [2]float64{15, 30}},
		{name: "rawWKTUTM", format: "wkt", srid: 32633, xy: [2]float64{500000, 3318785.352608442}, expected: [2]float64{15, 30}},
		{name: "customCRS", format: "wkb", definition: "+proj=utm +zone=33 +datum=WGS84 +units=m +no_defs", xy: [2]float64{500000, 3318785.352608442}, expected: [2]float64{15, 30}},
		{name: "nonzeroDatumShift", format: "wkb", definition: "+proj=longlat +ellps=GRS80 +towgs84=0,100,0 +no_defs", xy: [2]float64{0, 0}, expected: [2]float64{0.0008983152840459143, 0}},
		{name: "MOSsystemInfo", format: "mos", definition: "+proj=utm +zone=33 +datum=WGS84 +units=m +no_defs", xy: [2]float64{500000, 3318785.352608442}, expected: [2]float64{15, 30}, systemInfo: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			layer := map[string]any{"geometry_format": tc.format, "geometry_type": "point"}
			if tc.srid != 0 {
				layer["srid"] = tc.srid
			}
			if tc.definition != "" && !tc.systemInfo {
				layer["crs_defn"] = tc.definition
			}
			rows := [][]any{{int64(7), acceptanceGeometry(t, tc.format, tc.srid, geom.Point(tc.xy)), "published"}}
			if tc.systemInfo {
				rows = append([][]any{{int64(0), acceptanceSystemInfo(tc.definition), "metadata"}}, rows...)
			}
			p, s := acceptanceService(t, layer, rows)
			sourceBefore := []provider.Feature{}
			_, err := p.QueryFeatures(context.Background(), "source", provider.FeatureQuery{Limit: 1, IDs: []uint64{7}}, func(f *provider.Feature) error { sourceBefore = append(sourceBefore, *f); return nil })
			if err != nil {
				t.Fatal(err)
			}
			value, err := s.QueryFeature(context.Background(), "public", 7)
			if err != nil {
				t.Fatal(err)
			}
			var geometry struct {
				Type        string
				Coordinates [2]float64
			}
			if err := json.Unmarshal(value.Geometry, &geometry); err != nil {
				t.Fatal(err)
			}
			if geometry.Type != "Point" || value.ID != 7 || value.Properties["name"] != "published" {
				t.Fatalf("identity/type/properties lost: %+v %s", value, value.Geometry)
			}
			for axis, expected := range tc.expected {
				if math.Abs(geometry.Coordinates[axis]-expected) > 1e-6 {
					t.Fatalf("axis %d: got %.12f want %.12f", axis, geometry.Coordinates[axis], expected)
				}
			}
			sourceAfter := []provider.Feature{}
			_, err = p.QueryFeatures(context.Background(), "source", provider.FeatureQuery{Limit: 1, IDs: []uint64{7}}, func(f *provider.Feature) error { sourceAfter = append(sourceAfter, *f); return nil })
			if err != nil || !reflect.DeepEqual(sourceBefore, sourceAfter) {
				t.Fatalf("provider source changed: %v", err)
			}
		})
	}
}

func TestGPKGAcceptancePageNullCollectionAndCancellation(t *testing.T) {
	collection := geom.Collection{geom.Point{15, 30}, geom.Collection{geom.LineString{{15, 30}, {16, 31}}}}
	_, s := acceptanceService(t, map[string]any{"geometry_format": "wkb", "geometry_type": "point", "srid": 4326}, [][]any{
		{int64(10), acceptanceGeometry(t, "wkb", 4326, collection), "collection"},
		{int64(20), nil, "null"},
		{int64(30), acceptanceGeometry(t, "wkb", 4326, geom.Point{100, 50}), "outside"},
	})
	query := provider.FeatureQuery{Limit: 1, Bounds: []geom.Extent{{15, 30, 16, 31}}, BoundsSRID: 4326}
	page, err := s.QueryCollectionPage(context.Background(), "public", query)
	if err != nil || len(page.Features) != 1 || page.Features[0].ID != 10 || !page.HasMore {
		t.Fatalf("first page: %+v %v", page, err)
	}
	var geometry map[string]any
	if err := json.Unmarshal(page.Features[0].Geometry, &geometry); err != nil {
		t.Fatal(err)
	}
	if geometry["type"] != "GeometryCollection" {
		t.Fatalf("collection flattened: %s", page.Features[0].Geometry)
	}
	var expectedNested any
	if err := json.Unmarshal([]byte(`{"type":"GeometryCollection","geometries":[{"type":"Point","coordinates":[15,30]},{"type":"GeometryCollection","geometries":[{"type":"LineString","coordinates":[[15,30],[16,31]]}]}]}`), &expectedNested); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(geometry, expectedNested) {
		t.Fatalf("nested collection members/types/coordinates changed: %s", page.Features[0].Geometry)
	}

	query.Offset = 1
	page, err = s.QueryCollectionPage(context.Background(), "public", query)
	if err != nil || len(page.Features) != 1 || page.Features[0].ID != 20 || page.HasMore || string(page.Features[0].Geometry) != "null" {
		t.Fatalf("null second page: %+v %v", page, err)
	}
	var encoded bytes.Buffer
	if err := WriteGeoJSON(context.Background(), &encoded, page); err != nil {
		t.Fatal(err)
	}
	if !json.Valid(encoded.Bytes()) {
		t.Fatalf("invalid GeoJSON: %s", encoded.Bytes())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = s.QueryCollectionPage(ctx, "public", provider.FeatureQuery{Limit: 10})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancel chain lost: %v", err)
	}
	callbackErr := errors.New("acceptance callback")
	deliveries := 0
	_, err = s.QueryCollection(context.Background(), "public", provider.FeatureQuery{Limit: 10}, func(Feature) error { deliveries++; return fmt.Errorf("caller: %w", callbackErr) })
	if !errors.Is(err, callbackErr) || deliveries != 1 {
		t.Fatalf("callback error/stop: %d %v", deliveries, err)
	}
	var wg sync.WaitGroup
	failures := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := s.QueryFeature(context.Background(), "public", 20)
			if err == nil && string(value.Geometry) != "null" {
				err = errors.New("concurrent null result changed")
			}
			failures <- err
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Error(err)
		}
	}
}

func TestGPKGAcceptanceNativePolygonClosure(t *testing.T) {
	// Explicit expected outer/hole coordinates are independent of the decoder;
	// legacy WKB decode may omit duplicate closure, but GeoJSON must close rings.
	polygon := geom.Polygon{
		{{15, 30}, {16, 30}, {16, 31}, {15, 31}, {15, 30}},
		{{15.2, 30.2}, {15.2, 30.4}, {15.4, 30.4}, {15.4, 30.2}, {15.2, 30.2}},
	}
	_, s := acceptanceService(t, map[string]any{"geometry_format": "gpkg", "geometry_type": "polygon", "srid": 4326}, [][]any{{int64(7), acceptanceGeometry(t, "gpkg", 4326, polygon), "polygon"}})
	value, err := s.QueryFeature(context.Background(), "public", 7)
	if err != nil {
		t.Fatal(err)
	}
	var geometry struct {
		Type        string
		Coordinates [][][2]float64
	}
	if err := json.Unmarshal(value.Geometry, &geometry); err != nil {
		t.Fatal(err)
	}
	expected := [][][2]float64{
		{{15, 30}, {16, 30}, {16, 31}, {15, 31}, {15, 30}},
		{{15.2, 30.2}, {15.2, 30.4}, {15.4, 30.4}, {15.4, 30.2}, {15.2, 30.2}},
	}
	if geometry.Type != "Polygon" || len(geometry.Coordinates) != 2 {
		t.Fatalf("polygon/holes lost: %s", value.Geometry)
	}
	for ring, want := range expected {
		got := geometry.Coordinates[ring]
		if len(got) != len(want) {
			t.Fatalf("ring%d closure/count: %v", ring, got)
		}
		for point := range want {
			for axis := range 2 {
				if math.Abs(got[point][axis]-want[point][axis]) > 1e-10 {
					t.Fatalf("ring%dpoint%daxis%d changed", ring, point, axis)
				}
			}
		}
	}
}

func acceptanceService(t *testing.T, layer map[string]any, rows [][]any) (*gpkg.Provider, *Service) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "acceptance.gpkg")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, ddl := range []string{"CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,name TEXT)", "CREATE TABLE gpkg_contents(table_name TEXT,data_type TEXT,min_x REAL,min_y REAL,max_x REAL,max_y REAL,srs_id INTEGER)", "CREATE TABLE gpkg_geometry_columns(table_name TEXT,column_name TEXT,geometry_type_name TEXT,srs_id INTEGER,z INTEGER,m INTEGER)"} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	if layer["geometry_format"] == "gpkg" {
		if _, err := db.Exec("INSERT INTO gpkg_contents VALUES ('items','features',-180,-90,180,90,4326);INSERT INTO gpkg_geometry_columns VALUES ('items','geom','POINT',4326,0,0)"); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range rows {
		if _, err := db.Exec("INSERT INTO items VALUES (?,?,?)", row...); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	layer["name"] = "source"
	layer["tablename"] = "items"
	layer["id_fieldname"] = "id"
	layer["geometry_fieldname"] = "geom"
	layer["fields"] = []string{"name"}
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
	layers, err := p.Layers()
	if err != nil || len(layers) != 1 {
		t.Fatalf("layers: %v", err)
	}
	s, err := NewService([]CollectionSource{{ID: "public", Layer: layers[0], Querier: p}})
	if err != nil {
		t.Fatal(err)
	}
	return p, s
}

func acceptanceGeometry(t *testing.T, format string, srid int, g geom.Geometry) any {
	t.Helper()
	if format == "wkt" {
		point := g.(geom.Point)
		return fmt.Sprintf("POINT (%.17g %.17g)", point[0], point[1])
	}
	if format == "mos" {
		point := g.(geom.Point)
		blob := []byte{mos.TypePoint, 0}
		blob = binary.LittleEndian.AppendUint16(blob, 0)
		blob = binary.LittleEndian.AppendUint16(blob, 1)
		blob = binary.LittleEndian.AppendUint32(blob, 1)
		blob = binary.LittleEndian.AppendUint32(blob, 1)
		blob = binary.LittleEndian.AppendUint32(blob, uint32(int32(math.Round(point[0]*100))))
		blob = binary.LittleEndian.AppendUint32(blob, uint32(int32(math.Round(point[1]*100))))
		return blob
	}
	blob, err := wkb.EncodeBytes(g)
	if err != nil {
		t.Fatal(err)
	}
	if format == "gpkg" {
		header := binary.LittleEndian.AppendUint32([]byte{'G', 'P', 0, 1}, uint32(srid))
		return append(header, blob...)
	}
	return blob
}
func acceptanceSystemInfo(projection string) []byte {
	blob := make([]byte, 64+len(projection))
	blob[0] = 5
	copy(blob[1:], "Ver 1")
	binary.LittleEndian.PutUint32(blob[11:15], 2)
	blob[15] = 1
	binary.LittleEndian.PutUint32(blob[26:30], 1)
	binary.LittleEndian.PutUint32(blob[60:64], uint32(len(projection)))
	copy(blob[64:], projection)
	return blob
}
