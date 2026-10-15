package observability

import "time"

type FeatureResource uint8

const (
	FeatureResourceUnknown FeatureResource = iota
	FeatureResourceLanding
	FeatureResourceAPI
	FeatureResourceConformance
	FeatureResourceCollections
	FeatureResourceCollection
	FeatureResourceQueryables
	FeatureResourceItems
	FeatureResourceItem
)

type FeatureMethod uint8

const (
	FeatureMethodOther FeatureMethod = iota
	FeatureMethodGET
	FeatureMethodHEAD
	FeatureMethodOPTIONS
)

type FeatureRequestObservation struct {
	Resource FeatureResource
	Method   FeatureMethod
	Status   int
	Duration time.Duration
}

// FeatureRequestObserver is optional and receives completed outer HTTP outcomes.
// Implementations must be concurrency-safe and must not retain request values.
type FeatureRequestObserver interface {
	ObserveFeatureRequest(FeatureRequestObservation)
}
