package features

import (
	"context"
	"errors"
	"time"

	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/provider"
)

type QueryClass uint8

const (
	QueryClassUnknown QueryClass = iota
	QueryClassUnfiltered
	QueryClassID
	QueryClassBBox
	QueryClassDatetime
	QueryClassFilter
	QueryClassCRS
	QueryClassMixed
)

type QueryPushdown uint8

const (
	QueryPushdownUnknown QueryPushdown = iota
	QueryPushdownNone
	QueryPushdownSQLFilter
)

type QueryOutcome uint8

const (
	QueryOutcomeUnknown QueryOutcome = iota
	QueryOutcomeOK
	QueryOutcomeCanceled
	QueryOutcomeDeadline
	QueryOutcomeInvalid
	QueryOutcomeUnsupported
	QueryOutcomeSourceError
	QueryOutcomeCallbackError
	QueryOutcomeError
)

// QueryObservation contains only bounded labels and measurements, never source/query values.
// Duration covers the provider and its synchronous service/caller callbacks.
type QueryObservation struct {
	Backend      provider.FeatureQueryBackend
	Pushdown     QueryPushdown
	Class        QueryClass
	Outcome      QueryOutcome
	Duration     time.Duration
	RowsReturned uint64
}

// QueryObserver must be concurrency-safe. Calls are synchronous and do not retain contexts.
type QueryObserver interface{ ObserveFeatureQuery(QueryObservation) }

// WithQueryObserver creates a service sharing only immutable collection metadata.
func (s *Service) WithQueryObserver(observer QueryObserver) *Service {
	if s == nil {
		return nil
	}
	copy := *s
	copy.queryObserver = observer
	return &copy
}

func executionMetadata(querier provider.FeatureQuerier) provider.FeatureQueryExecutionMetadata {
	capability, ok := querier.(provider.FeatureQueryExecutionInfo)
	if !ok {
		return provider.FeatureQueryExecutionMetadata{}
	}
	metadata, err := capability.FeatureQueryExecutionInfo()
	if err != nil {
		return provider.FeatureQueryExecutionMetadata{}
	}
	if metadata.Backend > provider.FeatureQueryBackendHANA {
		metadata.Backend = provider.FeatureQueryBackendUnknown
	}
	if metadata.ScalarFilter > provider.FeatureFilterExecutionSQL {
		metadata.ScalarFilter = provider.FeatureFilterExecutionUnknown
	}
	return metadata
}

func queryClass(query provider.FeatureQuery, outputCRS bool) QueryClass {
	class, count := QueryClassUnfiltered, 0
	for _, predicate := range []struct {
		present bool
		class   QueryClass
	}{
		{len(query.IDs) != 0, QueryClassID},
		{len(query.Bounds) != 0 || len(query.Bounds3D) != 0, QueryClassBBox},
		{query.Temporal != nil, QueryClassDatetime},
		{query.Filter != nil, QueryClassFilter},
		{outputCRS, QueryClassCRS},
	} {
		if predicate.present {
			class = predicate.class
			count++
		}
	}
	if count > 1 {
		return QueryClassMixed
	}
	return class
}

func queryOutcome(err error, callbackError bool) QueryOutcome {
	if err == nil {
		return QueryOutcomeOK
	}
	if errors.Is(err, context.Canceled) {
		return QueryOutcomeCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return QueryOutcomeDeadline
	}
	var source provider.FeatureDataError
	if errors.As(err, &source) {
		return QueryOutcomeSourceError
	}
	if callbackError {
		return QueryOutcomeCallbackError
	}
	var invalid provider.InvalidFeatureQueryError
	if errors.As(err, &invalid) {
		return QueryOutcomeInvalid
	}
	if errors.Is(err, provider.ErrUnsupported) {
		return QueryOutcomeUnsupported
	}
	return QueryOutcomeError
}

func deliverQueryObservation(observer QueryObserver, observation QueryObservation) {
	defer func() {
		if recover() != nil {
			log.Error("feature query observer failed")
		}
	}()
	observer.ObserveFeatureQuery(observation)
}
