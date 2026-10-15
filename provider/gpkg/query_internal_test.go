//go:build cgo

package gpkg

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"encoding/binary"
	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
)

func queryTestProvider(t *testing.T, ddl string, overrides map[string]interface{}) (*Provider, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "query.gpkg")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ddl); err != nil {
		t.Fatal(err)
	}
	conf := map[string]interface{}{"name": "items", "tablename": "items", "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 4326}
	for key, value := range overrides {
		conf[key] = value
	}
	if _, ok := overrides["sql"]; ok {
		delete(conf, "tablename")
	}
	tiler, err := NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]interface{}{conf}}, nil)
	if err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	p := tiler.(*Provider)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return p, db
}

func queryIDs(t *testing.T, p *Provider, q provider.FeatureQuery) ([]uint64, provider.FeatureQueryResult) {
	t.Helper()
	ids := []uint64{}
	r, err := p.QueryFeatures(context.Background(), "items", q, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
	if err != nil {
		t.Fatal(err)
	}
	return ids, r
}

func TestFeatureTemporalPrecision(t *testing.T) {
	for _, profile := range []struct {
		name  string
		scale int64
	}{
		{"unix_seconds", 1}, {"unix_milliseconds", 1000}, {"unix_microseconds", 1000000}, {"unix_nanoseconds", 1000000000},
	} {
		t.Run(profile.name, func(t *testing.T) {
			p, db := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY, geom TEXT, at INTEGER)", map[string]interface{}{"temporal_field": "at", "temporal_storage": profile.name})
			if _, err := db.Exec("INSERT INTO items VALUES (1,'POINT (0 0)',-1),(2,'POINT (0 0)',0),(3,'POINT (0 0)',1),(4,NULL,NULL)"); err != nil {
				t.Fatal(err)
			}
			minusNS := time.Unix(-1, 999999999)
			ids, _ := queryIDs(t, p, provider.FeatureQuery{Limit: 10, Temporal: &provider.TemporalConstraint{Start: &minusNS, End: &minusNS}})
			want := []uint64{4}
			if profile.scale == 1000000000 {
				want = []uint64{1, 4}
			}
			if !reflect.DeepEqual(ids, want) {
				t.Fatalf("-1ns=%v want=%v", ids, want)
			}
			zero := time.Unix(0, 0)
			ids, _ = queryIDs(t, p, provider.FeatureQuery{Limit: 10, Temporal: &provider.TemporalConstraint{Start: &zero, End: &zero}})
			if !reflect.DeepEqual(ids, []uint64{2, 4}) {
				t.Fatalf("zero=%v", ids)
			}
			farFuture := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
			ids, _ = queryIDs(t, p, provider.FeatureQuery{Limit: 10, Temporal: &provider.TemporalConstraint{Start: &farFuture, End: &farFuture}})
			if !reflect.DeepEqual(ids, []uint64{4}) {
				t.Fatalf("far future=%v", ids)
			}
			if _, err := db.Exec("INSERT INTO items VALUES (5,'POINT (0 0)',?),(6,'POINT (0 0)',?)", int64(math.MinInt64), int64(math.MaxInt64)); err != nil {
				t.Fatal(err)
			}
			if profile.scale == 1000000000 {
				boundary := time.Unix(0, math.MaxInt64)
				ids, _ = queryIDs(t, p, provider.FeatureQuery{Limit: 10, Temporal: &provider.TemporalConstraint{Start: &boundary, End: &boundary}})
				if !reflect.DeepEqual(ids, []uint64{4, 6}) {
					t.Fatalf("int64 upper boundary=%v", ids)
				}
			}
		})
	}
}

