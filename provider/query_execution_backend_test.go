//go:build cgo

package provider_test

import (
	"testing"

	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/gpkg"
	"github.com/alexeydott/tegola/provider/hana"
	"github.com/alexeydott/tegola/provider/mysql"
	"github.com/alexeydott/tegola/provider/postgis"
)

func TestFeatureQueryExecutionMetadataDoesNotNeedConnection(t *testing.T) {
	for _, test := range []struct {
		getter  provider.FeatureQueryExecutionInfo
		backend provider.FeatureQueryBackend
	}{
		{&gpkg.Provider{}, provider.FeatureQueryBackendGPKG},
		{&mysql.Provider{}, provider.FeatureQueryBackendMySQL},
		{&postgis.Provider{}, provider.FeatureQueryBackendPostGIS},
		{&hana.Provider{}, provider.FeatureQueryBackendHANA},
	} {
		metadata, err := test.getter.FeatureQueryExecutionInfo()
		if err != nil || metadata.Backend != test.backend || metadata.ScalarFilter != provider.FeatureFilterExecutionSQL {
			t.Fatal(metadata, err)
		}
	}
}
