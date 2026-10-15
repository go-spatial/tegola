//go:build cgo

// Command features measures a synthetic local GeoPackage feature-query profile.
package main

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	metricprovider "github.com/alexeydott/tegola/observability/prometheus"
	"github.com/alexeydott/tegola/ogc/cql2"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/gpkg"
)

var dataDir string
var observed bool
var reference = map[string]measurement{}

type measurement struct {
	Dataset    int     `json:"dataset"`
	Class      string  `json:"class"`
	Median     float64 `json:"p50_ms"`
	Maximum    float64 `json:"sample_max_ms"`
	Allocation uint64  `json:"mean_alloc_bytes"`
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func wire(x, y float64) []byte {
	b := []byte{'G', 'P', 0, 1}
	b = binary.LittleEndian.AppendUint32(b, 4326)
	b = append(b, 1)
	b = binary.LittleEndian.AppendUint32(b, 1)
	b = binary.LittleEndian.AppendUint64(b, math.Float64bits(x))
	return binary.LittleEndian.AppendUint64(b, math.Float64bits(y))
}
func main() {
	flag.StringVar(&dataDir, "data-dir", "", "Owned synthetic dataset directory (required); existing files are read-only")
	flag.BoolVar(&observed, "observe", false, "Enable the actual instance-private Prometheus query observer")
	referencePath := flag.String("reference", "", "Approved JSONL reference; enforce timing/allocation gates when set")
	sizes := flag.String("sizes", "10000,100000,1000000,10000000", "Subset of the four fixed dataset sizes")
	flag.Parse()
	if dataDir == "" {
		panic("data-dir is required")
	}
	must(os.MkdirAll(dataDir, 0700))
	if *referencePath != "" {
		file, err := os.Open(*referencePath)
		must(err)
		decoder := json.NewDecoder(file)
		for decoder.More() {
			var value measurement
			must(decoder.Decode(&value))
			reference[key(value.Dataset, value.Class)] = value
		}
		must(file.Close())
		if len(reference) == 0 {
			panic("approved reference is empty")
		}
	}
	enc := json.NewEncoder(os.Stdout)
	for _, text := range strings.Split(*sizes, ",") {
		n, err := strconv.Atoi(text)
		must(err)
		if n != 10000 && n != 100000 && n != 1000000 && n != 10000000 {
			panic("unsupported dataset size")
		}
		run(n, enc)
	}
}
func run(n int, enc *json.Encoder) {
	path := filepath.Join(dataDir, fmt.Sprintf("synthetic-%d.gpkg", n))
	if _, err := os.Stat(path); os.IsNotExist(err) {
		func() {
			db, err := sql.Open("sqlite3", path)
			must(err)
			defer func() { must(db.Close()) }()
			for _, s := range []string{"CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,n INTEGER,s TEXT,at INTEGER)", "CREATE VIRTUAL TABLE rtree_items_geom USING rtree(id,minx,maxx,miny,maxy)", "CREATE INDEX items_at ON items(at)", "CREATE INDEX items_n ON items(n)", "CREATE TABLE gpkg_contents(table_name TEXT,data_type TEXT,min_x REAL,min_y REAL,max_x REAL,max_y REAL,srs_id INTEGER)", "CREATE TABLE gpkg_geometry_columns(table_name TEXT,column_name TEXT,geometry_type_name TEXT,srs_id INTEGER,z INTEGER,m INTEGER)", "INSERT INTO gpkg_contents VALUES('items','features',0,0,100,50,4326)", "INSERT INTO gpkg_geometry_columns VALUES('items','geom','POINT',4326,0,0)"} {
				_, err = db.Exec(s)
				must(err)
			}
			tx, err := db.Begin()
			must(err)
			defer func() {
				if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
					must(err)
				}
			}()
			row, err := tx.Prepare("INSERT INTO items VALUES(?,?,?,?,?)")
			must(err)
			defer func() { must(row.Close()) }()
			idx, err := tx.Prepare("INSERT INTO rtree_items_geom VALUES(?,?,?,?,?)")
			must(err)
			defer func() { must(idx.Close()) }()
			for i := 1; i <= n; i++ {
				x := float64((i-1)%1000) / 10
				y := float64(((i-1)/1000)%500) / 10
				_, err = row.Exec(i, wire(x, y), i%100, "value", 1700000000+i)
				must(err)
				_, err = idx.Exec(i, x, x, y, y)
				must(err)
			}
			must(tx.Commit())
		}()
	}
	tiler, err := gpkg.NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]any{{"name": "items", "tablename": "items", "id_fieldname": "id", "geometry_fieldname": "geom", "srid": 4326, "fields": []string{"n", "s"}, "temporal_field": "at", "temporal_storage": "unix_seconds"}}}, nil)
	must(err)
	p := tiler.(*gpkg.Provider)
	defer func() { must(p.Close()) }()
	layers, err := p.Layers()
	must(err)
	s, err := features.NewService([]features.CollectionSource{{ID: "items", Layer: layers[0], Querier: p}})
	must(err)
	if observed {
		observer, err := metricprovider.New(dict.Dict{})
		must(err)
		s = s.WithQueryObserver(observer.(features.QueryObserver))
	}
	filter, err := cql2.Parse("n = 7")
	must(err)
	instant := time.Unix(1700000000+int64(n/2), 0)
	type scenario struct {
		name    string
		q       provider.FeatureQuery
		options features.QueryOptions
	}
	cases := []scenario{{"id", provider.FeatureQuery{Limit: 10, IDs: []uint64{uint64(n / 2)}}, features.QueryOptions{}}, {"bbox_selective", provider.FeatureQuery{Limit: 10, Bounds: []geom.Extent{{10, 0.2, 10.2, 0.4}}, BoundsSRID: 4326}, features.QueryOptions{}}, {"bbox_broad", provider.FeatureQuery{Limit: 10, Bounds: []geom.Extent{{0, 0, 100, 50}}, BoundsSRID: 4326}, features.QueryOptions{}}, {"limit_10", provider.FeatureQuery{Limit: 10}, features.QueryOptions{}}, {"limit_100", provider.FeatureQuery{Limit: 100}, features.QueryOptions{}}, {"limit_max_1000", provider.FeatureQuery{Limit: 1000}, features.QueryOptions{}}, {"datetime", provider.FeatureQuery{Limit: 10, Temporal: &provider.TemporalConstraint{Start: &instant, End: &instant}}, features.QueryOptions{}}, {"property_filter", provider.FeatureQuery{Limit: 10, Filter: &filter}, features.QueryOptions{}}, {"crs_3857", provider.FeatureQuery{Limit: 10}, features.QueryOptions{OutputCRS: "http://www.opengis.net/def/crs/EPSG/0/3857"}}}
	cases = append(cases, scenario{"cql2_parse_query", provider.FeatureQuery{Limit: 10, Filter: &filter}, features.QueryOptions{}})
	for _, c := range cases {
		expected, total := expectedIDs(n, c.name, int(c.q.Limit))
		var times []float64
		var alloc uint64
		var count uint64
		var matched *uint64
		for i := 0; i < 4; i++ {
			var before, after runtime.MemStats
			batch := 1
			if c.name == "id" || c.name == "limit_10" || c.name == "limit_100" || c.name == "crs_3857" {
				batch = 20
			}
			pages := make([]features.FeatureCollection, batch)
			runtime.ReadMemStats(&before)
			start := time.Now()
			var page features.FeatureCollection
			for j := 0; j < batch; j++ {
				q := c.q
				if c.name == "cql2_parse_query" {
					parsed, e := cql2.Parse("n = 7")
					must(e)
					q.Filter = &parsed
				}
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				page, err = s.QueryCollectionPageWithOptions(ctx, "items", q, c.options)
				cancel()
				must(err)
				pages[j] = page
			}
			elapsed := time.Since(start) / time.Duration(batch)
			runtime.ReadMemStats(&after)
			for _, page := range pages {
				assertPage(page, expected, total, c.name)
			}
			if i > 0 {
				times = append(times, float64(elapsed.Nanoseconds())/1e6)
				alloc += (after.TotalAlloc - before.TotalAlloc) / uint64(batch)
			}
			count = page.NumberReturned
			matched = page.NumberMatched
		}
		sort.Float64s(times)
		value := measurement{Dataset: n, Class: c.name, Median: times[1], Maximum: times[2], Allocation: alloc / 3}
		must(enc.Encode(map[string]any{"dataset": n, "class": c.name, "warmups": 1, "samples": 3, "p50_ms": times[1], "sample_max_ms": times[2], "raw_ms": times, "mean_alloc_bytes": alloc / 3, "returned": count, "matched": matched, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "observer": observed, "semantic_oracle": "analytic_grid"}))
		if len(reference) != 0 {
			must(enforce(value, reference))
		}
	}
}

