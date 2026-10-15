package querytest

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexeydott/tegola/provider"
)

func TestFixtureTemporalIndependentIntegerGoldens(t *testing.T) {
	for _, tc := range []struct {
		unit  TemporalPropertyStorage
		value time.Time
		want  int64
	}{
		{UnixSeconds, time.Unix(-1, 0), -1}, {UnixMilliseconds, time.Unix(-1, 999000000), -1},
		{UnixMicroseconds, time.Unix(-1, 999999000), -1}, {UnixNanoseconds, time.Unix(-1, 999999999), -1},
		{UnixSeconds, time.Unix(1483228799, 0), 1483228799},
		{UnixMilliseconds, time.Unix(1483228799, 0), 1483228799000},
		{UnixMicroseconds, time.Unix(1483228799, 0), 1483228799000000},
		{UnixNanoseconds, time.Unix(1483228799, 0), 1483228799000000000},
	} {
		got, err := EncodeFixtureTemporal(tc.value, tc.unit)
		if err != nil || got != tc.want {
			t.Fatalf("unit%d got%d err%v want%d", tc.unit, got, err, tc.want)
		}
	}
	for _, unit := range []TemporalPropertyStorage{UnixSeconds, UnixMilliseconds, UnixMicroseconds, UnixNanoseconds} {
		for _, edge := range []int64{math.MinInt64, math.MaxInt64} {
			value := timeFromFixtureInteger(edge, unit)
			got, err := EncodeFixtureTemporal(value, unit)
			if err != nil || got != edge {
				t.Fatalf("source tick inverse unit%d edge%d got%d err%v", unit, edge, got, err)
			}
		}
		if unit != UnixNanoseconds {
			if _, err := EncodeFixtureTemporal(time.Unix(0, 1), unit); err == nil {
				t.Fatal("lossy source flooring accepted")
			}
		}
	}
	if _, err := EncodeFixtureTemporal(time.Unix(9223372037, 0), UnixNanoseconds); err == nil {
		t.Fatal("source overflow accepted")
	}
	if _, err := EncodeFixtureTemporal(time.Unix(0, 0), 0); err == nil {
		t.Fatal("unknown unit accepted")
	}
}

func exactTemporalControl(c caseSpec) Instance {
	if c.invalidQuery {
		return Instance{Querier: brokenQuerier{run: func(_ context.Context, _ provider.FeatureQuery,
			_ func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
			return provider.FeatureQueryResult{}, provider.InvalidFeatureQueryError{Field: "temporal", Reason: "static control"}
		}}}
	}
	return dimensionalControl(c, false, false)
}

func TestExactTemporalStaticControls(t *testing.T) {
	for _, unit := range []TemporalPropertyStorage{UnixSeconds, UnixMilliseconds, UnixMicroseconds, UnixNanoseconds} {
		for _, c := range exactTemporalCases(unit) {
			t.Run(c.name+unitName(unit), func(t *testing.T) {
				if issues := checkCase(exactTemporalControl(c), c); len(issues) != 0 {
					t.Fatal(issues)
				}
				if c.invalidQuery || c.malformed {
					bad := Instance{Querier: brokenQuerier{run: func(_ context.Context, _ provider.FeatureQuery,
						_ func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
						return provider.FeatureQueryResult{}, provider.ErrUnsupported
					}}}
					if len(checkCase(bad, c)) == 0 {
						t.Fatal("query unsupported masked invalid/source error")
					}
					return
				}
				// A truncating implementation incorrectly admits first ordinary source
				// instant at a tail/leap boundary or fabricates membership/count.
				wrong := staticDomainDelivery(c, []uint64{10, 90})
				count := uint64(99)
				if len(checkResponse(provider.FeatureQueryResult{NumberReturned: 2, NumberMatched: &count}, wrong, c, ExactCount)) == 0 {
					t.Fatal("incorrect exact-time result accepted")
				}
				if strings.Contains(c.name, "tail") || strings.Contains(c.name, "tenth") ||
					strings.Contains(c.name, "leap instant") || strings.Contains(c.name, "leap fraction") {
					if len(c.ids) == 1 && c.ids[0] == 90 {
						collapsed := staticDomainDelivery(c, []uint64{10})
						count := uint64(1)
						result := provider.FeatureQueryResult{NumberReturned: 1, NumberMatched: &count}
						if len(checkResponse(result, collapsed, c, ExactCount)) == 0 {
							t.Fatal("collapsed tail/leap same-count membership accepted")
						}
						result.NumberMatched = nil
						if len(checkResponse(result, collapsed, c, UnknownCount)) == 0 {
							t.Fatal("collapsed tail/leap accepted with unknown count")
						}
					}
				}
			})
		}
	}
}

func TestRawTemporalSourceAndPublicOracle(t *testing.T) {
	for _, unit := range []TemporalPropertyStorage{UnixSeconds, UnixMilliseconds, UnixMicroseconds, UnixNanoseconds} {
		for _, c := range exactTemporalCases(unit) {
			if c.fixture.Rows[0].RawTemporal == nil {
				continue
			}
			profile := TemporalPropertyProfile{StartField: "start", EndField: "end", Storage: unit}
			copy, err := withPublicTemporalProperties(c.fixture, profile)
			if err != nil {
				t.Fatal(err)
			}
			if copy.Rows[0].Feature.Tags["start"] != int64(math.MinInt64) ||
				copy.Rows[1].Feature.Tags["end"] != int64(math.MaxInt64) || copy.Rows[2].Feature.Tags["start"] != nil {
				t.Fatal("raw integer public oracle changed extrema/NULL")
			}
			*copy.Rows[0].RawTemporal.Start = 0
			if *c.fixture.Rows[0].RawTemporal.Start != math.MinInt64 {
				t.Fatal("raw temporal source shared")
			}
			undeclared := cloneFixture(c.fixture)
			undeclared.TemporalStorage = 0
			if _, err := withPublicTemporalProperties(undeclared, profile); err == nil {
				t.Fatal("raw ticks accepted without declared storage")
			}
			// Ignoring literal source ticks treats both extrema as absent-time rows.
			ignored := staticDomainDelivery(c, []uint64{10, 20, 90})
			count := uint64(3)
			if len(checkResponse(provider.FeatureQueryResult{NumberReturned: 3, NumberMatched: &count}, ignored, c, ExactCount)) == 0 {
				t.Fatal("ignored raw temporal source accepted")
			}
		}
	}
}

func unitName(unit TemporalPropertyStorage) string {
	switch unit {
	case UnixSeconds:
		return " seconds"
	case UnixMilliseconds:
		return " milliseconds"
	case UnixMicroseconds:
		return " microseconds"
	default:
		return " nanoseconds"
	}
}

func TestExactTemporalQueryOwnershipAndValidation(t *testing.T) {
	for _, c := range exactTemporalCases(UnixNanoseconds) {
		copy := cloneQuery(c.query)
		before := cloneQuery(c.query)
		if copy.Temporal != nil {
			copy.Temporal.StartSubNanosecond = "999"
			if copy.Temporal.Start != nil {
				*copy.Temporal.Start = time.Unix(10, 0)
			}
		}
		if !reflect.DeepEqual(c.query, before) {
			t.Fatal("temporal fixture shares endpoints")
		}
		err := c.query.Validate()
		var invalid provider.InvalidFeatureQueryError
		if c.invalidQuery != errors.As(err, &invalid) {
			t.Fatalf("literal validity %s: %v", c.name, err)
		}
	}
}
