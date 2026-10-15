//go:build cgo

package gpkg_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

func TestGPKGPrecommitFailureNotCommitted(t *testing.T) {
	for _, mode := range []string{"metadata", "apply"} {
		t.Run(mode, func(t *testing.T) {
			path, w := newMutationFixture(t)
			db, err := sql.Open("sqlite3", path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = db.Close() }()
			ctx := context.Background()
			// Commit the service tables first so the rollback assertion can inspect them.
			tx, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if mode == "metadata" {
				for _, q := range []string{
					`CREATE TABLE gpkg_contents(table_name TEXT,last_change TEXT,min_x REAL,max_x REAL,min_y REAL,max_y REAL)`,
					`INSERT INTO gpkg_contents(table_name) VALUES('parcels')`,
					`CREATE TRIGGER reject_metadata BEFORE UPDATE ON gpkg_contents BEGIN SELECT RAISE(ABORT,'metadata denied'); END`,
				} {
					if _, err = db.Exec(q); err != nil {
						t.Fatal(err)
					}
				}
			}
			tx, err = w.BeginFeatureTx(ctx, provider.TxOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if _, err = tx.Apply(ctx, provider.Mutation{Op: provider.MutationInsert, Collection: "parcels", Properties: map[string]provider.MutationValue{"name": strVal("temporary"), "lots": intVal(1)}}); err != nil {
				t.Fatal(err)
			}
			if mode == "apply" {
				if _, err = tx.Apply(ctx, provider.Mutation{Op: provider.MutationInsert, Collection: "parcels", Properties: map[string]provider.MutationValue{"hidden": strVal("invalid")}}); err == nil {
					t.Fatal("expected invalid property")
				}
			}
			receipt, err := tx.Commit(ctx)
			if err == nil || receipt.Status != provider.CommitNotCommitted {
				t.Errorf("receipt=%+v err=%v", receipt, err)
			}
			if me, ok := provider.AsMutationError(err); ok && me.Kind == provider.MutationErrCommitUnknown {
				t.Errorf("precommit failure classified unknown: %v", err)
			}
			for _, table := range []string{"parcels", "tegola_audit", "tegola_outbox", "tegola_revisions"} {
				var n int
				if err = db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
					t.Fatal(err)
				}
				if n != 0 {
					t.Errorf("%s persisted %d rows", table, n)
				}
			}
		})
	}
}