func key(n int, class string) string { return strconv.Itoa(n) + ":" + class }

// expectedIDs is an independent analytic model of the fixture, not a provider result.
func expectedIDs(n int, class string, limit int) ([]uint64, uint64) {
	ids := make([]uint64, 0, limit)
	var total uint64
	for i := 1; i <= n; i++ {
		match := true
		switch class {
		case "id", "datetime":
			match = i == n/2
		case "bbox_selective":
			x := (i - 1) % 1000
			y := ((i - 1) / 1000) % 500
			match = x >= 100 && x <= 102 && y >= 2 && y <= 4
		case "property_filter", "cql2_parse_query":
			match = i%100 == 7
		}
		if match {
			total++
			if len(ids) < limit {
				ids = append(ids, uint64(i))
			}
		}
	}
	return ids, total
}

func assertPage(page features.FeatureCollection, ids []uint64, total uint64, class string) {
	if page.Type != "FeatureCollection" || page.NumberReturned != uint64(len(ids)) || len(page.Features) != len(ids) || page.HasMore != (total > uint64(len(ids))) {
		panic("independent paging oracle failed")
	}
	if page.HasMore {
		if page.NumberMatched != nil {
			panic("expected unknown count after lookahead")
		}
	} else if page.NumberMatched == nil || *page.NumberMatched != total {
		panic("independent exact count oracle failed")
	}
	for index, feature := range page.Features {
		id := ids[index]
		if feature.ID != id || feature.Type != "Feature" {
			panic("independent ID oracle failed")
		}
		x := float64((id-1)%1000) / 10
		y := float64(((id-1)/1000)%500) / 10
		if class == "crs_3857" {
			x *= 111319.49079327357
			y = 6378137 * math.Log(math.Tan(math.Pi/4+y*math.Pi/360))
		}
		var geometry struct {
			Type        string    `json:"type"`
			Coordinates []float64 `json:"coordinates"`
		}
		must(json.Unmarshal(feature.Geometry, &geometry))
		if geometry.Type != "Point" || len(geometry.Coordinates) != 2 || math.Abs(geometry.Coordinates[0]-x) > 1e-8 || math.Abs(geometry.Coordinates[1]-y) > 1e-8 {
			panic("independent geometry oracle failed")
		}
		properties := map[string]any{"n": json.Number(strconv.FormatUint(id%100, 10)), "s": "value"}
		if !reflect.DeepEqual(feature.Properties, properties) {
			panic("independent property oracle failed")
		}
	}
}

func enforce(actual measurement, references map[string]measurement) error {
	ref, ok := references[key(actual.Dataset, actual.Class)]
	if !ok {
		return fmt.Errorf("missing approved reference")
	}
	median := math.Min(30000, math.Max(1, ref.Median*1.35))
	maximum := math.Min(30000, math.Max(2, ref.Maximum*1.35))
	allocation := float64(ref.Allocation)*1.2 + 4096
	if actual.Median > median || actual.Maximum > maximum || float64(actual.Allocation) > allocation {
		return fmt.Errorf("performance budget exceeded: dataset=%d class=%s", actual.Dataset, actual.Class)
	}
	return nil
}
