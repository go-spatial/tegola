package features

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

func TestGeoJSONSelectedNullablePropertyPresence(t *testing.T) {
	for _, tc := range []struct {
		name string
		tags map[string]any
		want map[string]string
	}{
		{"selected null and zero", map[string]any{"name": nil, "value": int64(0)}, map[string]string{"name": "null", "value": "0"}},
		{"empty string and selected null", map[string]any{"name": "", "value": nil}, map[string]string{"name": `""`, "value": "null"}},
		{"false remains false", map[string]any{"flag": false}, map[string]string{"flag": "false"}},
		{"selected null sibling absent", map[string]any{"name": nil}, map[string]string{"name": "null"}},
		{"empty properties", map[string]any{}, map[string]string{}},
		{"nil source properties", nil, map[string]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := &provider.Feature{ID: 10, SRID: 4326, Tags: tc.tags}
			var before map[string]any
			if source.Tags != nil {
				before = make(map[string]any, len(source.Tags))
				for key, value := range source.Tags {
					before[key] = value
				}
			}
			s := newTestService(t, 4326, featureQuerier(source))
			page, err := s.QueryCollectionPage(context.Background(), "public", provider.FeatureQuery{Limit: 1})
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := WriteGeoJSON(context.Background(), &out, page); err != nil {
				t.Fatal(err)
			}
			var document struct {
				Features []struct {
					Properties json.RawMessage `json:"properties"`
				} `json:"features"`
			}
			if err := json.Unmarshal(out.Bytes(), &document); err != nil || len(document.Features) != 1 {
				t.Fatal("invalid feature page", err, out.String())
			}
			var properties map[string]json.RawMessage
			if err := json.Unmarshal(document.Features[0].Properties, &properties); err != nil {
				t.Fatal(err)
			}
			if properties == nil || len(properties) != len(tc.want) {
				t.Fatal("selected null and absent/empty properties conflated", string(document.Features[0].Properties))
			}
			for key, want := range tc.want {
				value, present := properties[key]
				if !present || string(value) != want {
					t.Fatalf("property %s presence/value: %v %s, want %s", key, present, value, want)
				}
			}
			if len(tc.want) == 0 && !bytes.Equal(document.Features[0].Properties, []byte("{}")) {
				t.Fatal("empty properties are not an object", string(document.Features[0].Properties))
			}
			if value, present := source.Tags["name"]; present && value == nil {
				page.Features[0].Properties["name"] = "changed"
				if value, present := source.Tags["name"]; !present || value != nil {
					t.Fatal("nullable property snapshot aliases source")
				}
			}
			if !reflect.DeepEqual(source.Tags, before) {
				t.Fatal("source property map changed")
			}
		})
	}
}
