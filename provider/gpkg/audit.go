//go:build cgo

package gpkg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alexeydott/tegola/feature/audit"
	"github.com/alexeydott/tegola/provider"
	pa "github.com/alexeydott/tegola/provider/audit"
)

// ensureAuditTables creates tegola_audit and tegola_outbox if not exist.
// Called at transaction begin (idempotent).
func ensureAuditTables(ctx context.Context, tx *sql.Tx) error {
	var hasVersion int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tegola_schema_version'`).Scan(&hasVersion); err != nil {
		return err
	}
	if hasVersion > 0 {
		var version sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT MAX(version) FROM tegola_schema_version`).Scan(&version); err != nil {
			return err
		}
		if version.Valid && (version.Int64 < 1 || version.Int64 > pa.CurrentSchemaVersion) {
			return fmt.Errorf("unsupported service schema version %d", version.Int64)
		}
	}
	stmts := []string{pa.RevisionSQLiteDDL,
		`CREATE TABLE IF NOT EXISTS tegola_audit (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts TEXT NOT NULL,
			actor TEXT NOT NULL DEFAULT '',
			collection TEXT NOT NULL,
			operation TEXT NOT NULL,
			feature_id INTEGER NOT NULL,
			revision_before TEXT NOT NULL DEFAULT '',
			revision_after TEXT NOT NULL DEFAULT '',
			transaction_id TEXT NOT NULL DEFAULT '',
			request_id TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS tegola_outbox (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts TEXT NOT NULL,
			event_type TEXT NOT NULL,
			collection TEXT NOT NULL,
			feature_id INTEGER NOT NULL,
			payload TEXT NOT NULL DEFAULT '{}',
			dispatched INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tegola_outbox_undispatched ON tegola_outbox(dispatched) WHERE dispatched = 0`,
	}
	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("audit table setup: %w", err)
		}
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('tegola_revisions') WHERE name='incarnation'`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE tegola_revisions ADD COLUMN incarnation INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	return nil
}

// recordAuditTx writes an audit entry and outbox event in the given transaction.
// Must be called before Commit; failures abort the transaction.
func recordAuditTx(ctx context.Context, tx *sql.Tx, entry audit.Entry, eventType string) error {
	ts := entry.Timestamp.UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO tegola_audit (ts, actor, collection, operation, feature_id, revision_before, revision_after, transaction_id, request_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ts, entry.Actor, entry.Collection, entry.Operation, entry.FeatureID,
		entry.RevisionBefore, entry.RevisionAfter, entry.TransactionID, entry.RequestID,
	); err != nil {
		return fmt.Errorf("audit insert: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO tegola_outbox (ts, event_type, collection, feature_id, payload, dispatched)
		 VALUES (?, ?, ?, ?, ?, 0)`,
		ts, eventType, entry.Collection, entry.FeatureID, "{}",
	); err != nil {
		return fmt.Errorf("outbox insert: %w", err)
	}
	return nil
}

// auditEntryFor builds an audit.Entry from a mutation outcome.
func auditEntryFor(collection string, op provider.MutationOp, outcome provider.MutationOutcome, actor, requestID, txID string) audit.Entry {
	opStr := op.String()
	var revBefore, revAfter string
	switch op {
	case provider.MutationInsert:
		revAfter = outcome.Revision
	case provider.MutationDelete:
		revBefore = outcome.RevisionBefore
	default:
		revBefore = outcome.RevisionBefore
		revAfter = outcome.Revision
	}
	return audit.Entry{
		Timestamp:      time.Now(),
		Actor:          actor,
		Collection:     collection,
		Operation:      opStr,
		FeatureID:      outcome.FeatureID,
		RevisionBefore: revBefore,
		RevisionAfter:  revAfter,
		TransactionID:  txID,
		RequestID:      requestID,
	}
}

// outboxEventType maps a mutation op to an outbox event type.
func outboxEventType(op provider.MutationOp) string {
	switch op {
	case provider.MutationInsert:
		return audit.EventCreated
	case provider.MutationReplace:
		return audit.EventReplaced
	case provider.MutationUpdate:
		return audit.EventUpdated
	case provider.MutationDelete:
		return audit.EventDeleted
	default:
		return "feature.changed"
	}
}