func TestFeatureTemporalIntervalsAndInvalidRows(t *testing.T) {
	p, db := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY, geom TEXT, begin INTEGER, finish INTEGER)", map[string]interface{}{"temporal_start_field": "begin", "temporal_end_field": "finish", "temporal_storage": "unix_seconds"})
	if _, err := db.Exec("INSERT INTO items VALUES (1,NULL,-1,1),(2,NULL,NULL,0),(3,NULL,0,NULL),(4,NULL,NULL,NULL)"); err != nil {
		t.Fatal(err)
	}
	instant := time.Unix(0, 1)
	ids, _ := queryIDs(t, p, provider.FeatureQuery{Limit: 10, Temporal: &provider.TemporalConstraint{Start: &instant, End: &instant}})
	if !reflect.DeepEqual(ids, []uint64{1, 3, 4}) {
		t.Fatalf("subunit interval=%v", ids)
	}
	for _, value := range []any{"invalid", float64(1.5)} {
		if _, err := db.Exec("INSERT INTO items VALUES (5,NULL,?,0)", value); err != nil {
			t.Fatal(err)
		}
		_, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10, Temporal: &provider.TemporalConstraint{Start: &instant, End: &instant}}, func(*provider.Feature) error { return nil })
		var invalid provider.InvalidFeatureQueryError
		if !errors.As(err, &invalid) {
			t.Fatalf("invalid source coerced/hidden: %v", err)
		}
		if _, err := db.Exec("DELETE FROM items WHERE id=5"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("INSERT INTO items VALUES (5,NULL,2,1)"); err != nil {
		t.Fatal(err)
	}
	_, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10, Temporal: &provider.TemporalConstraint{Start: &instant, End: &instant}}, func(*provider.Feature) error { return nil })
	var invalid provider.InvalidFeatureQueryError
	if !errors.As(err, &invalid) {
		t.Fatalf("reversed source hidden: %v", err)
	}
}

func TestFeatureExactPagingAfterCoarseFalsePositives(t *testing.T) {
	p, db := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY, geom TEXT, minx REAL,maxx REAL,miny REAL,maxy REAL)", nil)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for id := 1; id <= 600; id++ {
		if _, err := tx.Exec("INSERT INTO items VALUES (?,'POINT (100 100)',0,0,0,0)", id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec("INSERT INTO items VALUES (601,'POINT (0 0)',0,0,0,0),(602,'POINT (0 0)',0,0,0,0),(603,NULL,999,999,999,999)"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	q := provider.FeatureQuery{Limit: 1, Offset: 1, Bounds: []geom.Extent{{0, 0, 0, 0}}, BoundsSRID: 4326}
	ids, result := queryIDs(t, p, q)
	if !reflect.DeepEqual(ids, []uint64{602}) || !result.HasMore {
		t.Fatalf("logical paging=%v %+v", ids, result)
	}
	q.Offset = 2
	ids, result = queryIDs(t, p, q)
	if !reflect.DeepEqual(ids, []uint64{603}) || result.HasMore || result.NumberMatched == nil || *result.NumberMatched != 3 {
		t.Fatalf("absence/exact count=%v %+v", ids, result)
	}
	_, args, _, err := featureCandidatePredicate(p.layers["items"], q)
	if err != nil || len(args) != 4 {
		t.Fatalf("spatial bounds must be bound params: %v %v", args, err)
	}
}

func TestFeatureProfileAndTemporalRegistration(t *testing.T) {
	for _, tc := range []struct {
		name        string
		keys        map[string]interface{}
		unsupported bool
	}{
		{"partial", map[string]interface{}{"temporal_start_field": "at", "temporal_storage": "unix_seconds"}, false},
		{"missing", map[string]interface{}{"temporal_field": "missing", "temporal_storage": "unix_seconds"}, false},
		{"same physical field", map[string]interface{}{"temporal_start_field": "at", "temporal_end_field": "AT", "temporal_storage": "unix_seconds"}, false},
		{"storage without fields", map[string]interface{}{"temporal_storage": "unix_seconds"}, false},
		{"unknown storage", map[string]interface{}{"temporal_field": "at", "temporal_storage": "datetime"}, true},
		{"custom temporal unsupported", map[string]interface{}{"sql": "SELECT id,geom,at FROM items", "temporal_field": "at", "temporal_storage": "unix_seconds"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.gpkg")
			db, err := sql.Open("sqlite3", path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec("CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT,at INTEGER)"); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			conf := map[string]interface{}{"name": "items", "tablename": "items", "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 4326}
			for k, v := range tc.keys {
				conf[k] = v
			}
			if _, ok := tc.keys["sql"]; ok {
				delete(conf, "tablename")
			}
			_, err = NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]interface{}{conf}}, nil)
			if tc.unsupported {
				if !errors.Is(err, provider.ErrUnsupported) {
					t.Fatalf("unsupported error=%v", err)
				}
			} else {
				var invalid provider.InvalidFeatureQueryError
				if !errors.As(err, &invalid) {
					t.Fatalf("typed error=%v", err)
				}
			}
		})
	}
	p, _ := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT)", map[string]interface{}{"sql": "SELECT id,geom FROM items"})
	if !errors.Is(p.layers["items"].FeatureQuerySupported(), provider.ErrUnsupported) {
		t.Fatal("custom SQL feature capability")
	}
	if _, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { return nil }); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatalf("custom SQL query=%v", err)
	}
}

