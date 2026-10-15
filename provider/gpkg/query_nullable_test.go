//go:build cgo

package gpkg

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

func TestFeatureSelectedNullableProperties(t *testing.T) {
	p, db := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT,name TEXT,value INTEGER,private_time INTEGER)", map[string]interface{}{"fields": []string{"name", "value"}, "temporal_field": "private_time", "temporal_storage": "unix_seconds"})
	if _, err := db.Exec("INSERT INTO items VALUES(10,'POINT(1 2)',NULL,0,NULL),(20,'POINT(1 2)','',NULL,NULL),(30,'POINT(1 2)','road',7,NULL)"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		fields []string
		want   []map[string]any
	}{
		{"all", nil, []map[string]any{{"name": nil, "value": int64(0)}, {"name": "", "value": nil}, {"name": "road", "value": int64(7)}}},
		{"name", []string{"name"}, []map[string]any{{"name": nil}, {"name": ""}, {"name": "road"}}},
		{"value", []string{"value"}, []map[string]any{{"value": int64(0)}, {"value": nil}, {"value": int64(7)}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []map[string]any
			result, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10, Fields: tc.fields}, func(f *provider.Feature) error { got = append(got, f.Tags); return nil })
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("properties: %#v want %#v", got, tc.want)
			}
			if result.NumberMatched == nil || *result.NumberMatched != 3 || result.NumberReturned != 3 {
				t.Fatalf("counts: %#v", result)
			}
		})
	}
	layer := p.layers["items"]
	decoded, err := p.decodeRow(context.Background(), layer, []string{"id", "geom", "name", "value"}, []any{int64(10), "POINT(1 2)", nil, int64(0)}, rowDecodePolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := decoded.Feature.Tags["name"]; exists {
		t.Fatal("shared tile decoder changed NULL omission")
	}
	if decoded.Feature.Tags["value"] != int64(0) {
		t.Fatal("zero changed")
	}
}

func TestFeatureCustomSQLCapabilityRejected(t *testing.T) {
	for _, custom := range []bool{false, true} {
		name := "ordinary"
		conf := map[string]interface{}{"fields": []string{"name"}}
		if custom {
			name = "custom unsupported"
			conf["sql"] = "SELECT id,geom,name FROM items WHERE id=10"
		}
		t.Run(name, func(t *testing.T) {
			p, db := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT,name TEXT)", conf)
			if _, err := db.Exec("INSERT INTO items VALUES(10,'POINT(1 2)',NULL)"); err != nil {
				t.Fatal(err)
			}
			capabilityErr := p.layers["items"].FeatureQuerySupported()
			calls := 0
			result, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10}, func(f *provider.Feature) error {
				calls++
				if !reflect.DeepEqual(f.Tags, map[string]any{"name": nil}) {
					t.Fatalf("ordinary selected NULL: %#v", f.Tags)
				}
				return nil
			})
			if custom {
				if !errors.Is(capabilityErr, provider.ErrUnsupported) || !errors.Is(err, provider.ErrUnsupported) {
					t.Fatalf("custom capability=%v query=%v", capabilityErr, err)
				}
				if calls != 0 || result.NumberReturned != 0 {
					t.Fatalf("unsupported source delivered features: calls=%d result=%#v", calls, result)
				}
				return
			}
			if capabilityErr != nil || err != nil || calls != 1 || result.NumberMatched == nil || *result.NumberMatched != 1 {
				t.Fatalf("ordinary capability=%v query=%v calls=%d result=%#v", capabilityErr, err, calls, result)
			}
		})
	}
}
