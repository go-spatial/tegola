package mysql

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/internal/featuresql"
)

func TestFeatureSQLDirectLineageAndBoundWhere(t *testing.T) {
	p, store := newFeatureTestProvider(t, nil)
	p.Database = "gis"
	store.profile.schema.columns[2].collation = "utf8mb4_bin"
	plan, err := featuresql.Parse("SELECT f.id AS publicid, f.geom AS shape, f.name AS label, f.start AS begin, f.end AS finish FROM gis.items f WHERE f.name='secret''quoted' AND f.id IN (1,9007199254740993)", featuresql.MySQL)
	if err != nil {
		t.Fatal(err)
	}
	layer := Layer{name: "items", bboxTable: "unrelated_tile_source", idFieldname: "publicid", geomFieldname: "shape", srid: 4326,
		geometryFormat: GeometryFormatWKT, bboxFields: [4]string{"name", "MINX", "MINY", "MAXY"}}
	f, err := p.registerFeatureSelection(layer, dict.Dict{"temporal_start_field": "begin", "temporal_end_field": "finish", "temporal_storage": "unix_nanoseconds"}, plan)
	if err != nil {
		t.Fatal(err)
	}
	if f.physicalID() != "id" || f.geometry != "shape" || f.temporal.StartField != "begin" {
		t.Fatalf("lineage: %#v", f)
	}
	if containsFeatureFold(f.properties, "label") {
		t.Fatal("private source column exposed through alias")
	}
	if strings.Contains(f.filter, "secret") || !strings.Contains(f.filter, "CAST(? AS SIGNED)") || !reflect.DeepEqual(f.filterArgs, []any{"secret'quoted", int64(1), int64(9007199254740993)}) {
		t.Fatalf("bound predicate: %s %#v", f.filter, f.filterArgs)
	}
	columns, selected := f.queryColumns(f.properties)
	if strings.Contains(strings.Join(selected, ","), layer.bboxTable) || strings.Contains(f.filter, layer.bboxTable) {
		t.Fatal("tile bounds qualifier leaked into feature SQL lineage")
	}
	if columns[0] != "publicid" || !strings.Contains(strings.Join(selected, ","), "l.`id` AS `publicid`") {
		t.Fatalf("projection SQL: %v %v", columns, selected)
	}
	feature, _, representable, err := f.decodeFeature(columns, []any{int64(1), []byte("POINT(1 1)"), nil, nil}, nil)
	if err != nil || !representable || !reflect.DeepEqual(feature.Tags, map[string]any{"begin": nil, "finish": nil}) {
		t.Fatalf("public custom temporal NULL values: %#v %v", feature.Tags, err)
	}
}

func TestFeatureSQLAdmissionClassificationWithoutExecution(t *testing.T) {
	p := &Provider{}
	layer := Layer{sql: "tile SQL", srid: 4326, geometryFormat: GeometryFormatWKT}
	for _, text := range []any{"", 3, "SELECT id,geom FROM items;", "SELECT id,geom FROM items /* hidden */", "SELECT id,geom FROM items WHERE name='a\\b'"} {
		_, err := p.registerFeatureLayer(layer, dict.Dict{"feature_sql": text})
		var invalid provider.InvalidFeatureQueryError
		if !errors.As(err, &invalid) {
			t.Fatalf("invalid selection classification: %v", err)
		}
	}
	for _, text := range []string{"SELECT * FROM items", "SELECT id,geom FROM items LIMIT 1", "SELECT id,geom FROM items JOIN other ON items.id=other.id"} {
		_, err := p.registerFeatureLayer(layer, dict.Dict{"feature_sql": text})
		if !errors.Is(err, provider.ErrUnsupported) {
			t.Fatalf("unsupported selection classification: %v", err)
		}
	}
}

func TestFeatureSQLExactTypedDecimalAndUnsigned(t *testing.T) {
	parseLiteral := func(text string) featuresql.Literal {
		plan, err := featuresql.Parse("SELECT id,geom FROM items WHERE id="+text, featuresql.MySQL)
		if err != nil {
			t.Fatal(err)
		}
		predicate, _ := plan.Predicate()
		return predicate.Literals()[0]
	}
	unsigned := featuresql.ColumnMetadata{Kind: featuresql.IntegerColumn, Bits: 64, Unsigned: true}
	value, err := bindFeatureLiteral(unsigned, "=", parseLiteral("18446744073709551615"))
	if err != nil || value != uint64(math.MaxUint64) {
		t.Fatalf("full unsigned: %#v %v", value, err)
	}
	decimal := featuresql.ColumnMetadata{Kind: featuresql.DecimalColumn, Precision: 30, Scale: 10}
	value, err = bindFeatureLiteral(decimal, "=", parseLiteral("9007199254740993.1234567890"))
	if err != nil || value != "9007199254740993.1234567890" {
		t.Fatalf("decimal narrowed: %#v %v", value, err)
	}
	cast, err := featureParameterExpression(decimal, "=", "?")
	if err != nil || cast != "CAST(? AS DECIMAL(30,10))" {
		t.Fatal(cast, err)
	}
	for _, number := range []string{"0.12345678901", "1e999999999", "100000000000000000000"} {
		if _, err := bindFeatureLiteral(decimal, "=", parseLiteral(number)); !errors.Is(err, provider.ErrUnsupported) {
			t.Fatalf("unrepresentable decimal accepted: %v", err)
		}
	}
	property, err := featureProperty(featureColumn{dataType: "decimal"}, []byte("9007199254740993.1234567890"))
	if err != nil || property != json.Number("9007199254740993.1234567890") {
		t.Fatalf("property precision: %v %v", property, err)
	}
	if _, err := bindFeatureLiteral(featuresql.ColumnMetadata{Kind: featuresql.FloatColumn}, "=", parseLiteral("0.1")); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatal(err)
	}
}

func TestFeatureSQLQualifierCaseAndAtomicProjection(t *testing.T) {
	for _, mode := range []int{0, 1} {
		p, store := newFeatureTestProvider(t, nil)
		p.Database = "gis"
		store.profile.schema.lowerCaseTables = mode
		plan, err := featuresql.Parse("SELECT F.id, F.geom FROM items f", featuresql.MySQL)
		if err != nil {
			t.Fatal(err)
		}
		_, err = p.registerFeatureSelection(Layer{idFieldname: "id", geomFieldname: "geom", srid: 4326, geometryFormat: GeometryFormatWKT}, dict.Dict{}, plan)
		if (mode == 0 && err == nil) || (mode == 1 && err != nil) {
			t.Fatalf("alias table case mode%d: %v", mode, err)
		}
	}
	f := testFeatureProfile(t, false)
	f.projection = []featureProjection{{source: "secret.id", output: "id"}, {source: "shape`blob", output: "geom"}}
	_, selected := f.queryColumns(nil)
	if !reflect.DeepEqual(selected, []string{"l.`secret.id` AS `id`", "l.`shape``blob` AS `geom`"}) {
		t.Fatalf("atomic lineage: %v", selected)
	}
}
