//go:build cgo

package gpkg

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider"
)

func TestQueryAcceptanceTemporalStoragePrecision(t *testing.T) {
	profiles := []struct {
		name  string
		scale int64
	}{{"unix_seconds", 1}, {"unix_milliseconds", 1000}, {"unix_microseconds", 1000000}, {"unix_nanoseconds", 1000000000}}
	for _, profile := range profiles {
		t.Run(profile.name, func(t *testing.T) {
			p, _ := acceptanceProvider(t, "wkb", false, map[string]any{"temporal_field": "ts", "temporal_storage": profile.name}, [][]any{
				acceptanceRow(10, acceptanceWKB(t, geom.Point{0, 0}), int64(-1), nil, [4]float64{}),
				acceptanceRow(20, acceptanceWKB(t, geom.Point{0, 0}), int64(0), nil, [4]float64{}),
				acceptanceRow(30, nil, nil, nil, [4]float64{}),
			})
			cases := []struct {
				name    string
				instant time.Time
				ids     []uint64
			}{
				{"epoch", time.Unix(0, 0), []uint64{20, 30}},
				{"beforeepoch1ns", time.Unix(0, -1), []uint64{30}},
				{"afterepoch1ns", time.Unix(0, 1), []uint64{30}},
				{"negativeunit", time.Unix(-1, 1000000000-1000000000/profile.scale), []uint64{10, 30}},
			}
			if profile.scale == 1000000000 {
				cases[1].ids = []uint64{10, 30}
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					instant := tc.instant
					ids, _, err := acceptanceIDs(p, provider.FeatureQuery{Limit: 10, Temporal: &provider.TemporalConstraint{Start: &instant, End: &instant}})
					if err != nil || !reflect.DeepEqual(ids, tc.ids) {
						t.Fatalf("got %v want %v: %v", ids, tc.ids, err)
					}
				})
			}
		})
	}
}

