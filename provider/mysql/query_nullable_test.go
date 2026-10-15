package mysql

import (
	"reflect"
	"testing"
)

func TestFeatureSelectedNullableProperties(t *testing.T) {
	f := testFeatureProfile(t, false)
	for _, value := range []any{nil, "", "road"} {
		feature, match, representable, err := f.decodeFeature([]string{"id", "geom", "name", "start"}, []any{int64(1), "POINT(1 2)", value, nil}, nil)
		if err != nil || !match || !representable {
			t.Fatalf("decode: %v", err)
		}
		if !reflect.DeepEqual(feature.Tags, map[string]any{"name": value}) {
			t.Fatalf("properties: %#v", feature.Tags)
		}
	}
	feature, _, _, err := f.decodeFeature([]string{"id", "geom"}, []any{int64(1), nil}, nil)
	if err != nil || len(feature.Tags) != 0 {
		t.Fatalf("unselected properties: %#v %v", feature.Tags, err)
	}
}
