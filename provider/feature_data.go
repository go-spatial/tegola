package provider

import "fmt"

// FeatureDataError distinguishes corrupt source rows from invalid client
// queries. Its underlying error remains available to errors.Is/As; HTTP
// adapters must return a generic server error rather than expose source data.
type FeatureDataError struct{ Err error }

func (e FeatureDataError) Error() string {
	return fmt.Sprintf("provider: invalid feature source data: %v", e.Err)
}

func (e FeatureDataError) Unwrap() error { return e.Err }