func TestFeatureUnsupportedTableIdentity(t *testing.T) {
	p, _ := queryTestProvider(t, "CREATE TABLE items(id INTEGER,geom TEXT)", nil)
	if !errors.Is(p.layers["items"].FeatureQuerySupported(), provider.ErrUnsupported) {
		t.Fatal("nonunique identity admitted")
	}
	_, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { return nil })
	if !errors.Is(err, provider.ErrUnsupported) {
		t.Fatalf("unsupported query=%v", err)
	}
}

func TestFeatureWKBEmptyContainersWithMisleadingBounds(t *testing.T) {
	p, db := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,minx REAL,maxx REAL,miny REAL,maxy REAL)", map[string]interface{}{"geometry_format": "wkb"})
	for i, g := range []geom.Geometry{geom.Collection{geom.Collection{}}, geom.MultiLineString{[][2]float64{}}, geom.Polygon{}} {
		blob, err := wkb.EncodeBytes(g)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("INSERT INTO items VALUES (?,?,999,999,999,999)", i+1, blob); err != nil {
			t.Fatal(err)
		}
	}
	for i, g := range []geom.Geometry{geom.Polygon{{{100, 100}, {101, 100}, {101, 101}}}, geom.Collection{geom.Point{100, 100}}} {
		blob, err := wkb.EncodeBytes(g)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("INSERT INTO items VALUES (?,?,999,999,999,999)", i+4, blob); err != nil {
			t.Fatal(err)
		}
	}
	ids, _ := queryIDs(t, p, provider.FeatureQuery{Limit: 10, Bounds: []geom.Extent{{0, 0, 0, 0}}, BoundsSRID: 4326})
	if !reflect.DeepEqual(ids, []uint64{1, 2, 3}) {
		t.Fatalf("empty containers dropped by coarse bounds: %v", ids)
	}
	where, args, _, err := featureCandidatePredicate(p.layers["items"], provider.FeatureQuery{Limit: 10, Bounds: []geom.Extent{{0, 0, 0, 0}}, BoundsSRID: 4326})
	if err != nil {
		t.Fatal(err)
	}
	var candidateCount int
	if err := db.QueryRow("SELECT count(*) FROM items l WHERE "+where, args...).Scan(&candidateCount); err != nil {
		t.Fatal(err)
	}
	if candidateCount != 5 {
		t.Fatalf("conservative aggregate inspection limitation changed: %d", candidateCount)
	}
}

func TestFeatureMOSPointAndStrictMalformedContainer(t *testing.T) {
	p, db := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,minx REAL,maxx REAL,miny REAL,maxy REAL)", map[string]interface{}{"geometry_format": "mos", "mos_precision": 2.0})
	point := []byte{2, 0, 0, 0, 1, 0, 1, 0, 0, 0, 1, 0, 0, 0}
	point = binary.LittleEndian.AppendUint32(point, 100)
	point = binary.LittleEndian.AppendUint32(point, 200)
	if _, err := db.Exec("INSERT INTO items VALUES (1,?,100,100,200,200)", point); err != nil {
		t.Fatal(err)
	}
	ids, _ := queryIDs(t, p, provider.FeatureQuery{Limit: 10, Bounds: []geom.Extent{{1, 2, 1, 2}}, BoundsSRID: 4326})
	if !reflect.DeepEqual(ids, []uint64{1}) {
		t.Fatalf("MOS quantized bbox=%v", ids)
	}
	// Empty MOS containers are malformed in the shared decoder, not an
	// excuse for silent omission when their stored bounds do not intersect.
	if _, err := db.Exec("INSERT INTO items VALUES (2,?,999,999,999,999)", []byte{1, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	_, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10, Bounds: []geom.Extent{{1, 2, 1, 2}}, BoundsSRID: 4326}, func(*provider.Feature) error { return nil })
	if err == nil {
		t.Fatal("malformed conservative MOS candidate silently discarded")
	}
}

