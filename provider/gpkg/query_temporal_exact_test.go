//go:build cgo

package gpkg

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/alexeydott/tegola/provider"
)

func TestExactEpochBoundFractionsAndLeap(t *testing.T) {
	for _, scale := range []int64{1, 1000, 1000000, 1000000000} {
		for _, tc := range []struct {
			name        string
			t           time.Time
			fraction    string
			leap        bool
			floor, ceil string
		}{
			{"tenth decimal", time.Unix(0, 0), "1", false, "0", "1"},
			{"negative fraction", time.Unix(-1, 999999999), "1", false, "-1", "0"},
			{"zero fraction", time.Unix(0, 0), "000", false, "0", "0"},
			{"inserted second", time.Date(2016, 12, 31, 23, 59, 59, 123, time.UTC), "12345", true, fmt.Sprint(int64(1483228800)*scale - 1), fmt.Sprint(int64(1483228800) * scale)},
		} {
			t.Run(fmt.Sprintf("%s/%d", tc.name, scale), func(t *testing.T) {
				if got := exactEpochBound(tc.t, scale, false, tc.fraction, tc.leap).String(); got != tc.floor {
					t.Fatalf("floor %s want%s", got, tc.floor)
				}
				if got := exactEpochBound(tc.t, scale, true, tc.fraction, tc.leap).String(); got != tc.ceil {
					t.Fatalf("ceil %s want%s", got, tc.ceil)
				}
			})
		}
	}
	target := time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC)
	bound := exactEpochBound(target, 1000000000, true, "1", false)
	var args []any
	if got := integerBoundSQL("ts", ">=", bound, &args); got != "0" || len(args) != 0 {
		t.Fatalf("overflow lower %s args%v", got, args)
	}
	if got := integerBoundSQL("ts", "<=", bound, &args); got != "1" || len(args) != 0 {
		t.Fatalf("overflow upper %s args%v", got, args)
	}
}

func TestTemporalExactIntegerSourceQueries(t *testing.T) {
	for _, profile := range []struct {
		name  string
		scale int64
	}{{"unix_seconds", 1}, {"unix_milliseconds", 1000}, {"unix_microseconds", 1000000}, {"unix_nanoseconds", 1000000000}} {
		t.Run(profile.name, func(t *testing.T) {
			leap := time.Date(2016, 12, 31, 23, 59, 59, 123, time.UTC)
			next := int64(1483228800) * profile.scale
			ddl := fmt.Sprintf("CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT,ts INTEGER,te INTEGER); INSERT INTO items VALUES(1,NULL,%d,%d),(2,NULL,%d,%d),(3,NULL,%d,%d),(4,NULL,NULL,NULL)", next-1, next, next-1, next-1, next, next)
			interval, _ := queryTestProvider(t, ddl, map[string]interface{}{"temporal_start_field": "ts", "temporal_end_field": "te", "temporal_storage": profile.name})
			constraint := &provider.TemporalConstraint{Start: &leap, End: &leap, StartLeapSecond: true, EndLeapSecond: true, StartSubNanosecond: "456", EndSubNanosecond: "456"}
			ids, result := queryIDs(t, interval, provider.FeatureQuery{Limit: 10, Temporal: constraint})
			if !reflect.DeepEqual(ids, []uint64{1, 4}) || result.NumberMatched == nil || *result.NumberMatched != 2 {
				t.Fatalf("leap intervals%v result%+v", ids, result)
			}
			instant, _ := queryTestProvider(t, ddl, map[string]interface{}{"temporal_field": "ts", "temporal_storage": profile.name})
			ids, _ = queryIDs(t, instant, provider.FeatureQuery{Limit: 10, Temporal: constraint})
			if !reflect.DeepEqual(ids, []uint64{4}) {
				t.Fatalf("leap has no POSIX instant: %v", ids)
			}
			for _, date := range []time.Time{time.Unix(0, 0), time.Unix(-1, 999999999)} {
				fractional, _ := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT,ts INTEGER,te INTEGER); INSERT INTO items VALUES(1,NULL,-1,1),(2,NULL,0,0),(3,NULL,NULL,NULL)", map[string]interface{}{"temporal_start_field": "ts", "temporal_end_field": "te", "temporal_storage": profile.name})
				constraint = &provider.TemporalConstraint{Start: &date, End: &date, StartSubNanosecond: "1", EndSubNanosecond: "1"}
				ids, _ = queryIDs(t, fractional, provider.FeatureQuery{Limit: 10, Temporal: constraint})
				if !reflect.DeepEqual(ids, []uint64{1, 3}) {
					t.Fatalf("subtick interval must not be globally empty: %v", ids)
				}
				point, _ := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT,ts INTEGER); INSERT INTO items VALUES(1,NULL,-1),(2,NULL,0),(3,NULL,1),(4,NULL,NULL)", map[string]interface{}{"temporal_field": "ts", "temporal_storage": profile.name})
				ids, _ = queryIDs(t, point, provider.FeatureQuery{Limit: 10, Temporal: constraint})
				if !reflect.DeepEqual(ids, []uint64{4}) {
					t.Fatalf("subtick instant must not round to source: %v", ids)
				}
			}
		})
	}
}
