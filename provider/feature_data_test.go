package provider

import (
	"errors"
	"testing"
)

func TestFeatureDataErrorPreservesUnderlyingType(t *testing.T) {
	underlying := InvalidFeatureQueryError{Field: "temporal", Reason: "source is not an integer"}
	err := FeatureDataError{Err: underlying}
	var data FeatureDataError
	var invalid InvalidFeatureQueryError
	if !errors.As(err, &data) || !errors.As(err, &invalid) {
		t.Fatal("source error chain lost")
	}
}
