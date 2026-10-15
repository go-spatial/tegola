package querytest

import (
	"github.com/alexeydott/tegola/provider"
)

// Only explicitly declared public temporal aliases extend this suite's static
// catalog. Private/default temporal reads never become inferred queryables.
func withFilterTemporalProperties(fixture Fixture, profile TemporalPropertyProfile) (Fixture, error) {
	if fixture.TemporalStorage != 0 {
		profile.Storage = fixture.TemporalStorage
	}
	fields := append([]provider.FeatureQueryable{}, fixture.FilterFields...)
	fields = append(fields,
		provider.FeatureQueryable{Name: profile.StartField, Type: provider.QueryableInteger, Nullable: true},
		provider.FeatureQueryable{Name: profile.EndField, Type: provider.QueryableInteger, Nullable: true},
	)
	catalog, err := provider.NewFeatureQueryables(fields)
	if err != nil {
		return Fixture{}, err
	}
	actual, err := withPublicTemporalProperties(fixture, profile)
	if err != nil {
		return Fixture{}, err
	}
	actual.FilterFields = catalog.Fields()
	return actual, nil
}
