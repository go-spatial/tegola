package postgis

import (
	"context"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

func TestInsertExplicitNullGeometryOverridesDefault(t *testing.T) {
	w, pool := followupPostGIS(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "ALTER TABLE items ALTER COLUMN geom SET DEFAULT decode('010100000000000000000000000000000000000000','hex')"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.DescribeWritable(ctx, "items"); err != nil {
		t.Fatal(err)
	}
	for _, explicitNull := range []bool{false, true} {
		tx, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
		if err != nil {
			t.Fatal(err)
		}
		m := provider.Mutation{Op: provider.MutationInsert, Collection: "items", GeometryAbsent: explicitNull}
		if !explicitNull {
			m.Properties = map[string]provider.MutationValue{"name": {Kind: provider.MutationValueString, String: "default"}}
		}
		out, err := tx.Apply(ctx, m)
		if err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if _, err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		var isNull bool
		if err := pool.QueryRow(ctx, "SELECT geom IS NULL FROM items WHERE id=$1", out.FeatureID).Scan(&isNull); err != nil {
			t.Fatal(err)
		}
		if isNull != explicitNull {
			t.Fatalf("explicitNull=%v stored NULL=%v", explicitNull, isNull)
		}
	}
}
