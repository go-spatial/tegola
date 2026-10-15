package hana_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/hana"
)

// These expectations are literals independent of compiler/database evaluation.
func TestFeatureFilterPhysicalProfilesLive(t *testing.T) {
	if os.Getenv(TESTENV) != "yes" || GetConnectionURI() == "" {
		t.Skip("live HANA physical scalar proof requires explicit connection configuration")
	}
	db, err := hana.OpenDB(GetConnectionURI())
	if err != nil {
		t.Fatal("open physical filter proof")
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error("close physical filter proof")
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	table := `"` + featureFixtureTable(t, "TEGOLA_SCALAR") + `"`
	if _, err := db.ExecContext(ctx, "CREATE ROW TABLE "+table+` ("id" BIGINT UNIQUE,"geom" VARBINARY(5000),"n" DECIMAL(38,2),"s" NVARCHAR(5000),"b" BOOLEAN,"floating" DOUBLE,"date" DATE,"fixed" VARCHAR(10))`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := db.ExecContext(cleanupCtx, "DROP TABLE "+table); err != nil {
			t.Error("drop physical filter proof")
		}
	})
	maximum := "999999999999999999999999999999999999.99"
	long := strings.Repeat("\uE000", 5000)
	rows := []struct{ n, s, b any }{{nil, nil, nil}, {"0", "", false}, {"1.23", "A", true}, {"-1.23", "A ", false}, {maximum, "\uE000", true}, {"-" + maximum, "\U00010000", true}, {"0", "\x00", false}, {"0", long, false}}
	for i, row := range rows {
		if _, err := db.ExecContext(ctx, "INSERT INTO "+table+` VALUES (?,NULL,CAST(CAST(? AS NVARCHAR(64)) AS DECIMAL(38,2)),?,?,NULL,NULL,NULL)`, int64(i+1), row.n, row.s, row.b); err != nil {
			t.Fatal(err)
		}
	}
	p, err := hana.CreateProvider(dict.Dict{hana.ConfigKeyName: "scalar_proof", hana.ConfigKeyURI: GetConnectionURI(), "layers": []map[string]any{{"name": "items", "tablename": table, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkb", "geometry_type": "point", "srid": 4326, "fields": []string{"n", "s", "b", "floating", "fixed"}}}}, nil, hana.ProviderType)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	metadataRows, metadataErr := db.QueryContext(ctx, "SELECT * FROM "+table+" LIMIT 0")
	if metadataErr != nil {
		t.Fatal(metadataErr)
	}
	defer func() {
		if err := metadataRows.Close(); err != nil {
			t.Error("close physical metadata rows")
		}
	}()
	metadataTypes, metadataErr := metadataRows.ColumnTypes()
	if metadataErr != nil {
		t.Fatal(metadataErr)
	}
	for _, metadataType := range metadataTypes {
		t.Logf("physical-result %s %s", metadataType.Name(), metadataType.DatabaseTypeName())
	}
	if err := metadataRows.Err(); err != nil {
		t.Fatal(err)
	}
	layers, err := p.Layers()
	if err != nil || len(layers) != 1 {
		t.Fatal("actual physical layer unavailable")
	}
	queryable, ok := layers[0].(provider.FeatureQueryableLayerInfo)
	if !ok {
		t.Fatal("queryable metadata absent")
	}
	catalog, err := queryable.FeatureQueryables()
	if err != nil {
		t.Fatal(err)
	}
	expected := []provider.FeatureQueryable{{Name: "b", Type: provider.QueryableBoolean, Nullable: true}, {Name: "n", Type: provider.QueryableNumber, Nullable: true}, {Name: "s", Type: provider.QueryableString, Nullable: true}}
	if !reflect.DeepEqual(catalog.Fields(), expected) {
		t.Fatalf("physical scalar catalog=%v", catalog.Fields())
	}
	type testCase struct {
		property, text string
		kind           provider.FilterScalarType
		op             provider.FilterCompareOperator
		ids            []uint64
	}
	cases := []testCase{
		{"n", "1.235", provider.FilterNumber, provider.FilterEqual, []uint64{}},
		{"n", "1.235", provider.FilterNumber, provider.FilterNotEqual, []uint64{2, 3, 4, 5, 6, 7, 8}},
		{"n", "1.235", provider.FilterNumber, provider.FilterLess, []uint64{2, 3, 4, 6, 7, 8}},
		{"n", "1.235", provider.FilterNumber, provider.FilterLessEqual, []uint64{2, 3, 4, 6, 7, 8}},
		{"n", "1.235", provider.FilterNumber, provider.FilterGreater, []uint64{5}},
		{"n", "1.235", provider.FilterNumber, provider.FilterGreaterEqual, []uint64{5}},
		{"n", "-1.235", provider.FilterNumber, provider.FilterLess, []uint64{6}},
		{"n", "-1.235", provider.FilterNumber, provider.FilterGreaterEqual, []uint64{2, 3, 4, 5, 7, 8}},
		{"n", maximum, provider.FilterNumber, provider.FilterEqual, []uint64{5}},
		{"n", "1e4096", provider.FilterNumber, provider.FilterLess, []uint64{2, 3, 4, 5, 6, 7, 8}},
		{"s", "A", provider.FilterString, provider.FilterEqual, []uint64{3}},
		{"s", "A", provider.FilterString, provider.FilterNotEqual, []uint64{2, 4, 5, 6, 7, 8}},
		{"s", "A", provider.FilterString, provider.FilterLess, []uint64{2, 7}},
		{"s", "A", provider.FilterString, provider.FilterLessEqual, []uint64{2, 3, 7}},
		{"s", "A", provider.FilterString, provider.FilterGreater, []uint64{4, 5, 6, 8}},
		{"s", "A", provider.FilterString, provider.FilterGreaterEqual, []uint64{3, 4, 5, 6, 8}},
		{"s", "\uE000", provider.FilterString, provider.FilterLess, []uint64{2, 3, 4, 7}},
		{"s", "\uE000", provider.FilterString, provider.FilterGreater, []uint64{6, 8}},
		{"s", "\x00", provider.FilterString, provider.FilterEqual, []uint64{7}},
		{"s", long, provider.FilterString, provider.FilterEqual, []uint64{8}},
		{"s", strings.Repeat("x", 16383) + "b", provider.FilterString, provider.FilterEqual, []uint64{}},
		{"b", "false", provider.FilterBoolean, provider.FilterEqual, []uint64{2, 4, 7, 8}},
		{"b", "false", provider.FilterBoolean, provider.FilterNotEqual, []uint64{3, 5, 6}},
		{"b", "false", provider.FilterBoolean, provider.FilterLess, []uint64{}},
		{"b", "false", provider.FilterBoolean, provider.FilterLessEqual, []uint64{2, 4, 7, 8}},
		{"b", "false", provider.FilterBoolean, provider.FilterGreater, []uint64{3, 5, 6}},
		{"b", "false", provider.FilterBoolean, provider.FilterGreaterEqual, []uint64{2, 3, 4, 5, 6, 7, 8}},
	}
	for i, c := range cases {
		t.Run(c.property+"/"+string(rune('A'+i)), func(t *testing.T) {
			literal, err := provider.NewFilterLiteral(c.kind, c.text)
			if err != nil {
				t.Fatal(err)
			}
			filter, err := provider.NewFilterExpression(provider.FilterNode{Kind: provider.FilterCompare, Property: c.property, Operator: c.op, Literal: literal})
			if err != nil {
				t.Fatal(err)
			}
			ids := []uint64{}
			_, err = p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Filter: &filter, Fields: []string{"n", "s", "b"}, Limit: 100}, func(f *provider.Feature) error {
				ids = append(ids, f.ID)
				if f.ID == 5 {
					if f.Tags["n"] != json.Number(maximum) {
						t.Errorf("decimal source serialization=%v", f.Tags["n"])
					}
				}
				return nil
			})
			if err != nil || !reflect.DeepEqual(ids, c.ids) {
				t.Fatalf("ids=%v expected=%v err=%v", ids, c.ids, err)
			}
		})
	}
}
