//go:build cgo

package gpkg_test

import (
	"context"
	"database/sql"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/gpkg"
	"testing"
)

func TestReviewGPKGPublicFieldsAndAliasCAS(t *testing.T) {
	path, w := newMutationFixture(t)
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	}()
	if _, err = db.Exec("ALTER TABLE parcels ADD COLUMN secret TEXT DEFAULT 'private'"); err != nil {
		t.Fatal(err)
	}
	p, err := gpkg.NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]interface{}{{"name": "alias", "tablename": "parcels", "id_fieldname": "fid", "geometry_fieldname": "geom", "geometry_format": "wkb", "srid": 4326, "fields": []string{"name", "lots", "price"}}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	alias := p.(*gpkg.Provider).MutationWriter()
	ctx := context.Background()
	apply := func(writer provider.MutationProvider, m provider.Mutation) (provider.MutationOutcome, error) {
		tx, err := writer.BeginFeatureTx(ctx, provider.TxOptions{})
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
	initial, err := w.(provider.RevisionReader).CurrentRevision(ctx, "parcels", 1)
	if err != nil || initial != "0.0" {
		t.Fatalf("initial revision %q %v", initial, err)
	}
	first, err := apply(w, provider.Mutation{Op: provider.MutationInsert, Collection: "parcels", Properties: map[string]provider.MutationValue{"name": strVal("original"), "lots": intVal(1)}})
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != "0.1" {
		t.Fatalf("missing automatic tracking: %+v", first)
	}
	next, err := apply(alias, provider.Mutation{Op: provider.MutationReplace, Collection: "alias", FeatureID: first.FeatureID, IfRevision: first.Revision, Properties: map[string]provider.MutationValue{"name": strVal("replaced"), "lots": intVal(2)}})
	if err != nil {
		t.Fatal(err)
	}
	if next.Revision != "0.2" {
		t.Fatalf("alias revision %+v", next)
	}
	var secret string
	if err = db.QueryRow("SELECT secret FROM parcels WHERE fid=?", first.FeatureID).Scan(&secret); err != nil || secret != "private" {
		t.Fatalf("replace erased private field: %q %v", secret, err)
	}
	_, err = apply(w, provider.Mutation{Op: provider.MutationUpdate, Collection: "parcels", FeatureID: first.FeatureID, IfRevision: first.Revision, Properties: map[string]provider.MutationValue{"name": strVal("stale")}})
	if me, ok := provider.AsMutationError(err); !ok || me.Kind != provider.MutationErrPreconditionFailed {
		t.Fatalf("alias stale write accepted: %v", err)
	}
	_, err = apply(w, provider.Mutation{Op: provider.MutationUpdate, Collection: "parcels", FeatureID: first.FeatureID, Properties: map[string]provider.MutationValue{"secret": strVal("exposed")}})
	if err == nil {
		t.Fatal("private property writable")
	}
	sd, err := w.(provider.SchemaProvider).DescribeSchema(ctx, "parcels")
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range sd.Columns {
		if column.Name == "secret" {
			t.Fatal("private property exposed in schema")
		}
	}
}
