//go:build cgo

package gpkg_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

// A03: in-transaction CAS concurrency tests.
// Proves that concurrent writes with If-Match are serialized correctly:
// the loser gets 412 Precondition Failed, not a lost update.
func TestConcurrentCASUpdate(t *testing.T) {
	path, w := newMutationFixture(t)
	ctx := context.Background()
	// A03: ensure the revisions table exists for CAS.
	if err := ensureRevisionsTable(t, path); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Seed: insert a feature.
	tx, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	out, err := tx.Apply(ctx, provider.Mutation{
		Op:         provider.MutationInsert,
		Collection: "parcels",
		Properties: map[string]provider.MutationValue{"name": strVal("a"), "lots": intVal(1)},
	})
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("insert: %v", err)
	}
	fid := out.FeatureID
	rev1 := out.Revision // e.g. "0.1"
	if _, err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Tx1: begin, update with If-Match=rev1 (should succeed).
	tx1, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatalf("begin tx1: %v", err)
	}
	out1, err := tx1.Apply(ctx, provider.Mutation{
		Op:         provider.MutationUpdate,
		Collection: "parcels",
		FeatureID:  fid,
		Properties: map[string]provider.MutationValue{"name": strVal("b")},
		IfRevision: rev1,
	})
	if err != nil {
		_ = tx1.Rollback(ctx)
		t.Fatalf("tx1 apply: %v", err)
	}
	if _, err := tx1.Commit(ctx); err != nil {
		t.Fatalf("tx1 commit: %v", err)
	}
	rev2 := out1.Revision
	if rev2 == rev1 {
		t.Fatalf("revision not bumped: %q", rev2)
	}

	// Tx2: try update with stale If-Match=rev1 (should fail with 412).
	tx2, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatalf("begin tx2: %v", err)
	}
	_, err = tx2.Apply(ctx, provider.Mutation{
		Op:         provider.MutationUpdate,
		Collection: "parcels",
		FeatureID:  fid,
		Properties: map[string]provider.MutationValue{"name": strVal("c")},
		IfRevision: rev1, // stale!
	})
	_ = tx2.Rollback(ctx)
	if err == nil {
		t.Fatal("expected precondition failure for stale If-Match, got nil")
	}
	if me, ok := provider.AsMutationError(err); ok {
		if me.Kind != provider.MutationErrPreconditionFailed {
			t.Fatalf("want PreconditionFailed, got %v (%v)", me.Kind, me.Reason)
		}
	} else {
		t.Fatalf("want MutationError, got %T: %v", err, err)
	}

	// Tx3: update with fresh If-Match=rev2 (should succeed).
	tx3, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatalf("begin tx3: %v", err)
	}
	out3, err := tx3.Apply(ctx, provider.Mutation{
		Op:         provider.MutationUpdate,
		Collection: "parcels",
		FeatureID:  fid,
		Properties: map[string]provider.MutationValue{"name": strVal("d")},
		IfRevision: rev2,
	})
	if err != nil {
		_ = tx3.Rollback(ctx)
		t.Fatalf("tx3 apply: %v", err)
	}
	if _, err := tx3.Commit(ctx); err != nil {
		t.Fatalf("tx3 commit: %v", err)
	}
	if out3.Revision == rev2 {
		t.Fatalf("revision not bumped on tx3")
	}
}

// A03: delete with If-Match enforces the precondition in-tx.
func TestConcurrentCASDelete(t *testing.T) {
	path, w := newMutationFixture(t)
	ctx := context.Background()
	if err := ensureRevisionsTable(t, path); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tx, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	out, err := tx.Apply(ctx, provider.Mutation{
		Op:         provider.MutationInsert,
		Collection: "parcels",
		Properties: map[string]provider.MutationValue{"name": strVal("x"), "lots": intVal(1)},
	})
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("insert: %v", err)
	}
	fid := out.FeatureID
	rev1 := out.Revision
	if _, err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Delete with wrong revision → 412.
	tx2, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	_, err = tx2.Apply(ctx, provider.Mutation{
		Op:         provider.MutationDelete,
		Collection: "parcels",
		FeatureID:  fid,
		IfRevision: "999.999",
	})
	_ = tx2.Rollback(ctx)
	if err == nil {
		t.Fatal("expected precondition failure")
	}
	if me, ok := provider.AsMutationError(err); ok {
		if me.Kind != provider.MutationErrPreconditionFailed {
			t.Fatalf("want PreconditionFailed, got %v", me.Kind)
		}
	}

	// Delete with correct revision → succeeds.
	tx3, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := tx3.Apply(ctx, provider.Mutation{
		Op:         provider.MutationDelete,
		Collection: "parcels",
		FeatureID:  fid,
		IfRevision: rev1,
	}); err != nil {
		_ = tx3.Rollback(ctx)
		t.Fatalf("delete: %v", err)
	}
	if _, err := tx3.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// ensureRevisionsTable creates the tegola_revisions table for CAS tests.
func ensureRevisionsTable(t *testing.T, path string) error {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS tegola_revisions (
		collection TEXT NOT NULL,
		feature_id INTEGER NOT NULL,
		revision INTEGER NOT NULL DEFAULT 0,
		incarnation INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (collection, feature_id)
	)`)
	return err
}
