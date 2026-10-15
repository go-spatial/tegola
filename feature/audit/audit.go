// Package audit implements W13: atomic audit log and transactional outbox.
//
// Every successful mutation records an audit entry and an outbox event in
// the SAME database transaction as the data change. The outbox dispatcher
// publishes events after commit for cache invalidation and downstream
// consumers.
package audit

import (
	"time"
)

// Entry is one audit log record.
type Entry struct {
	// ID is the audit record ID (assigned by storage).
	ID int64
	// Timestamp is when the mutation was applied.
	Timestamp time.Time
	// Actor identifies who made the change.
	Actor string
	// Collection is the logical collection name.
	Collection string
	// Operation is insert, replace, update, or delete.
	Operation string
	// FeatureID is the affected feature's ID.
	FeatureID uint64
	// RevisionBefore is the revision before the change (empty for insert).
	RevisionBefore string
	// RevisionAfter is the revision after the change (empty for delete).
	RevisionAfter string
	// TransactionID correlates with the provider's native transaction.
	TransactionID string
	// RequestID correlates with the HTTP request/transaction.
	RequestID string
}

// Event is one outbox event for downstream dispatch.
type Event struct {
	// ID is the outbox record ID (assigned by storage).
	ID int64
	// Timestamp is when the event was created.
	Timestamp time.Time
	// Type is the event type (e.g. "feature.created", "feature.updated").
	Type string
	// Collection is the logical collection name.
	Collection string
	// FeatureID is the affected feature's ID.
	FeatureID uint64
	// Payload carries event-specific data (bounds, revision, etc).
	// Secrets and raw HTTP bodies are never stored here.
	Payload map[string]string
	// Dispatched marks whether the event was published.
	Dispatched bool
}

// EventType constants.
const (
	EventCreated  = "feature.created"
	EventReplaced = "feature.replaced"
	EventUpdated  = "feature.updated"
	EventDeleted  = "feature.deleted"
)

// Store persists audit entries and outbox events.
type Store interface {
	// RecordAudit inserts an audit entry. Called in the data transaction.
	RecordAudit(entry Entry) error
	// EnqueueOutbox inserts an outbox event. Called in the data transaction.
	EnqueueOutbox(event Event) error
	// ClaimOutbox returns undispatched events for publishing.
	ClaimOutbox(limit int) ([]Event, error)
	// MarkDispatched marks events as published.
	MarkDispatched(ids []int64) error
}
