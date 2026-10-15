//go:build cgo

package provider_test

import (
	"context"
	"testing"

	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/gpkg"
	"github.com/alexeydott/tegola/provider/hana"
	"github.com/alexeydott/tegola/provider/mysql"
	"github.com/alexeydott/tegola/provider/postgis"
)

func TestUnconfiguredFeatureProviderRejectsFilteredQuery(t *testing.T) {
	expression, err := provider.NewFilterExpression(provider.FilterNode{
		Kind: provider.FilterBooleanConstant, Boolean: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	query := provider.FeatureQuery{Limit: 1, Filter: &expression}
	backends := map[string]provider.FeatureQuerier{
		"gpkg":    &gpkg.Provider{},
		"mysql":   &mysql.Provider{},
		"hana":    &hana.Provider{},
		"postgis": &postgis.Provider{},
	}
	for name, backend := range backends {
		t.Run(name, func(t *testing.T) {
			calls := 0
			result, err := backend.QueryFeatures(
				context.Background(),
				"items",
				query,
				func(*provider.Feature) error {
					calls++
					return nil
				},
			)
			if err == nil || calls != 0 || result.NumberReturned != 0 {
				t.Fatalf("unconfigured source admitted: result=%#v calls=%d err=%v", result, calls, err)
			}
			if expression.Root().Boolean || expression.Validate() != nil {
				t.Fatal("query mutated filter")
			}
		})
	}
}
