//go:build cgo

package gpkg_test

import (
	"context"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

// A38: entity incarnation lifecycle.
// Delete increments incarnation (tombstone); recreate gets the new incarnation.
// Stale If-Match (old incarnation) is rejected.
func TestIncarnationLifecycle(t *testing.T) {
	path, w := newMutationFixture(t)
	ctx := context.Background()
	if err := ensureRevisionsTable(t, path); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 1. Insert → incarnation 0.
	tx, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	out, err := tx.Apply(ctx, provider.Mutation{
		Op:         provider.MutationInsert,
		Collection: "parcels",
		Properties: map[string]provider.MutationValue{"name": strVal("v1"), "lots": intVal(1)},
	})
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("insert: %v", err)
	}
	fid := out.FeatureID
	rev1 := out.Revision // "0.1"
	t.Logf("insert: revision=%q", rev1)
	if _, err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if rev1 != "0.1" {
		t.Fatalf("want revision 0.1, got %q", rev1)
	}

	// 2. Delete → incarnation increments to 1.
	tx2, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := tx2.Apply(ctx, provider.Mutation{
		Op:         provider.MutationDelete,
		Collection: "parcels",
		FeatureID:  fid,
		IfRevision: rev1,
	}); err != nil {
		_ = tx2.Rollback(ctx)
		t.Fatalf("delete: %v", err)
	}
	if _, err := tx2.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// 3. Re-insert same ID → new generation, incarnation 1.
	tx3, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	out3, err := tx3.Apply(ctx, provider.Mutation{
		Op:         provider.MutationInsert,
		Collection: "parcels",
		Properties: map[string]provider.MutationValue{"name": strVal("v2"), "lots": intVal(2)},
	})
	if err != nil {
		_ = tx3.Rollback(ctx)
		t.Fatalf("re-insert: %v", err)
	}
	rev3 := out3.Revision
	t.Logf("re-insert: revision=%q", rev3)
	if _, err := tx3.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	// Incarnation should be 1 (not 0).
	if rev3 != "1.1" {
		t.Fatalf("want revision 1.1 (incarnation 1), got %q", rev3)
	}
	current, err := w.(provider.RevisionReader).CurrentRevision(ctx, "parcels", out3.FeatureID)
	if err != nil || current != rev3 {
		t.Fatalf("read revision must match mutation revision: got %q, %v; want %q", current, err, rev3)
	}

	// 4. Stale If-Match (old incarnation 0.1) → 412.
	tx4, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	_, err = tx4.Apply(ctx, provider.Mutation{
		Op:         provider.MutationUpdate,
		Collection: "parcels",
		FeatureID:  fid,
		Properties: map[string]provider.MutationValue{"name": strVal("v3")},
		IfRevision: rev1, // stale: 0.1, current is 1.1
	})
	_ = tx4.Rollback(ctx)
	if err == nil {
		t.Fatal("expected 412 for stale incarnation")
	}
	if me, ok := provider.AsMutationError(err); ok {
		if me.Kind != provider.MutationErrPreconditionFailed {
			t.Fatalf("want PreconditionFailed, got %v", me.Kind)
		}
	} else {
		t.Fatalf("want MutationError, got %T", err)
	}

	// 5. Fresh If-Match (1.1) → succeeds.
	tx5, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	out5, err := tx5.Apply(ctx, provider.Mutation{
		Op:         provider.MutationUpdate,
		Collection: "parcels",
		FeatureID:  fid,
		Properties: map[string]provider.MutationValue{"name": strVal("v3")},
		IfRevision: rev3,
	})
	if err != nil {
		_ = tx5.Rollback(ctx)
		t.Fatalf("update: %v", err)
	}
	if _, err := tx5.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if out5.Revision != "1.2" {
		t.Fatalf("want 1.2, got %q", out5.Revision)
	}
}
