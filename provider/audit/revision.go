// Package audit: revision tracking for A03 optimistic concurrency.
//
// A03 requires the If-Match revision check to happen INSIDE the native
// data transaction, not as a separate HTTP-layer read before it. Without
// this, two clients can both pass the precondition check and overwrite
// each other (TOCTOU race).
//
// Design: a tegola_revisions sidecar table maps (collection, feature_id)
// to a monotonic revision counter. It is updated in the SAME transaction
// as the data change:
//
//  1. checkRevision: SELECT the current revision (locked by the tx).
//     If IfRevision != "" and != current -> PreconditionFailed (412).
//  2. After the data mutation succeeds: bump the revision.
//
// The table must be created by migration before write traffic (A01: no
// DDL inside the data transaction). If the table is missing, revision
// checks fail with a clear error instead of silently passing.
//
// Revision 0 means "no revision recorded yet" (row never written through
// this path). The first guarded write initializes it.
package audit

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/alexeydott/tegola/provider"
)

// RevisionDDL statements for migration (NOT executed in data transactions).
const (
	RevisionSQLiteDDL = `CREATE TABLE IF NOT EXISTS tegola_revisions (
	collection TEXT NOT NULL,
	feature_id INTEGER NOT NULL,
	revision INTEGER NOT NULL DEFAULT 0,
	incarnation INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (collection, feature_id)
);`
	RevisionMySQLDDL = `CREATE TABLE IF NOT EXISTS tegola_revisions (
	collection VARCHAR(255) NOT NULL,
	feature_id BIGINT NOT NULL,
	revision BIGINT NOT NULL DEFAULT 0,
	incarnation BIGINT NOT NULL DEFAULT 0,
	PRIMARY KEY (collection, feature_id)
);`
	RevisionPostgresDDL = `CREATE TABLE IF NOT EXISTS tegola_revisions (
	collection TEXT NOT NULL,
	feature_id BIGINT NOT NULL,
	revision BIGINT NOT NULL DEFAULT 0,
	incarnation BIGINT NOT NULL DEFAULT 0,
	PRIMARY KEY (collection, feature_id)
);`
)

// CheckAndBumpRevisionSQL implements the A03 check inside a *sql.Tx
// (SQLite/MySQL). It SELECTs the current revision, compares against
// want (the IfRevision precondition; "" skips the check), and bumps.
//
// Returns the new revision, or a *provider.MutationError.
// RevisionBump carries old and new revisions (R12).
type RevisionBump struct {
	Old int64
	New int64
	// A38: incarnation (entity generation).
	OldInc int64
	NewInc int64
}

func CheckAndBumpRevisionSQL(ctx context.Context, tx *sql.Tx, collection string, featureID uint64, want string, dialect string) (RevisionBump, error) {
	var cur, curInc int64
	var found bool
	// Lock the revision row within this transaction.
	// MySQL: SELECT ... FOR UPDATE. SQLite: the tx itself serializes.
	q := `SELECT revision, incarnation FROM tegola_revisions WHERE collection = ? AND feature_id = ?`
	if dialect == "mysql" {
		q += ` FOR UPDATE`
	}
	err := tx.QueryRowContext(ctx, q, collection, featureID).Scan(&cur, &curInc)
	switch {
	case err == nil:
		found = true
	case err == sql.ErrNoRows:
		found = false
	default:
		// Table missing: revision tracking not migrated. If no precondition
		// was requested, skip silently (backward compat). If IfRevision was
		// given, we cannot enforce it -> fail explicitly (A03).
		if IsMissingTable(err) {
			if want != "" {
				return RevisionBump{}, &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: "revision precondition requires tegola_revisions table (run migration)"}
			}
			// R01: table not migrated -> no revision tracking. Return -1
			// so callers emit "" (hash ETag fallback), not "0".
			return RevisionBump{Old: -1, New: -1}, nil
		}
		return RevisionBump{}, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("revision read: %v", err)}
	}
	if want != "" {
		wantInc, wantRev, err := ParseRevision(want)
		if err != nil {
			return RevisionBump{}, err
		}

		var curRevNum, curIncNum int64
		if found {
			curRevNum = cur
			curIncNum = curInc
		}
		if wantRev != curRevNum || wantInc != curIncNum {
			return RevisionBump{}, &provider.MutationError{Kind: provider.MutationErrPreconditionFailed, Reason: fmt.Sprintf("revision mismatch: have %d.%d, want %d.%d", curIncNum, curRevNum, wantInc, wantRev)}
		}
	}
	newRev := cur + 1
	if !found {
		newRev = 1
	}
	// A38: preserve incarnation on bump (only DELETE increments it).
	newInc := curInc
	if !found {
		newInc = 0
	}
	if dialect == "mysql" {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO tegola_revisions (collection, feature_id, revision, incarnation) VALUES (?, ?, ?, ?)
			 ON DUPLICATE KEY UPDATE revision = ?, incarnation = ?`, collection, featureID, newRev, newInc, newRev, newInc)
	} else {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO tegola_revisions (collection, feature_id, revision, incarnation) VALUES (?, ?, ?, ?)
			 ON CONFLICT(collection, feature_id) DO UPDATE SET revision = excluded.revision, incarnation = excluded.incarnation`,
			collection, featureID, newRev, newInc)
	}
	if err != nil {
		if IsMissingTable(err) {
			// No revision table: skip bump.
			return RevisionBump{Old: -1, New: -1}, nil
		}
		return RevisionBump{}, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("revision bump: %v", err)}
	}
	oldRev := cur
	oldInc := curInc
	if !found {
		oldRev = 0
		oldInc = 0
	}
	// newInc was set earlier (preserved from curInc, or 0 if not found).
	return RevisionBump{Old: oldRev, New: newRev, OldInc: oldInc, NewInc: newInc}, nil
}

