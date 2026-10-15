package features

import (
	"errors"
	"reflect"
	"testing"
)

func TestPublicCatalogDetachedMetadata(t *testing.T) {
	sources := []CollectionSource{
		{ID: "z", Title: "Z title", Description: "Z description", Layer: &testLayer{name: "source", srid: 4326}, Querier: testQuerier(emptyQuerier)},
		{ID: "a", Title: "A title", Layer: &testLayer{name: "source", srid: 4326}, Querier: testQuerier(emptyQuerier)},
	}
	s, err := NewService(sources)
	if err != nil {
		t.Fatal(err)
	}
	sources[0].Title = "changed"
	got := s.Collections()
	want := []CollectionMetadata{{ID: "a", Title: "A title"}, {ID: "z", Title: "Z title", Description: "Z description"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("catalog %v want %v", got, want)
	}
	got[1].Title = "changed snapshot"
	metadata, err := s.Collection("z")
	if err != nil || metadata != want[1] {
		t.Fatalf("retained snapshot: %v %v", metadata, err)
	}
	metadata.Description = "changed value"
	if !reflect.DeepEqual(s.Collections(), want) {
		t.Fatal("value mutation leaked")
	}
	_, err = s.Collection("missing")
	var missing CollectionNotFoundError
	if !errors.As(err, &missing) || missing.CollectionID != "missing" {
		t.Fatalf("missing classification: %v", err)
	}
}
