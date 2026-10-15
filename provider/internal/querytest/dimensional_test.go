package querytest

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
)

// Delivery lists are fixed by each literal case, not computed by a spatial
// predicate. These controls prove checker mechanics, never backend parity.
func dimensionalControl(c caseSpec, shared, mutateQuery bool) Instance {
	features := staticDomainDelivery(c, c.ids)
	return Instance{CountMode: ExactCount, Querier: brokenQuerier{run: func(
		_ context.Context, q provider.FeatureQuery, callback func(*provider.Feature) error,
	) (provider.FeatureQueryResult, error) {
		if mutateQuery && len(q.Bounds3D) != 0 {
			q.Bounds3D[0][0] = -999
		}
		if c.malformed {
			return provider.FeatureQueryResult{}, provider.FeatureDataError{Err: errors.New("literal invalid coordinates")}
		}
		for i := range features {
			feature := cloneFeature(features[i])
			if shared {
				if err := callback(&features[i]); err != nil {
					return provider.FeatureQueryResult{}, err
				}
			} else if err := callback(&feature); err != nil {
				return provider.FeatureQueryResult{}, err
			}
		}
		count := c.matched
		return provider.FeatureQueryResult{
			NumberReturned: uint64(len(features)), NumberMatched: &count, HasMore: c.more,
		}, nil
	}}}
}

func TestDimensionalStaticControls(t *testing.T) {
	for _, c := range dimensionalCases() {
		t.Run(c.name, func(t *testing.T) {
			if issues := checkCase(dimensionalControl(c, false, false), c); len(issues) != 0 {
				t.Fatal(issues)
			}
			if c.malformed {
				return
			}
			count := c.matched
			features := staticDomainDelivery(c, c.ids)
			result := provider.FeatureQueryResult{
				NumberReturned: uint64(len(features)), NumberMatched: &count, HasMore: c.more,
			}
			if len(features) == 0 {
				features = staticDomainDelivery(c, []uint64{10})
				result.NumberReturned = uint64(len(features))
			} else {
				features[0].Geometry = geom.Point{999, 999}
			}
			if len(checkResponse(result, features, c, ExactCount)) == 0 {
				t.Fatal("false positive or lost XYZ accepted")
			}
			if len(c.query.Bounds3D) > 0 && len(checkCase(dimensionalControl(c, false, true), c)) == 0 {
				t.Fatal("mutated XYZ query accepted")
			}
			if c.mutateDelivery && len(checkCase(dimensionalControl(c, true, false), c)) == 0 {
				t.Fatal("nested shared XYZ delivery accepted")
			}
		})
	}
}

func TestDimensionalSnapshotsAreDetached(t *testing.T) {
	for _, c := range dimensionalCases() {
		if c.malformed {
			continue
		} // NaN is intentionally not reflect-equal to itself.
		before := cloneFixture(c.fixture)
		copy := cloneFixture(c.fixture)
		for _, row := range copy.Rows {
			mutateGeometry(row.Feature.Geometry)
		}
		if !reflect.DeepEqual(c.fixture, before) {
			t.Fatalf("fixture ownership: %s", c.name)
		}
		query := cloneQuery(c.query)
		if len(query.Bounds3D) > 0 {
			query.Bounds3D[0][0] = -999
			if c.query.Bounds3D[0][0] == -999 {
				t.Fatal("Bounds3D shared")
			}
		}
	}
}

func TestDimensionalErrorControls(t *testing.T) {
	for _, c := range dimensionalCases() {
		if !c.malformed {
			continue
		}
		for _, err := range []error{nil, errors.New("wrong classification"), provider.ErrUnsupported} {
			bad := Instance{Querier: brokenQuerier{run: func(
				_ context.Context, _ provider.FeatureQuery, callback func(*provider.Feature) error,
			) (provider.FeatureQueryResult, error) {
				if err == nil {
					feature := cloneFeature(c.fixture.Rows[0].Feature)
					_ = callback(&feature)
				}
				return provider.FeatureQueryResult{}, err
			}}}
			if c.malformed && !c.sourceData && err != nil {
				continue
			}
			if len(checkCase(bad, c)) == 0 {
				t.Fatal("invalid source control accepted")
			}
		}
	}
}

func TestDimensionalConcurrentControls(t *testing.T) {
	for _, c := range dimensionalCases() {
		if !c.mutateDelivery {
			continue
		}
		if issues := checkDimensionalConcurrent(dimensionalControl(c, false, false), c); len(issues) != 0 {
			t.Fatal(issues)
		}
		wrong := c
		wrong.matched++
		if len(checkDimensionalConcurrent(dimensionalControl(wrong, false, false), c)) == 0 {
			t.Fatal("concurrent wrong count escaped diagnostics")
		}
	}
}

func TestRawWhollyEmptyNegativeControls(t *testing.T) {
	for _, c := range dimensionalCases() {
		if c.fixture.Rows[0].RawEmptyGeometry == nil {
			continue
		}
		if _, err := EncodeFixtureWKB(c.fixture.Rows[0].RawEmptyGeometry); err != nil {
			t.Fatal(err)
		}
		count := uint64(0)
		if len(checkResponse(provider.FeatureQueryResult{NumberMatched: &count}, nil, c, ExactCount)) == 0 {
			t.Fatal("wholly empty treated as nonmatch")
		}
		wrong := Instance{Querier: brokenQuerier{run: func(
			_ context.Context, _ provider.FeatureQuery, _ func(*provider.Feature) error,
		) (provider.FeatureQueryResult, error) {
			return provider.FeatureQueryResult{}, errors.New("empty rejected")
		}}}
		if len(checkCase(wrong, c)) == 0 {
			t.Fatal("wholly empty rejected without diagnostic")
		}
		copy := cloneFixture(c.fixture)
		if collection, ok := copy.Rows[0].RawEmptyGeometry.(geom.Collection); ok {
			collection[0] = geom.PointZ{99, 99, 99}
			if reflect.DeepEqual(collection, c.fixture.Rows[0].RawEmptyGeometry) {
				t.Fatal("raw source descriptor shared")
			}
		}
	}
}
