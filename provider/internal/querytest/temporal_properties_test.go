package querytest

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/alexeydott/tegola/provider"
)

func TestPublicTemporalPropertyStaticOracles(t *testing.T) {
	// Epoch constants are independently fixed calendar values for 2026-10-01
	// 10:00/12:00 UTC, not obtained from the production temporal converter.
	for _, tc := range []struct {
		storage    TemporalPropertyStorage
		start, end int64
	}{
		{UnixSeconds, 1790848800, 1790856000},
		{UnixMilliseconds, 1790848800000, 1790856000000},
		{UnixMicroseconds, 1790848800000000, 1790856000000000},
		{UnixNanoseconds, 1790848800000000000, 1790856000000000000},
	} {
		fixture := canonicalFixture()
		before := cloneFixture(fixture)
		profile := TemporalPropertyProfile{StartField: "start_time", EndField: "end_time", Storage: tc.storage}
		got, err := withPublicTemporalProperties(fixture, profile)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range got.Rows {
			if row.Feature.ID == 10 && (row.Feature.Tags["start_time"] != tc.start || row.Feature.Tags["end_time"] != tc.end) {
				t.Fatal("fixed epoch differs", row.Feature.Tags)
			}
			if row.Feature.ID == 40 {
				start, okStart := row.Feature.Tags["start_time"]
				end, okEnd := row.Feature.Tags["end_time"]
				if !okStart || !okEnd || start != nil || end != nil {
					t.Fatal("NULL public aliases not preserved", row.Feature.Tags)
				}
			}
		}
		if !reflect.DeepEqual(fixture, before) {
			t.Fatal("ordinary fixture mutated")
		}
		got.Rows[0].Feature.Tags["name"] = "modified"
		*got.Rows[0].Start = got.Rows[0].Start.Add(time.Hour)
		if !reflect.DeepEqual(fixture, before) {
			t.Fatal("profile fixture retains mutable original storage")
		}
	}
}

func TestPublicTemporalPropertyExactStorage(t *testing.T) {
	negative := time.Unix(-1, 1000)
	for _, tc := range []struct {
		storage TemporalPropertyStorage
		want    int64
		valid   bool
	}{
		{UnixSeconds, 0, false}, {UnixMilliseconds, 0, false},
		{UnixMicroseconds, -999999, true}, {UnixNanoseconds, -999999000, true},
	} {
		got, err := fixtureTemporalInteger(&negative, tc.storage)
		if (err == nil) != tc.valid || (tc.valid && got != tc.want) {
			t.Fatal("negative exact storage", got, err)
		}
	}
	far := time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := fixtureTemporalInteger(&far, UnixNanoseconds); err == nil {
		t.Fatal("overflow silently narrowed")
	}
	for _, profile := range []TemporalPropertyProfile{{}, {"same", "same", UnixSeconds}, {"name", "end_time", UnixSeconds}, {"start_time", "end_time", 0}} {
		copy, err := copyTemporalPropertyProfile(&profile)
		if err == nil {
			_, err = withPublicTemporalProperties(canonicalFixture(), *copy)
		}
		if err == nil {
			t.Fatal("invalid or colliding property profile accepted")
		}
	}
	profile := &TemporalPropertyProfile{"start_time", "end_time", UnixSeconds}
	copy, err := copyTemporalPropertyProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	profile.StartField = "mutated"
	if copy.StartField != "start_time" {
		t.Fatal("caller profile retained")
	}
	if copy, err := copyTemporalPropertyProfile(nil); err != nil || copy != nil {
		t.Fatal("ordinary nil option changed")
	}
}

// Reference deliveries use fixed IDs/values. They prove checker behavior only,
// not provider filtering or database parity.
func TestPublicTemporalPropertyCheckerControls(t *testing.T) {
	fixture, err := withPublicTemporalProperties(canonicalFixture(), TemporalPropertyProfile{
		StartField: "start_time", EndField: "end_time", Storage: UnixSeconds,
	})
	if err != nil {
		t.Fatal(err)
	}
	c := caseSpec{fixture: fixture, query: provider.FeatureQuery{Limit: 100}, ids: []uint64{10, 20, 30, 40}, matched: 4}
	features := []provider.Feature{}
	for _, id := range c.ids {
		for _, row := range fixture.Rows {
			if row.Feature.ID == id {
				features = append(features, cloneFeature(row.Feature))
			}
		}
	}
	count := uint64(4)
	result := provider.FeatureQueryResult{NumberReturned: 4, NumberMatched: &count}
	if issues := checkResponse(result, features, c, ExactCount); len(issues) != 0 {
		t.Fatal(issues)
	}
	for _, change := range []func([]provider.Feature){
		func(f []provider.Feature) { delete(f[0].Tags, "start_time") },
		func(f []provider.Feature) { f[0].Tags["start_time"] = int64(1790848801) },
		func(f []provider.Feature) {
			f[0].Tags["start_time"], f[0].Tags["end_time"] = f[0].Tags["end_time"], f[0].Tags["start_time"]
		},
		func(f []provider.Feature) { delete(f[3].Tags, "end_time") },
	} {
		bad := make([]provider.Feature, len(features))
		for i, f := range features {
			bad[i] = cloneFeature(f)
		}
		change(bad)
		if len(checkResponse(result, bad, c, ExactCount)) == 0 {
			t.Fatal("wrong projected temporal properties accepted")
		}
	}
	projected := c
	projected.query.Fields = []string{"name"}
	delivery := make([]provider.Feature, len(features))
	for i, f := range features {
		delivery[i] = cloneFeature(f)
		delivery[i].Tags = map[string]any{"name": f.Tags["name"]}
	}
	if issues := checkResponse(result, delivery, projected, ExactCount); len(issues) != 0 {
		t.Fatal("explicit fields subset changed", issues)
	}
	if len(checkResponse(result, features, projected, ExactCount)) == 0 {
		t.Fatal("unrequested temporal properties leaked into subset")
	}
}

func TestPublicTemporalPropertyConcurrentChecker(t *testing.T) {
	fixture, err := withPublicTemporalProperties(canonicalFixture(), TemporalPropertyProfile{
		StartField: "start_time", EndField: "end_time", Storage: UnixSeconds,
	})
	if err != nil {
		t.Fatal(err)
	}
	querier := brokenQuerier{run: func(_ context.Context, _ provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		for _, id := range []uint64{10, 20} {
			for _, row := range fixture.Rows {
				if row.Feature.ID == id {
					f := cloneFeature(row.Feature)
					if err := fn(&f); err != nil {
						return provider.FeatureQueryResult{}, err
					}
				}
			}
		}
		count := uint64(3)
		return provider.FeatureQueryResult{NumberReturned: 2, NumberMatched: &count, HasMore: true}, nil
	}}
	if issues := checkConcurrentWithFixture(Instance{Querier: querier, CountMode: ExactCount}, fixture); len(issues) != 0 {
		t.Fatal("profiled concurrent oracle differs", issues)
	}
}
