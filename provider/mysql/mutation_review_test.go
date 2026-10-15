package mysql

import (
	"context"
	"database/sql"
	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/tegola/provider"
	pa "github.com/alexeydott/tegola/provider/audit"
	driver "github.com/go-sql-driver/mysql"
	"os"
	"testing"
	"time"
)

func TestReviewMySQLNativeMutation(t *testing.T) {
	dsn := os.Getenv("TEGOLA_REVIEW_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEGOLA_REVIEW_MYSQL_DSN not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t.Run("migration", func(t *testing.T) {
		if err := pa.Migrate(ctx, db, "mysql"); err != nil {
			t.Fatal(err)
		}
	})
	// Establish the schema independently so lifecycle failures remain visible even if migration fails.
	for _, stmt := range splitStmts(pa.MySQLDDL) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.ExecContext(ctx, "INSERT IGNORE INTO tegola_schema_version VALUES (2,'test')"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, "DROP TABLE IF EXISTS review_features"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, "CREATE TABLE review_features (id BIGINT PRIMARY KEY AUTO_INCREMENT, geom GEOMETRY, name VARCHAR(100), extra VARCHAR(100), secret VARCHAR(100) DEFAULT 'private') ENGINE=InnoDB"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, "DELETE FROM tegola_revisions WHERE collection LIKE '%review_features%'"); err != nil {
		t.Fatal(err)
	}
	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	p := &Provider{db: db, Database: cfg.DBName, layers: map[string]Layer{"review_features": {tagFieldnames: []string{"name", "extra"}, name: "review_features", tablename: "review_features", idFieldname: "id", geomFieldname: "geom", geomType: geom.Point{}, srid: 4326, geometryFormat: "mysql", serverFlavor: "mysql"}}}
	p.layers["review_alias"] = p.layers["review_features"]
	w := p.writer()
	if _, err = w.DescribeWritable(ctx, "review_features"); err != nil {
		t.Fatal(err)
	}
	point, _ := wkb.EncodeBytes(geom.Point{120, 35})
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
	out, err := apply(provider.Mutation{Op: provider.MutationInsert, Collection: "review_features", Properties: map[string]provider.MutationValue{"name": val("one"), "extra": val("old")}, GeometryWKB: point, GeometrySRID: 4326})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("axis", func(t *testing.T) {
		var lon, lat float64
		if err := db.QueryRowContext(ctx, "SELECT ST_Longitude(geom),ST_Latitude(geom) FROM review_features WHERE id=?", out.FeatureID).Scan(&lon, &lat); err != nil {
			t.Fatal(err)
		}
		if lon != 120 || lat != 35 {
			t.Fatalf("coordinate corruption: %v,%v", lon, lat)
		}
	})
	t.Run("revision", func(t *testing.T) {
		rev, err := w.CurrentRevision(ctx, "review_features", out.FeatureID)
		if err != nil || rev != out.Revision {
			t.Fatalf("read=%q write=%q err=%v", rev, out.Revision, err)
		}
	})
	t.Run("identical replace", func(t *testing.T) {
		next, err := apply(provider.Mutation{Op: provider.MutationReplace, Collection: "review_features", FeatureID: out.FeatureID, IfRevision: out.Revision, Properties: map[string]provider.MutationValue{"name": val("one"), "extra": val("old")}})
		if err != nil {
			t.Fatal(err)
		}
		if next.Revision != "0.2" {
			t.Fatalf("replacement changed incarnation: %+v", next)
		}
		out = next
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
		if _, err := db.ExecContext(ctx, "INSERT INTO tegola_schema_version VALUES(3,'2026-10-04')"); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if _, err := db.ExecContext(ctx, "DELETE FROM tegola_schema_version WHERE version=3"); err != nil {
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
