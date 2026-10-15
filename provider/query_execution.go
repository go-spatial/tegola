package provider

// FeatureQueryBackend is a bounded implementation identity, never a configured name.
type FeatureQueryBackend uint8

const (
	FeatureQueryBackendUnknown FeatureQueryBackend = iota
	FeatureQueryBackendGPKG
	FeatureQueryBackendPostGIS
	// FeatureQueryBackendMySQL includes the shared MySQL/MariaDB implementation.
	FeatureQueryBackendMySQL
	FeatureQueryBackendHANA
)

// FeatureFilterExecution describes how an admitted scalar filter is executed.
// SQL does not imply indexed execution or SQL-only spatial/temporal validation.
type FeatureFilterExecution uint8

const (
	FeatureFilterExecutionUnknown FeatureFilterExecution = iota
	FeatureFilterExecutionSQL
)

type FeatureQueryExecutionMetadata struct {
	Backend      FeatureQueryBackend
	ScalarFilter FeatureFilterExecution
}

// FeatureQueryExecutionInfo is optional observability metadata. It must not do I/O.
// Consumers snapshot it once; unavailable or invalid metadata means unknown.
type FeatureQueryExecutionInfo interface {
	FeatureQueryExecutionInfo() (FeatureQueryExecutionMetadata, error)
}
