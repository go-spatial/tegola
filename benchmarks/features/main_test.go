//go:build cgo

package main

import (
	"encoding/json"
	"testing"

	"github.com/alexeydott/tegola/ogc/features"
)

func TestAnalyticGridOracleLiteralAndNegativeControls(t *testing.T) {
	ids, total := expectedIDs(10000, "bbox_selective", 10)
	want := []uint64{2101, 2102, 2103, 3101, 3102, 3103, 4101, 4102, 4103}
	if total != 9 || len(ids) != len(want) {
		t.Fatal(ids, total)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatal(ids)
		}
	}
	count := uint64(1)
	valid := func() features.FeatureCollection {
		return features.FeatureCollection{Type: "FeatureCollection", NumberReturned: 1, NumberMatched: &count, Features: []features.Feature{{Type: "Feature", ID: 1, Geometry: json.RawMessage(`{"type":"Point","coordinates":[0,0]}`), Properties: map[string]any{"n": json.Number("1"), "s": "value"}}}}
	}
	assertPage(valid(), []uint64{1}, 1, "id")
	for _, mutate := range []func(*features.FeatureCollection){
		func(p *features.FeatureCollection) { p.Features[0].ID = 2 },
		func(p *features.FeatureCollection) {
			p.Features[0].Geometry = json.RawMessage(`{"type":"Point","coordinates":[1,0]}`)
		},
		func(p *features.FeatureCollection) {
			p.Features[0].Geometry = json.RawMessage(`{"type":"Point","coordinates":[0,0,0]}`)
		},
		func(p *features.FeatureCollection) { p.HasMore = true },
		func(p *features.FeatureCollection) { p.NumberMatched = nil },
		func(p *features.FeatureCollection) { p.Features[0].Properties["n"] = nil },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("broken semantic result escaped oracle")
				}
			}()
			page := valid()
			mutate(&page)
			assertPage(page, []uint64{1}, 1, "id")
		}()
	}
}

func TestPerformanceBudgetFormulaAndNegativeControls(t *testing.T) {
	ref := map[string]measurement{key(10000, "id"): {Median: .1, Maximum: .2, Allocation: 1000}}
	if err := enforce(measurement{Dataset: 10000, Class: "id", Median: 1, Maximum: 2, Allocation: 5296}, ref); err != nil {
		t.Fatal(err)
	}
	for _, value := range []measurement{
		{Dataset: 10000, Class: "id", Median: 1.01},
		{Dataset: 10000, Class: "id", Maximum: 2.01},
		{Dataset: 10000, Class: "id", Allocation: 5297},
		{Dataset: 10000, Class: "missing"},
	} {
		if enforce(value, ref) == nil {
			t.Fatal("performance regression escaped", value)
		}
	}
	ref = map[string]measurement{key(10000000, "bbox_broad"): {Median: 26000, Maximum: 27000, Allocation: 1000}}
	if enforce(measurement{Dataset: 10000000, Class: "bbox_broad", Median: 30001}, ref) == nil {
		t.Fatal("operational timeout ceiling escaped")
	}
}