func TestQueryAcceptanceTemporalNULLInvalidAndRange(t *testing.T) {
	p, db := acceptanceProvider(t, "wkb", false, map[string]any{"temporal_start_field": "ts", "temporal_end_field": "te", "temporal_storage": "unix_seconds"}, [][]any{
		acceptanceRow(10, nil, int64(-1), int64(1), [4]float64{}),
		acceptanceRow(20, nil, nil, int64(0), [4]float64{}),
		acceptanceRow(30, nil, int64(0), nil, [4]float64{}),
		acceptanceRow(40, nil, nil, nil, [4]float64{}),
	})
	instant := time.Unix(0, 1)
	ids, _, err := acceptanceIDs(p, provider.FeatureQuery{Limit: 10, Temporal: &provider.TemporalConstraint{Start: &instant, End: &instant}})
	if err != nil || !reflect.DeepEqual(ids, []uint64{10, 30, 40}) {
		t.Fatalf("subunit interval overlap/null: %v %v", ids, err)
	}
	far := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	ids, _, err = acceptanceIDs(p, provider.FeatureQuery{Limit: 10, Temporal: &provider.TemporalConstraint{Start: &far, End: &far}})
	if err != nil || !reflect.DeepEqual(ids, []uint64{30, 40}) {
		t.Fatalf("far bound: %v %v", ids, err)
	}
	for _, bad := range []struct {
		name       string
		start, end any
	}{{"TEXT", "invalid", int64(2)}, {"REAL", 0.5, int64(2)}, {"reversed", int64(2), int64(-2)}} {
		t.Run(bad.name, func(t *testing.T) {
			if _, err := db.Exec("INSERT INTO items(id,geom,name,ts,te) VALUES(99,NULL,'bad',?,?)", bad.start, bad.end); err != nil {
				t.Fatal(err)
			}
			_, _, err := acceptanceIDs(p, provider.FeatureQuery{Limit: 100, Temporal: &provider.TemporalConstraint{Start: &far, End: &far}})
			var invalid provider.InvalidFeatureQueryError
			if !errors.As(err, &invalid) {
				t.Fatalf("badsource silently filtered: %v", err)
			}
			if _, err := db.Exec("DELETE FROM items WHERE id=99"); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("nanosecondint64extremes", func(t *testing.T) {
		p, _ := acceptanceProvider(t, "wkb", false, map[string]any{"temporal_field": "ts", "temporal_storage": "unix_nanoseconds"}, [][]any{
			acceptanceRow(10, nil, int64(math.MinInt64), nil, [4]float64{}), acceptanceRow(20, nil, int64(math.MaxInt64), nil, [4]float64{}), acceptanceRow(30, nil, nil, nil, [4]float64{}),
		})
		for _, tc := range []struct {
			instant time.Time
			ids     []uint64
		}{{time.Unix(0, math.MinInt64), []uint64{10, 30}}, {time.Unix(0, math.MaxInt64), []uint64{20, 30}}, {far, []uint64{30}}} {
			instant := tc.instant
			ids, _, err := acceptanceIDs(p, provider.FeatureQuery{Limit: 10, Temporal: &provider.TemporalConstraint{Start: &instant, End: &instant}})
			if err != nil || !reflect.DeepEqual(ids, tc.ids) {
				t.Fatalf("int64extreme: %v want %v: %v", ids, tc.ids, err)
			}
		}
	})
}

func TestQueryAcceptanceExactPagingAbsenceAndNativeIndex(t *testing.T) {
	rows := [][]any{
		acceptanceRow(10, acceptanceNative(t, geom.LineString{{0, 0}, {2, 0}}, false), nil, nil, [4]float64{0, 2, 0, 2}),
		acceptanceRow(20, acceptanceNative(t, geom.LineString{{0, 2}, {2, 2}}, false), nil, nil, [4]float64{0, 2, 0, 2}),
		acceptanceRow(30, acceptanceNative(t, geom.Point{1, 1}, false), nil, nil, [4]float64{1, 1, 1, 1}),
		acceptanceRow(40, nil, nil, nil, [4]float64{100, 100, 100, 100}),
		acceptanceRow(50, acceptanceNative(t, nil, true), nil, nil, [4]float64{100, 100, 100, 100}),
	}
	for id := int64(100); id < 2100; id++ {
		rows = append(rows, acceptanceRow(id, acceptanceNative(t, geom.Point{80, 40}, false), nil, nil, [4]float64{80, 80, 40, 40}))
	}
	// Valid native Point type and full payload classify this nonfinite geometry as
	// nonempty; outside ordinary indexed bounds it must not be decoded.
	rows = append(rows, acceptanceRow(9999, acceptanceNative(t, geom.Point{math.Inf(1), 40}, false), nil, nil, [4]float64{80, 80, 40, 40}))
	p, db := acceptanceProvider(t, "gpkg", true, nil, rows)
	if _, err := db.Exec("DELETE FROM rtree_items_geom WHERE id=40"); err != nil {
		t.Fatal(err)
	}
	q := provider.FeatureQuery{Limit: 1, Offset: 1, Bounds: []geom.Extent{{1, 1, 1, 1}, {1, 1, 1, 1}}, BoundsSRID: 4326}
	ids, result, err := acceptanceIDs(p, q)
	if err != nil || !reflect.DeepEqual(ids, []uint64{40}) || !result.HasMore {
		t.Fatalf("exact pagingafterfalsepositives/union: %v %+v %v", ids, result, err)
	}
	q.Offset = 2
	ids, result, err = acceptanceIDs(p, q)
	if err != nil || !reflect.DeepEqual(ids, []uint64{50}) || result.HasMore {
		t.Fatalf("empty geometrywithmisleadingbounds: %v %+v %v", ids, result, err)
	}
	q.Offset = 0
	q.Limit = 10
	where, args, impossible, err := featureCandidatePredicate(p.layers["source"], q)
	if err != nil {
		t.Fatal(err)
	}
	if impossible {
		t.Fatal("validquery declared impossible")
	}
	statement := "SELECT l.id FROM items l WHERE " + where + " ORDER BY l.id LIMIT 256"

	details := func() []string {
		planRows, err := db.Query("EXPLAIN QUERY PLAN "+statement, args...)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := planRows.Close(); err != nil {
				t.Error(err)
			}
		}()
		details := []string{}
		for planRows.Next() {
			var id, parent, unused int
			var detail string
			if err := planRows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			details = append(details, detail)
		}
		if err := planRows.Err(); err != nil {
			t.Fatal(err)
		}
		return details
	}()
	plan := strings.Join(details, "\n")
	t.Logf("real native candidate plan (including absence/indexless branches):\n%s", plan)
	if !strings.Contains(plan, "VIRTUAL TABLE INDEX") {
		t.Fatalf("RTree index not used: %s", plan)
	}
	if strings.Contains(plan, "SCAN l") {
		t.Log("LIMIT bounds returned candidates; absence/indexless source scan remains alongside index-driven candidate branches; not a no-full-source-scan performance PASS")
	}

	count := func() int {
		candidateRows, err := db.Query(statement, args...)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := candidateRows.Close(); err != nil {
				t.Error(err)
			}
		}()
		count := 0
		for candidateRows.Next() {
			count++
		}
		if err := candidateRows.Err(); err != nil {
			t.Fatal(err)
		}
		return count
	}()
	if count != 5 {
		t.Fatalf("nativeindexfailedpruningordinaryoutside/sentinel: candidates=%d want5", count)
	}
	// The same classified row selected by its ID/bounds must fail strict geometry validation.
	_, _, invalidGeometryErr := acceptanceIDs(p, provider.FeatureQuery{Limit: 10, IDs: []uint64{9999}, Bounds: []geom.Extent{{79, 39, 81, 41}}, BoundsSRID: 4326})
	if invalidGeometryErr == nil {
		t.Fatal("inside/ID-selected nonfinite Point accepted")
	}
	q = provider.FeatureQuery{Limit: 1, IDs: []uint64{30}}
	ids, _, err = acceptanceIDs(p, q)
	if err != nil || !reflect.DeepEqual(ids, []uint64{30}) {
		t.Fatalf("ID pushdowndecodedoutsidemalformed: %v %v", ids, err)
	}
}

func TestQueryAcceptanceRawAggregateAbsenceAndCrossCRS(t *testing.T) {
	basic.RegisterBuiltinProj4SRIDs()
	p, _ := acceptanceProvider(t, "wkb", false, nil, [][]any{
		acceptanceRow(10, acceptanceWKB(t, geom.Collection{geom.Collection{}}), nil, nil, [4]float64{100, 100, 100, 100}),
		acceptanceRow(20, acceptanceWKB(t, geom.Point{15, 30}), nil, nil, [4]float64{15, 15, 30, 30}),
	})
	ids, _, err := acceptanceIDs(p, provider.FeatureQuery{Limit: 10, Bounds: []geom.Extent{{15, 30, 15, 30}}, BoundsSRID: 4326})
	if err != nil || !reflect.DeepEqual(ids, []uint64{10, 20}) {
		t.Fatalf("rawaggregate absencecoverage: %v %v", ids, err)
	}
	x, y := 1669792.3618991037, 3503549.843504374
	ids, _, err = acceptanceIDs(p, provider.FeatureQuery{Limit: 10, Bounds: []geom.Extent{{x - 1, y - 1, x + 1, y + 1}}, BoundsSRID: 3857})
	if err != nil || !reflect.DeepEqual(ids, []uint64{10, 20}) {
		t.Fatalf("crossCRScorrectnessfallback: %v %v", ids, err)
	}
	t.Log("Cross-CRS and unclassified aggregate fallback correctness only; no indexed-pruning claim.")
}

func TestQueryAcceptanceMOSMalformedContainerAndNull(t *testing.T) {
	// The strict shared MOS decoder rejects a zero-point Polyline as malformed,
	// not decoded empty. Conservative candidate selection must expose the error
	// even with misleading quantized bounds. SQL NULL is valid absence instead.
	empty := []byte{mos.TypePolyline, 0}
	empty = binary.LittleEndian.AppendUint16(empty, 0)
	empty = binary.LittleEndian.AppendUint16(empty, 1)
	empty = binary.LittleEndian.AppendUint32(empty, 0)
	empty = binary.LittleEndian.AppendUint32(empty, 0)
	p, _ := acceptanceProvider(t, "mos", false, map[string]any{"mos_precision": 2, "mos_units": "m"}, [][]any{acceptanceRow(10, empty, nil, nil, [4]float64{10000, 10000, 10000, 10000})})
	q := provider.FeatureQuery{Limit: 10, Bounds: []geom.Extent{{1, 1, 2, 2}}, BoundsSRID: 4326}
	ids, _, err := acceptanceIDs(p, q)
	if err == nil || len(ids) != 0 {
		t.Fatalf("malformed MOS container accepted/emitted: %v %v", ids, err)
	}
	p, _ = acceptanceProvider(t, "mos", false, map[string]any{"mos_precision": 2, "mos_units": "m"}, [][]any{acceptanceRow(10, nil, nil, nil, [4]float64{10000, 10000, 10000, 10000})})
	ids, _, err = acceptanceIDs(p, q)
	if err != nil || !reflect.DeepEqual(ids, []uint64{10}) {
		t.Fatalf("valid-ID MOS SQLNULL lost: %v %v", ids, err)
	}
	t.Log("Malformed MOS zero-point container errors; SQL NULL is absence. No universal indexed-pruning claim.")

}

func TestQueryAcceptanceCancelAndCallback(t *testing.T) {
	p, _ := acceptanceProvider(t, "wkb", false, nil, [][]any{acceptanceRow(10, nil, nil, nil, [4]float64{}), acceptanceRow(20, nil, nil, nil, [4]float64{})})
	ctx, cancel := context.WithCancel(context.Background())
	deliveries := 0
	_, err := p.QueryFeatures(ctx, "source", provider.FeatureQuery{Limit: 10}, func(*provider.Feature) error { deliveries++; cancel(); return nil })
	if !errors.Is(err, context.Canceled) || deliveries != 1 {
		t.Fatalf("midcancel: %d %v", deliveries, err)
	}
	sentinel := errors.New("acceptancecallback")
	deliveries = 0
	_, err = p.QueryFeatures(context.Background(), "source", provider.FeatureQuery{Limit: 10}, func(*provider.Feature) error { deliveries++; return fmt.Errorf("callback: %w", sentinel) })
	if !errors.Is(err, sentinel) || deliveries != 1 {
		t.Fatalf("callbackstop/chain: %d %v", deliveries, err)
	}
}

func acceptanceProvider(t *testing.T, format string, rtree bool, extra map[string]any, rows [][]any) (*Provider, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "qa.gpkg")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	ddl := []string{"CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,name TEXT,ts INTEGER,te INTEGER,minx REAL,maxx REAL,miny REAL,maxy REAL)", "CREATE TABLE gpkg_contents(table_name TEXT,data_type TEXT,min_x REAL,min_y REAL,max_x REAL,max_y REAL,srs_id INTEGER)", "CREATE TABLE gpkg_geometry_columns(table_name TEXT,column_name TEXT,geometry_type_name TEXT,srs_id INTEGER,z INTEGER,m INTEGER)"}
	if format == "gpkg" {
		ddl = append(ddl, "INSERT INTO gpkg_contents VALUES('items','features',-180,-90,180,90,4326)", "INSERT INTO gpkg_geometry_columns VALUES('items','geom','GEOMETRY',4326,0,0)")
	}
	if rtree {
		ddl = append(ddl, "CREATE VIRTUAL TABLE rtree_items_geom USING rtree(id,minx,maxx,miny,maxy)")
	}
	for _, statement := range ddl {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range rows {
		if _, err := db.Exec("INSERT INTO items VALUES(?,?,?,?,?,?,?,?,?)", row...); err != nil {
			t.Fatal(err)
		}
		if rtree {
			if _, err := db.Exec("INSERT INTO rtree_items_geom VALUES(?,?,?,?,?)", row[0], row[5], row[6], row[7], row[8]); err != nil {
				t.Fatal(err)
			}
		}
	}
	config := map[string]any{"name": "source", "tablename": "items", "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": format, "geometry_type": "point", "srid": 4326, "fields": []string{"name"}}
	for key, value := range extra {
		config[key] = value
	}
	tiler, err := NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]any{config}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := tiler.(*Provider)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	})
	return p, db
}
func acceptanceRow(id int64, g, start, end any, b [4]float64) []any {
	return []any{id, g, fmt.Sprintf("row%d", id), start, end, b[0], b[1], b[2], b[3]}
}
func acceptanceIDs(p *Provider, q provider.FeatureQuery) ([]uint64, provider.FeatureQueryResult, error) {
	ids := []uint64{}
	result, err := p.QueryFeatures(context.Background(), "source", q, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
	return ids, result, err
}
func acceptanceWKB(t *testing.T, g geom.Geometry) []byte {
	t.Helper()
	blob, err := wkb.EncodeBytes(g)
	if err != nil {
		t.Fatal(err)
	}
	return blob
}
func acceptanceHeader(empty bool) []byte {
	flag := byte(1)
	if empty {
		flag |= 16
	}
	return binary.LittleEndian.AppendUint32([]byte{'G', 'P', 0, flag}, 4326)
}
func acceptanceNative(t *testing.T, g geom.Geometry, empty bool) []byte {
	t.Helper()
	header := acceptanceHeader(empty)
	if empty {
		body := binary.LittleEndian.AppendUint32([]byte{1}, 1)
		body = binary.LittleEndian.AppendUint64(body, math.Float64bits(math.NaN()))
		body = binary.LittleEndian.AppendUint64(body, math.Float64bits(math.NaN()))
		return append(header, body...)
	}
	return append(header, acceptanceWKB(t, g)...)
}

func TestQueryAcceptanceNativeHeaderOnlyRawStrict(t *testing.T) {
	header := acceptanceHeader(true)
	p, _ := acceptanceProvider(t, "gpkg", false, nil, [][]any{acceptanceRow(10, header, nil, nil, [4]float64{100, 100, 100, 100})})
	ids, _, err := acceptanceIDs(p, provider.FeatureQuery{Limit: 10, Bounds: []geom.Extent{{0, 0, 1, 1}}, BoundsSRID: 4326})
	if err == nil || len(ids) != 0 {
		t.Fatalf("header-only raw source must fail: %v %v", ids, err)
	}
	_, legacy, err := decodeGeometryValue(header, GeometryFormatGPKG, p.layers["source"].mosConfig)
	if err != nil || legacy != nil {
		t.Fatalf("legacy tile decode changed: %v %v", legacy, err)
	}
}