// BumpIncarnationOnDelete increments the entity incarnation on DELETE.
// A38: this is the tombstone — a subsequent INSERT (recreate) will see
// the incremented incarnation and know it's a new entity generation.
// The revision row is kept (not deleted) to preserve the incarnation.
func BumpIncarnationOnDelete(ctx context.Context, tx *sql.Tx, collection string, featureID uint64, dialect string) error {
	var curInc int64
	q := `SELECT incarnation FROM tegola_revisions WHERE collection = ? AND feature_id = ?`
	if dialect == "mysql" {
		q += ` FOR UPDATE`
	}
	err := tx.QueryRowContext(ctx, q, collection, featureID).Scan(&curInc)
	if err == sql.ErrNoRows {
		// No revision row: create one with incarnation=1 (deleted once).
		if dialect == "mysql" {
			_, err = tx.ExecContext(ctx,
				`INSERT INTO tegola_revisions (collection, feature_id, revision, incarnation) VALUES (?, ?, 0, 1)
				 ON DUPLICATE KEY UPDATE incarnation = incarnation + 1`, collection, featureID)
		} else {
			_, err = tx.ExecContext(ctx,
				`INSERT INTO tegola_revisions (collection, feature_id, revision, incarnation) VALUES (?, ?, 0, 1)
				 ON CONFLICT(collection, feature_id) DO UPDATE SET incarnation = tegola_revisions.incarnation + 1`,
				collection, featureID)
		}
		return err
	}
	if err != nil {
		if IsMissingTable(err) {
			return nil // no revision tracking; skip
		}
		return err
	}
	// Increment incarnation.
	if dialect == "mysql" {
		_, err = tx.ExecContext(ctx,
			`UPDATE tegola_revisions SET incarnation = incarnation + 1, revision = 0 WHERE collection = ? AND feature_id = ?`,
			collection, featureID)
	} else {
		_, err = tx.ExecContext(ctx,
			`UPDATE tegola_revisions SET incarnation = incarnation + 1, revision = 0 WHERE collection = ? AND feature_id = ?`,
			collection, featureID)
	}
	return err
}

// isMissingTable reports whether err is a "no such table" error.
// IsMissingTable reports whether err indicates a missing revisions table.
func IsMissingTable(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "doesn't exist") ||
		strings.Contains(msg, "does not exist")
}

// ParseRevision parses the complete token, rejecting trailing junk and negatives.
func ParseRevision(value string) (incarnation, revision int64, err error) {
	parts := strings.Split(value, ".")
	if len(parts) == 1 {
		parts = append([]string{"0"}, parts...)
	}
	if len(parts) != 2 {
		return 0, 0, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "invalid revision"}
	}
	incarnation, err = strconv.ParseInt(parts[0], 10, 64)
	if err == nil {
		revision, err = strconv.ParseInt(parts[1], 10, 64)
	}
	if err != nil || incarnation < 0 || revision < 0 {
		return 0, 0, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "invalid revision"}
	}
	return incarnation, revision, nil
}
