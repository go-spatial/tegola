//go:build cgo

package gpkg

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

func TestFeatureSourceDataClassification(t *testing.T) {
	for _, tc := range []struct {
		name, ddl string
		conf      map[string]interface{}
	}{
		{"negative identity", "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT); INSERT INTO items VALUES(-1,'POINT (0 0)')", nil},
		{"malformed geometry", "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT); INSERT INTO items VALUES(1,'POINT broken')", nil},
		{"source dimension disagreement", "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT); INSERT INTO items VALUES(1,'POINT Z (0 0 1)')", nil},
		{"unsupported source wire", "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT); INSERT INTO items VALUES(1,'POINT M (0 0 1)')", nil},
		{"noninteger source time", "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT,ts INTEGER); INSERT INTO items VALUES(1,NULL,1.5)", map[string]interface{}{"temporal_field": "ts", "temporal_storage": "unix_seconds"}},
		{"reversed source time", "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT,ts INTEGER,te INTEGER); INSERT INTO items VALUES(1,NULL,2,1)", map[string]interface{}{"temporal_start_field": "ts", "temporal_end_field": "te", "temporal_storage": "unix_seconds"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := queryTestProvider(t, tc.ddl, tc.conf)
			_, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10}, func(*provider.Feature) error { t.Fatal("corrupt source delivered"); return nil })
			var data provider.FeatureDataError
			if !errors.As(err, &data) {
				t.Fatalf("source classification missing: %v", err)
			}
			if data.Err == nil {
				t.Fatal("source error chain missing")
			}
		})
	}
	p, _ := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT)", nil)
	_, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{}, func(*provider.Feature) error { return nil })
	var data provider.FeatureDataError
	var invalid provider.InvalidFeatureQueryError
	if !errors.As(err, &invalid) || errors.As(err, &data) {
		t.Fatalf("invalid request classification changed: %v", err)
	}
	for _, cancel := range []error{context.Canceled, context.DeadlineExceeded} {
		wrapped := featureSourceError(fmt.Errorf("decoder: %w", cancel))
		if !errors.Is(wrapped, cancel) || errors.As(wrapped, &data) {
			t.Fatalf("cancellation classification changed: %v", wrapped)
		}
	}
}
