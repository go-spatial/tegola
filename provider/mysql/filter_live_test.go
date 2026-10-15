package mysql

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

func TestFeatureLiveFilterPhysicalProfiles(t *testing.T) {
	for _, flavor := range []string{GeometryFormatMySQL, GeometryFormatMariaDB} {
		t.Run(flavor, func(t *testing.T) {
			db, connection := featureLiveDatabase(t, flavor)
			t.Run("decimal65_30", func(t *testing.T) {
				table := featureLiveTable(t, db, "id BIGINT UNIQUE,geom TEXT,d DECIMAL(65,30)")
				maximum := strings.Repeat("9", 35) + "." + strings.Repeat("9", 30)
				for _, row := range [][]any{{int64(10), nil, nil}, {int64(20), nil, "-" + maximum},
					{int64(30), nil, "0.000000000000000000000000000001"}, {int64(40), nil, maximum}} {
					if _, err := db.Exec("INSERT INTO "+featureQuoteIdentifier(table)+" VALUES (?,?,?)", row...); err != nil {
						t.Fatal(err)
					}
				}
				p := featureLiveProvider(t, connection, map[string]any{"name": "items", "tablename": table,
					"id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 4326, "fields": []string{"d"}})
				for _, test := range []struct {
					name, literal string
					operator      provider.FilterCompareOperator
					not           bool
					ids           []uint64
				}{
					{"max", maximum, provider.FilterEqual, false, []uint64{40}},
					{"min", "-" + maximum, provider.FilterEqual, false, []uint64{20}},
					{"precision_tail", "0.0000000000000000000000000000009", provider.FilterGreater, false, []uint64{30, 40}},
					{"unrepresentable_NOT", "0.0000000000000000000000000000009", provider.FilterEqual, true, []uint64{20, 30, 40}},
					{"outdomain_NOT", "1e35", provider.FilterGreater, true, []uint64{20, 30, 40}},
				} {
					t.Run(test.name, func(t *testing.T) {
						literal, err := provider.NewFilterLiteral(provider.FilterNumber, test.literal)
						if err != nil {
							t.Fatal(err)
						}
						node := provider.FilterNode{Kind: provider.FilterCompare, Property: "d", Operator: test.operator, Literal: literal}
						if test.not {
							node = provider.FilterNode{Kind: provider.FilterNot, Children: []provider.FilterNode{node}}
						}
						expression, err := provider.NewFilterExpression(node)
						if err != nil {
							t.Fatal(err)
						}
						ids := []uint64{}
						result, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10, Filter: &expression}, func(feature *provider.Feature) error {
							ids = append(ids, feature.ID)
							return nil
						})
						if err != nil || !reflect.DeepEqual(ids, test.ids) || result.NumberMatched == nil || *result.NumberMatched != uint64(len(test.ids)) {
							t.Fatalf("max precision filter: IDs%v want%v result%+v err%v", ids, test.ids, result, err)
						}
					})
				}
			})
			// Real storage types and direct differently-named aliases prove each
			// published capability; these oracles never inspect provider results.
			table := featureLiveTable(t, db, "source_id BIGINT UNSIGNED UNIQUE,source_geom TEXT,"+
				"source_i BIGINT,source_u BIGINT UNSIGNED,source_d DECIMAL(20,2),"+
				"source_s TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin,source_b BIT(1),selection_flag INT NOT NULL")
			longText := strings.Repeat("x", 16378) + "\x00🙂 " // Exactly the 16 KiB decoded literal bound.
			for _, row := range [][]any{
				{uint64(10), nil, nil, nil, nil, nil, nil, 1},
				{uint64(20), "POINT (0 0)", "-9223372036854775808", "0", "-999999999999999999.99", "a", false, 1},
				{uint64(30), "POINT (0 0)", "-2", "9007199254740993", "-1.23", "a ", true, 1},
				{uint64(40), "POINT (0 0)", "-1", "18446744073709551615", "1.23", "z", true, 1},
				{uint64(50), "POINT (0 0)", "9223372036854775807", "18446744073709551614", "999999999999999999.99", longText, true, 1},
				{uint64(60), "POINT (0 0)", "0", "1", "0.00", "🙂\x00 ", true, 1},
				{uint64(90), "POINT (0 0)", "0", "18446744073709551615", "0.00", "excluded", false, 0},
			} {
				if _, err := db.Exec("INSERT INTO "+featureQuoteIdentifier(table)+" VALUES (?,?,?,?,?,?,?,?)", row...); err != nil {
					t.Fatal(err)
				}
			}
			for _, custom := range []bool{false, true} {
				name := "ordinary"
				if custom {
					name = "custom"
				}
				t.Run(name, func(t *testing.T) {
					layer := map[string]any{"name": "items", "tablename": table, "id_fieldname": "source_id", "geometry_fieldname": "source_geom",
						"geometry_format": "wkt", "geometry_type": "point", "srid": 4326,
						"fields": []string{"source_i", "source_u", "source_d", "source_s", "source_b"}}
					if custom {
						delete(layer, "tablename")
						layer["id_fieldname"], layer["geometry_fieldname"] = "id", "geom"
						layer["fields"] = []string{"i", "u", "d", "s", "b"}
						layer["sql"] = "SELECT source_id AS id,source_geom AS geom FROM " + featureQuoteIdentifier(table)
						layer["feature_sql"] = "SELECT p.source_id AS id,p.source_geom AS geom,p.source_i AS i,p.source_u AS u," +
							"p.source_d AS d,p.source_s AS s,p.source_b AS b FROM " + featureQuoteIdentifier(table) + " p WHERE p.selection_flag=1"
					}
					p := featureLiveProvider(t, connection, layer)
					catalog, err := p.layers["items"].FeatureQueryables()
					if err != nil {
						t.Fatal(err)
					}
					fieldName := func(name string) string {
						if custom {
							return name
						}
						return "source_" + name
					}
					for _, field := range []struct {
						name string
						kind provider.QueryableType
					}{{"i", provider.QueryableInteger}, {"u", provider.QueryableInteger}, {"d", provider.QueryableNumber}, {"s", provider.QueryableString}, {"b", provider.QueryableBoolean}} {
						actual, ok := catalog.Lookup(fieldName(field.name))
						if !ok || actual.Type != field.kind || !actual.Nullable {
							t.Fatalf("physical capability %s: %+v %v", field.name, actual, ok)
						}
					}
					all := []uint64{20, 30, 40, 50, 60}
					if !custom {
						all = append(all, 90)
					}
					for _, test := range []struct {
						name, property, literal string
						kind                    provider.FilterScalarType
						operator                provider.FilterCompareOperator
						not                     bool
						ids                     []uint64
					}{
						{"signed_min", "i", "-9223372036854775808", provider.FilterNumber, provider.FilterEqual, false, []uint64{20}},
						{"signed_negative_fraction", "i", "-1.2", provider.FilterNumber, provider.FilterLess, false, []uint64{20, 30}},
						{"signed_max", "i", "9223372036854775807", provider.FilterNumber, provider.FilterEqual, false, []uint64{50}},
						{"signed_outside_NOT", "i", "9223372036854775808", provider.FilterNumber, provider.FilterGreater, true, all},
						{"unsigned_max", "u", "18446744073709551615", provider.FilterNumber, provider.FilterEqual, false, physicalExpected(custom, []uint64{40}, true)},
						{"unsigned_precision", "u", "9007199254740992", provider.FilterNumber, provider.FilterGreater, false, physicalExpected(custom, []uint64{30, 40, 50}, true)},
						{"unsigned_outside_NOT", "u", "-1", provider.FilterNumber, provider.FilterEqual, true, all},
						{"decimal_max", "d", "999999999999999999.99", provider.FilterNumber, provider.FilterEqual, false, []uint64{50}},
						{"decimal_negative_tail", "d", "-1.234", provider.FilterNumber, provider.FilterLess, false, []uint64{20}},
						{"decimal_unrepresentable_NOT", "d", "1.234", provider.FilterNumber, provider.FilterEqual, true, all},
						{"decimal_extreme", "d", "1e1024", provider.FilterNumber, provider.FilterGreaterEqual, false, []uint64{}},
						{"string_16KiB", "s", longText, provider.FilterString, provider.FilterEqual, false, []uint64{50}},
						{"string_space", "s", "a ", provider.FilterString, provider.FilterEqual, false, []uint64{30}},
						{"string_supplementary_NUL", "s", "🙂\x00 ", provider.FilterString, provider.FilterEqual, false, []uint64{60}},
						{"string_ordinal", "s", "z", provider.FilterString, provider.FilterGreater, false, []uint64{60}},
						{"boolean_less", "b", "true", provider.FilterBoolean, provider.FilterLess, false, physicalExpected(custom, []uint64{20}, true)},
						{"boolean_greater", "b", "false", provider.FilterBoolean, provider.FilterGreater, false, []uint64{30, 40, 50, 60}},
					} {
						t.Run(test.name, func(t *testing.T) {
							literal, err := provider.NewFilterLiteral(test.kind, test.literal)
							if err != nil {
								t.Fatal(err)
							}
							node := provider.FilterNode{Kind: provider.FilterCompare, Property: fieldName(test.property), Operator: test.operator, Literal: literal}
							if test.not {
								node = provider.FilterNode{Kind: provider.FilterNot, Children: []provider.FilterNode{node}}
							}
							expression, err := provider.NewFilterExpression(node)
							if err != nil {
								t.Fatal(err)
							}
							ids := []uint64{}
							result, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 20, Filter: &expression}, func(feature *provider.Feature) error { ids = append(ids, feature.ID); return nil })
							if err != nil || !reflect.DeepEqual(ids, test.ids) || result.NumberMatched == nil || *result.NumberMatched != uint64(len(test.ids)) {
								t.Fatalf("physical filter got%v want%v result%+v err%v", ids, test.ids, result, err)
							}
						})
					}
					// Each NULL source is published explicitly; BIT(1) yields bool
					// rather than a binary/base64 tag in the raw feature path.
					_, err = p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{IDs: []uint64{10, 30}, Limit: 2}, func(feature *provider.Feature) error {
						if feature.ID == 10 {
							for _, name := range []string{"i", "u", "d", "s", "b"} {
								value, ok := feature.Tags[fieldName(name)]
								if !ok || value != nil {
									t.Errorf("NULL lost %s %#v", name, feature.Tags)
								}
							}
						} else if feature.Tags[fieldName("b")] != true {
							t.Errorf("BIT1 not bool %#v", feature.Tags)
						}
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
					t.Run("latin1_connection_exact_filter", func(t *testing.T) {
						// Keep source output ASCII/numeric while changing the connection
						// character set. Only the filter literal contains UTF-8/NUL;
						// its bound representation must remain ASCII hex.
						p.db.SetMaxOpenConns(1)
						if _, err := p.db.Exec("SET NAMES latin1"); err != nil {
							t.Fatal(err)
						}
						defer func() {
							if _, err := p.db.Exec("SET NAMES utf8mb4"); err != nil {
								t.Error(err)
							}
						}()
						literal, err := provider.NewFilterLiteral(provider.FilterString, longText)
						if err != nil {
							t.Fatal(err)
						}
						expression, err := provider.NewFilterExpression(provider.FilterNode{Kind: provider.FilterCompare,
							Property: fieldName("s"), Operator: provider.FilterEqual, Literal: literal})
						if err != nil {
							t.Fatal(err)
						}
						ids := []uint64{}
						_, err = p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10,
							Fields: []string{fieldName("i")}, Filter: &expression}, func(feature *provider.Feature) error {
							ids = append(ids, feature.ID)
							return nil
						})
						if err != nil || !reflect.DeepEqual(ids, []uint64{50}) {
							t.Fatalf("connection charset changed filter: %v %v", ids, err)
						}
					})
				})
			}
		})
	}
}

func physicalExpected(custom bool, ids []uint64, includeExcluded bool) []uint64 {
	if !custom && includeExcluded {
		return append(append([]uint64(nil), ids...), 90)
	}
	return ids
}