func TestFeatureNativeRTreeAbsenceAndIndexBranches(t *testing.T) {
	p, db := queryTestProvider(t, `CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB);
CREATE TABLE gpkg_contents(table_name TEXT,data_type TEXT,min_x REAL,min_y REAL,max_x REAL,max_y REAL,srs_id INTEGER);
CREATE TABLE gpkg_geometry_columns(table_name TEXT,column_name TEXT,geometry_type_name TEXT,srs_id INTEGER,z INTEGER,m INTEGER);
INSERT INTO gpkg_contents VALUES ('items','features',-180,-90,180,90,4326);
INSERT INTO gpkg_geometry_columns VALUES ('items','geom','POINT',4326,0,0);
CREATE VIRTUAL TABLE rtree_items_geom USING rtree(id,minx,maxx,miny,maxy);`, map[string]interface{}{"geometry_format": "gpkg"})
	point, err := wkb.EncodeBytes(geom.Point{0, 0})
	if err != nil {
		t.Fatal(err)
	}
	header := binary.LittleEndian.AppendUint32([]byte{'G', 'P', 0, 1}, 4326)
	emptyHeader := binary.LittleEndian.AppendUint32([]byte{'G', 'P', 0, 17}, 4326)
	emptyPoint := binary.LittleEndian.AppendUint32([]byte{1}, 1)
	emptyPoint = binary.LittleEndian.AppendUint64(emptyPoint, math.Float64bits(math.NaN()))
	emptyPoint = binary.LittleEndian.AppendUint64(emptyPoint, math.Float64bits(math.NaN()))
	emptyCollection, err := wkb.EncodeBytes(geom.Collection{geom.Collection{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		id      int
		blob    []byte
		indexed bool
	}{
		{1, append(append([]byte{}, header...), point...), true},
		{2, append(append([]byte{}, emptyHeader...), emptyPoint...), true},
		{3, append(append([]byte{}, emptyHeader...), emptyCollection...), true},
		{4, nil, true},
		{5, append(append([]byte{}, header...), point...), false},
	} {
		if _, err := db.Exec("INSERT INTO items VALUES (?,?)", row.id, row.blob); err != nil {
			t.Fatal(err)
		}
		if row.indexed {
			bounds := 999
			if row.id == 1 {
				bounds = 0
			}
			if _, err := db.Exec("INSERT INTO rtree_items_geom VALUES (?,?,?,?,?)", row.id, bounds, bounds, bounds, bounds); err != nil {
				t.Fatal(err)
			}
		}
	}
	layer := p.layers["items"]
	q := provider.FeatureQuery{Limit: 10, Bounds: []geom.Extent{{0, 0, 0, 0}}, BoundsSRID: 4326}
	ids, _ := queryIDs(t, p, q)
	if !reflect.DeepEqual(ids, []uint64{1, 2, 3, 4, 5}) {
		t.Fatalf("native absence/indexless coverage=%v", ids)
	}
	where, args, _, err := featureCandidatePredicate(layer, q)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query("EXPLAIN QUERY PLAN SELECT l.id FROM items l WHERE "+where, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Error(err)
		}
	}()
	var details []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	plan := strings.Join(details, "\n")
	t.Log(plan)
	if !strings.Contains(plan, "VIRTUAL TABLE INDEX") || !strings.Contains(plan, "UNION") {
		t.Fatalf("indexed branch absent: %s", plan)
	}
}

func TestFeatureRestrictedDomainBoundsCRS(t *testing.T) {
	p, db := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT)", nil)
	if _, err := db.Exec("INSERT INTO items VALUES (1,'POINT (151 -33)')"); err != nil {
		t.Fatal(err)
	}
	target, err := basic.RegisterProj4Defn("+proj=etmerc +lon_0=151 +lat_0=0 +ellps=WGS84 +units=m")
	if err != nil {
		t.Fatal(err)
	}
	projected, err := transformQueryGeometry(geom.Point{151, -33}, 4326, target)
	if err != nil {
		t.Fatal(err)
	}
	point := projected.(geom.Point)
	ids, _ := queryIDs(t, p, provider.FeatureQuery{Limit: 10, Bounds: []geom.Extent{{point[0] - 1, point[1] - 1, point[0] + 1, point[1] + 1}}, BoundsSRID: target})
	if !reflect.DeepEqual(ids, []uint64{1}) {
		t.Fatalf("valid local projection rejected: %v", ids)
	}
	_, err = p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10, Bounds: []geom.Extent{{0, 0, 1, 1}}, BoundsSRID: 987654321}, func(*provider.Feature) error { return nil })
	if !errors.Is(err, provider.ErrUnsupported) {
		t.Fatalf("unknown target CRS did not fail closed: %v", err)
	}
}
