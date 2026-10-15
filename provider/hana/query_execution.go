package hana

import "github.com/alexeydott/tegola/provider"

// FeatureQueryExecutionInfo reports the raw query implementation's scalar SQL path.
// It does not claim indexed execution or SQL-only exact spatial validation.
func (*Provider) FeatureQueryExecutionInfo() (provider.FeatureQueryExecutionMetadata, error) {
	return provider.FeatureQueryExecutionMetadata{Backend: provider.FeatureQueryBackendHANA, ScalarFilter: provider.FeatureFilterExecutionSQL}, nil
}
