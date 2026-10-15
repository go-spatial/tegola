package postgis

import (
	"context"
	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
	pa "github.com/alexeydott/tegola/provider/audit"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

func TestReviewPostGISNativeMutation(t *testing.T) {
	dsn := os.Getenv("TEGOLA_REVIEW_POSTGIS_DSN")
	if dsn == "" {
		t.Skip("TEGOLA_REVIEW_POSTGIS_DSN not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, stmt := range splitStmts(pa.PostgresDDL) {
		if _, err = pool.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	for _, stmt := range []string{"INSERT INTO tegola_schema_version VALUES (2,NOW()) ON CONFLICT DO NOTHING", "DROP TABLE IF EXISTS review_features", "CREATE TABLE review_features(id BIGSERIAL PRIMARY KEY, geom geometry(Point,4326), name TEXT, extra TEXT, secret TEXT DEFAULT 'private')", "DELETE FROM tegola_revisions WHERE collection LIKE '%review_features%'"} {
		if _, err = pool.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	p := &Provider{config: *pool.Config(), pool: &connectionPoolCollector{Pool: pool}, layers: map[string]Layer{"review_features": {feature: &featureProfile{projections: []featureProjection{{output: "name", column: featureColumn{name: "name"}}, {output: "extra", column: featureColumn{name: "extra"}}}}, name: "review_features", tablename: "review_features", idField: "id", geomField: "geom", geomType: geom.Point{}, srid: 4326}}}
	p.layers["review_alias"] = p.layers["review_features"]
	w := p.writer()
	if _, err = w.DescribeWritable(ctx, "review_features"); err != nil {
		t.Fatal(err)
	}
	if _, err = w.DescribeWritable(ctx, "review_alias"); err != nil {
		t.Fatal(err)
	}
	apply := func(m provider.Mutation) (provider.MutationOutcome, error) {
		tx, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
		if err != nil {
			return provider.MutationOutcome{}, err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		out, err := tx.Apply(ctx, m)
		if err != nil {
			return out, err
		}
		_, err = tx.Commit(ctx)
		return out, err
	}
	val := func(s string) provider.MutationValue {
		return provider.MutationValue{Kind: provider.MutationValueString, String: s}
	}
	out, err := apply(provider.Mutation{Op: provider.MutationInsert, Collection: "review_features", Properties: map[string]provider.MutationValue{"name": val("one"), "extra": val("old")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("revision format", func(t *testing.T) {
		if out.Revision != "0.1" {
			t.Fatalf("got %q", out.Revision)
		}
	})
	t.Run("guarded replacement", func(t *testing.T) {
		next, err := apply(provider.Mutation{Op: provider.MutationReplace, Collection: "review_features", FeatureID: out.FeatureID, IfRevision: out.Revision, Properties: map[string]provider.MutationValue{"name": val("one"), "extra": val("old")}})
		if err != nil {
			t.Fatal(err)
		}
		if next.Revision != "0.2" {
			t.Fatalf("replace changes incarnation: %+v", next)
		}
		out = next
	})
	t.Run("guarded update", func(t *testing.T) {
		next, err := apply(provider.Mutation{Op: provider.MutationUpdate, Collection: "review_features", FeatureID: out.FeatureID, IfRevision: out.Revision, Properties: map[string]provider.MutationValue{"name": val("two")}})
		if err != nil {
			t.Fatal(err)
		}
		out = next
	})
	t.Run("read revision", func(t *testing.T) {
		rev, err := w.CurrentRevision(ctx, "review_features", out.FeatureID)
		if err != nil || rev != out.Revision {
			t.Fatalf("read=%q write=%q err=%v", rev, out.Revision, err)
		}
	})
	t.Run("hidden property", func(t *testing.T) {
		_, err := apply(provider.Mutation{Op: provider.MutationUpdate, Collection: "review_features", FeatureID: out.FeatureID, Properties: map[string]provider.MutationValue{"secret": val("leak")}})
		if err == nil {
			t.Fatal("hidden column is writable")
		}
		schema, err := w.DescribeSchema(ctx, "review_features")
		if err != nil {
			t.Fatal(err)
		}
		for _, column := range schema.Columns {
			if column.Name == "secret" {
				t.Fatal("hidden column exposed in schema")
			}
		}
	})
	t.Run("alias CAS", func(t *testing.T) {
		stale := out.Revision
		next, err := apply(provider.Mutation{Op: provider.MutationUpdate, Collection: "review_alias", FeatureID: out.FeatureID, IfRevision: stale, Properties: map[string]provider.MutationValue{"name": val("alias")}})
		if err != nil {
			t.Fatal(err)
		}
		_, err = apply(provider.Mutation{Op: provider.MutationUpdate, Collection: "review_features", FeatureID: out.FeatureID, IfRevision: stale, Properties: map[string]provider.MutationValue{"name": val("stale")}})
		if me, ok := provider.AsMutationError(err); !ok || me.Kind != provider.MutationErrPreconditionFailed {
			t.Fatalf("stale alias write accepted: %v", err)
		}
		out = next
	})
	t.Run("delete", func(t *testing.T) {
		next, err := apply(provider.Mutation{Op: provider.MutationDelete, Collection: "review_features", FeatureID: out.FeatureID, IfRevision: out.Revision})
		if err != nil {
			t.Fatal(err)
		}
		if next.RevisionBefore != out.Revision || next.Revision != "1.0" {
			t.Fatalf("bad tombstone: %+v", next)
		}
	})
	t.Run("future schema", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "INSERT INTO tegola_schema_version VALUES(3,'2026-10-04')"); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if _, err := pool.Exec(ctx, "DELETE FROM tegola_schema_version WHERE version=3"); err != nil {
				t.Errorf("remove future schema fixture: %v", err)
			}
		}()
		tx, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
		if err == nil {
			_ = tx.Rollback(ctx)
			t.Fatal("future schema accepted")
		}
	})

}
