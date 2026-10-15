//go:build cgo

package gpkg_test

import (
	"context"
	"testing"

	"github.com/alexeydott/tegola/provider"
	pa "github.com/alexeydott/tegola/provider/audit"
)

func TestInsertExplicitNullGeometryOverridesDefault(t *testing.T) {
	p, db := mosBoundsFixture(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB DEFAULT X'010100000000000000000000000000000000000000',name TEXT)", map[string]any{"geometry_format": "wkb"})
	ctx := context.Background()
	if err := pa.Migrate(ctx, db, "sqlite"); err != nil {
		t.Fatal(err)
	}
	for _, explicitNull := range []bool{false, true} {
		tx, err := p.BeginFeatureTx(ctx, provider.TxOptions{})
		if err != nil {
			t.Fatal(err)
		}
		m := provider.Mutation{Op: provider.MutationInsert, Collection: "items", GeometryAbsent: explicitNull}
		if !explicitNull {
			m.Properties = map[string]provider.MutationValue{"name": strVal("default")}
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
		if err := db.QueryRow("SELECT geom IS NULL FROM items WHERE id=?", out.FeatureID).Scan(&isNull); err != nil {
			t.Fatal(err)
		}
		if isNull != explicitNull {
			t.Fatalf("explicitNull=%v stored NULL=%v", explicitNull, isNull)
		}
	}
}
