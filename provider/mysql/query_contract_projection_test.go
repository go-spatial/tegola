package mysql

import (
	"reflect"
	"testing"
)

func TestFeatureContractEmptyCustomProjectionKeepsTemporalMapping(t *testing.T) {
	for _, test := range []struct {
		name     string
		public   []string
		filtered bool
		want     []string
	}{
		{"empty", []string{}, false, []string{"s.source_id AS id", "s.source_geom AS geom", "s.source_start AS start_time", "s.source_end AS end_time"}},
		{"subset", []string{"name", "start_time", "end_time"}, false, []string{"s.source_id AS id", "s.source_geom AS geom", "s.source_name AS name", "s.source_start AS start_time", "s.source_end AS end_time"}},
		{"legacy", nil, false, []string{"s.source_id AS id", "s.source_geom AS geom", "s.source_name AS name", "s.source_value AS value", "s.source_start AS start_time", "s.source_end AS end_time"}},
		{"filters", []string{"b", "n", "s", "start_time", "end_time"}, true, []string{"s.source_id AS id", "s.source_geom AS geom", "s.source_start AS start_time", "s.source_end AS end_time", "s.source_n AS n", "s.source_s AS s", "s.source_b AS b"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := append([]string(nil), test.public...)
			projection := featureLiveContractProjection(test.public, test.filtered)
			if !reflect.DeepEqual(projection, test.want) {
				t.Fatalf("projection %v want %v", projection, test.want)
			}
			if len(test.public) != len(before) {
				t.Fatal("input property selection changed")
			}
			for i := range before {
				if before[i] != test.public[i] {
					t.Fatal("input property selection changed")
				}
			}
			projection[0] = "mutated"
			if reflect.DeepEqual(featureLiveContractProjection(test.public, test.filtered), projection) {
				t.Fatal("projection output retained")
			}
		})
	}
}
