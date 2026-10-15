package postgis

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/internal/querytest"
)

// Literal ID sets are independent of both generated SQL and provider comparisons.
func TestFeaturePostGISFilterLive(t *testing.T) {
	db, schema := featureLiveDatabase(t)
	for _, custom := range []bool{false, true} {
		name := "ordinary"
		if custom {
			name = "custom"
		}
		t.Run(name, func(t *testing.T) {
			table := pgQuoteIdent(schema) + "." + pgQuoteIdent(name+"_filters")
			prefix := ""
			if custom {
				prefix = "source_"
			}
			col := func(n string) string { return pgQuoteIdent(prefix + n) }
			ddl := "CREATE TABLE " + table + " (" + col("id") + " bigint PRIMARY KEY," + col("geom") + " text," + col("n") + " bigint," + col("s") + " text COLLATE \"C\"," + col("b") + " boolean," + col("t") + " bigint,selection_flag integer NOT NULL)"
			if _, err := db.Exec(context.Background(), ddl); err != nil {
				t.Fatal(err)
			}
			insert := "INSERT INTO " + table + " VALUES($1,$2,$3,$4,$5,$6,$7)"
			rows := [][]any{{int64(10), nil, nil, nil, nil, nil, 1}, {int64(20), "POINT(1 1)", int64(0), "a", false, int64(10), 1}, {int64(30), "POINT(2 2)", int64(5), "a ", true, int64(20), 1}, {int64(40), "POINT(3 3)", int64(9223372036854775807), "😀", true, int64(30), 1}, {int64(90), "POINT(4 4)", int64(99), "~", true, int64(40), 0}}
			for _, row := range rows {
				if _, err := db.Exec(context.Background(), insert, row...); err != nil {
					t.Fatal(err)
				}
			}
			conf := map[string]any{"name": "items", "tablename": table, "id_fieldname": prefix + "id", "geometry_fieldname": prefix + "geom", "geometry_type": "point", "geometry_format": "wkt", "srid": 4326, "fields": []string{prefix + "n", prefix + "s", prefix + "b", prefix + "t"}, "temporal_field": prefix + "t", "temporal_storage": "unix_seconds"}
			if custom {
				conf["id_fieldname"], conf["geometry_fieldname"], conf["temporal_field"] = "id", "geom", "t"
				conf["feature_sql"] = "SELECT source_id AS id,source_geom AS geom,source_n AS n,source_s AS s,source_b AS b,source_t AS t FROM " + table + " WHERE selection_flag = 1"
			}
			tiler, err := NewTileProvider(dict.Dict{"name": "filter_live", "uri": os.Getenv("PGURI"), "layers": []map[string]any{conf}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			p := tiler.(*Provider)
			defer p.pool.Close()
			queryables, err := p.layers["items"].FeatureQueryables()
			if err != nil {
				t.Fatal(err)
			}
			if len(queryables.Fields()) != 4 {
				t.Fatal("scalar catalog incomplete")
			}
			field := func(n string) string {
				if custom {
					return n
				}
				return prefix + n
			}
			type comparison struct {
				name, property, literal string
				kind                    provider.FilterScalarType
				op                      provider.FilterCompareOperator
				ids                     []uint64
				ordinary90              bool
				not                     bool
			}
			cases := []comparison{
				{"fraction less", "n", "0.5", provider.FilterNumber, provider.FilterLess, []uint64{20}, false, false},
				{"fraction equality", "n", "0.5", provider.FilterNumber, provider.FilterEqual, nil, false, false},
				{"fraction not equality", "n", "0.5", provider.FilterNumber, provider.FilterEqual, []uint64{20, 30, 40}, true, true},
				{"above int64", "n", "9223372036854775808", provider.FilterNumber, provider.FilterLess, []uint64{20, 30, 40}, true, false},
				{"below int64", "n", "-9223372036854775809", provider.FilterNumber, provider.FilterGreater, []uint64{20, 30, 40}, true, false},
				{"huge positive", "n", "1e4096", provider.FilterNumber, provider.FilterLess, []uint64{20, 30, 40}, true, false},
				{"huge negative", "n", "-1e4096", provider.FilterNumber, provider.FilterGreater, []uint64{20, 30, 40}, true, false},
				{"tiny positive", "n", "1e-4096", provider.FilterNumber, provider.FilterLess, []uint64{20}, false, false},
				{"tiny negative", "n", "-1e-4096", provider.FilterNumber, provider.FilterGreater, []uint64{20, 30, 40}, true, false},
				{"string eq", "s", "a", provider.FilterString, provider.FilterEqual, []uint64{20}, false, false},
				{"string ne", "s", "a", provider.FilterString, provider.FilterNotEqual, []uint64{30, 40}, true, false},
				{"string lt", "s", "a", provider.FilterString, provider.FilterLess, nil, false, false},
				{"string le", "s", "a", provider.FilterString, provider.FilterLessEqual, []uint64{20}, false, false},
				{"string gt", "s", "a", provider.FilterString, provider.FilterGreater, []uint64{30, 40}, true, false},
				{"string ge", "s", "a", provider.FilterString, provider.FilterGreaterEqual, []uint64{20, 30, 40}, true, false},
				{"supplementary", "s", "😀", provider.FilterString, provider.FilterEqual, []uint64{40}, false, false},
				{"NUL literal", "s", "a\x00", provider.FilterString, provider.FilterLess, []uint64{20}, false, false},
				{"bool eq", "b", "false", provider.FilterBoolean, provider.FilterEqual, []uint64{20}, false, false},
				{"bool ne", "b", "false", provider.FilterBoolean, provider.FilterNotEqual, []uint64{30, 40}, true, false},
				{"bool lt", "b", "false", provider.FilterBoolean, provider.FilterLess, nil, false, false},
				{"bool le", "b", "false", provider.FilterBoolean, provider.FilterLessEqual, []uint64{20}, false, false},
				{"bool gt", "b", "false", provider.FilterBoolean, provider.FilterGreater, []uint64{30, 40}, true, false},
				{"bool ge", "b", "false", provider.FilterBoolean, provider.FilterGreaterEqual, []uint64{20, 30, 40}, true, false},
			}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					node := filterTestCompare(t, field(c.property), c.literal, c.kind, c.op)
					if c.not {
						node = provider.FilterNode{Kind: provider.FilterNot, Children: []provider.FilterNode{node}}
					}
					q := provider.FeatureQuery{Limit: 100, Filter: filterTestExpression(t, node)}
					var ids []uint64
					_, err := p.QueryFeatures(context.Background(), "items", q, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
					expected := append([]uint64(nil), c.ids...)
					if !custom && c.ordinary90 {
						expected = append(expected, 90)
					}
					if err != nil || !reflect.DeepEqual(ids, expected) {
						t.Fatalf("ids=%v expected=%v err=%v", ids, expected, err)
					}
				})
			}
			t.Run("null NOT and combination", func(t *testing.T) {
				node := provider.FilterNode{Kind: provider.FilterAnd, Children: []provider.FilterNode{filterTestCompare(t, field("n"), "0.5", provider.FilterNumber, provider.FilterGreater), {Kind: provider.FilterIsNotNull, Property: field("s")}}}
				start, end := time.Unix(15, 0).UTC(), time.Unix(35, 0).UTC()
				q := provider.FeatureQuery{Limit: 1, Offset: 1, Filter: filterTestExpression(t, node), Temporal: &provider.TemporalConstraint{Start: &start, End: &end}, Bounds: []geom.Extent{{1, 1, 3, 3}}, BoundsSRID: 4326}
				var ids []uint64
				result, err := p.QueryFeatures(context.Background(), "items", q, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
				if err != nil || !reflect.DeepEqual(ids, []uint64{40}) || result.HasMore {
					t.Fatal(ids, result, err)
				}
			})
			t.Run("null IS NULL", func(t *testing.T) {
				q := provider.FeatureQuery{Limit: 1, Filter: filterTestExpression(t, provider.FilterNode{Kind: provider.FilterIsNull, Property: field("n")})}
				var ids []uint64
				_, err := p.QueryFeatures(context.Background(), "items", q, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
				if err != nil || !reflect.DeepEqual(ids, []uint64{10}) {
					t.Fatal(ids, err)
				}
			})
		})
	}
}

func TestFeaturePostGISFilterContractLive(t *testing.T) {
	db, schema := featureLiveDatabase(t)
	factory := func(t *testing.T, fixture querytest.Fixture) querytest.Instance {
		custom := fixture.Profile == querytest.CustomSelection
		name := fmt.Sprintf("filter_contract_%d", featureFixtureSequence.Add(1))
		table := pgQuoteIdent(schema) + "." + pgQuoteIdent(name)
		prefix := ""
		if custom {
			prefix = "source_"
		}
		physical := func(name string) string { return pgQuoteIdent(prefix + name) }
		ddl := "CREATE TABLE " + table + " (" + physical("id") + " bigint PRIMARY KEY," + physical("geom") + " bytea," + physical("n") + " bigint," + physical("s") + " text," + physical("b") + " boolean," + physical("start_time") + " bigint," + physical("end_time") + " bigint,selection_flag integer NOT NULL)"
		if _, err := db.Exec(context.Background(), ddl); err != nil {
			t.Fatal(err)
		}
		for _, row := range fixture.Rows {
			wire, err := querytest.EncodeFixtureWKB(row.Feature.Geometry)
			if err != nil {
				t.Fatal(err)
			}
			var start, end any
			if row.Start != nil {
				start, err = querytest.EncodeFixtureTemporal(*row.Start, fixture.TemporalStorage)
				if err != nil {
					t.Fatal(err)
				}
			}
			if row.End != nil {
				end, err = querytest.EncodeFixtureTemporal(*row.End, fixture.TemporalStorage)
				if err != nil {
					t.Fatal(err)
				}
			}
			included := 1
			if row.ExcludedBySelection {
				included = 0
			}
			if _, err := db.Exec(context.Background(), "INSERT INTO "+table+" VALUES($1,$2,$3,$4,$5,$6,$7,$8)", int64(row.Feature.ID), wire, row.Feature.Tags["n"], row.Feature.Tags["s"], row.Feature.Tags["b"], start, end, included); err != nil {
				t.Fatal(err)
			}
		}
		conf := map[string]any{"name": "items", "tablename": table, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkb", "geometry_type": "point", "srid": 4326, "temporal_start_field": "start_time", "temporal_end_field": "end_time", "temporal_storage": "unix_seconds", "fields": append([]string(nil), fixture.PublicFields...)}
		if custom {
			tileTable := pgQuoteIdent(schema) + "." + pgQuoteIdent(name+"_tile")
			if _, err := db.Exec(context.Background(), "CREATE TABLE "+tileTable+" (id bigint PRIMARY KEY,geom bytea)"); err != nil {
				t.Fatal(err)
			}
			conf["tablename"] = tileTable
			projections := []string{"f.source_id AS id", "f.source_geom AS geom"}
			for _, field := range fixture.PublicFields {
				projections = append(projections, "f."+physical(field)+" AS "+pgQuoteIdent(field))
			}
			conf["feature_sql"] = "SELECT " + strings.Join(projections, ",") + " FROM " + table + " f WHERE f.selection_flag = 1"
		}
		tiler, err := NewTileProvider(dict.Dict{"name": "filter_contract_live", "uri": os.Getenv("PGURI"), "layers": []map[string]any{conf}}, nil)
		if err != nil {
			return querytest.Instance{SetupError: err}
		}
		p := tiler.(*Provider)
		t.Cleanup(func() { p.pool.Close() })
		return querytest.Instance{Querier: p, Layer: "items", QueryableLayer: p.layers["items"], CountMode: querytest.UnknownCount}
	}
	querytest.RunFilterProfiles(t, factory, querytest.ProfileOptions{Custom: querytest.Options{PublicTemporalProperties: &querytest.TemporalPropertyProfile{StartField: "start_time", EndField: "end_time", Storage: querytest.UnixSeconds}}})
}
