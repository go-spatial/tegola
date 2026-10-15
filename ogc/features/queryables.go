package features

import (
	"errors"
	"fmt"

	"github.com/alexeydott/tegola/ogc/cql2"
	"github.com/alexeydott/tegola/provider"
)

// Queryables returns a detached catalog for the selected CQL2 text transport.
// Core properties and the provider's generic catalog retain their own scope.
func (s *Service) Queryables(collectionID string) (provider.FeatureQueryables, error) {
	collection, ok := s.collections[collectionID]
	if !ok {
		return provider.FeatureQueryables{}, CollectionNotFoundError{CollectionID: collectionID}
	}
	if !collection.queryablesAvailable {
		return provider.FeatureQueryables{}, fmt.Errorf("features: queryables unavailable: %w", provider.ErrUnsupported)
	}
	return provider.NewFeatureQueryables(collection.queryables.Fields())
}

func freezeCollectionQueryables(layer provider.LayerInfo) (provider.FeatureQueryables, bool, error) {
	info, ok := layer.(provider.FeatureQueryableLayerInfo)
	if !ok {
		return provider.FeatureQueryables{}, false, nil
	}
	catalog, err := info.FeatureQueryables()
	if errors.Is(err, provider.ErrUnsupported) {
		return provider.FeatureQueryables{}, false, nil
	}
	if err != nil {
		return provider.FeatureQueryables{}, false, err
	}
	// Validate the complete claimed catalog before allocating a transport subset.
	if err := catalog.Validate(); err != nil {
		return provider.FeatureQueryables{}, false, err
	}
	fields := catalog.Fields()
	visible := make([]provider.FeatureQueryable, 0, len(fields))
	for _, field := range fields {
		if cql2.CanReferenceProperty(field.Name) {
			visible = append(visible, field)
		}
	}
	detached, err := provider.NewFeatureQueryables(visible)
	return detached, err == nil, err
}
