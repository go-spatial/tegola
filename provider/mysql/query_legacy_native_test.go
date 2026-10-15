package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alexeydott/tegola/provider"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestNativeMySQL55LegacySnapshot(t *testing.T) {
	dsn := os.Getenv("TEGOLA_REVIEW_MYSQL_ADMIN_DSN")
	if dsn == "" {
		t.Skip("TEGOLA_REVIEW_MYSQL_ADMIN_DSN not set")
	}
	cfg, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid native test DSN")
	}
	cfg.DBName = ""
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	var version string
	if err := admin.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "5.5.") || strings.Contains(strings.ToLower(version), "mariadb") {
		t.Skip("this native regression requires MySQL 5.5")
	}
	name := fmt.Sprintf("tegola_identity_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+quoteIdent(name)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE " + quoteIdent(name)); err != nil {
			t.Error(err)
		}
	})
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, statement := range []string{
		"CREATE TABLE items(id BIGINT PRIMARY KEY,geom TEXT NOT NULL,title TEXT) ENGINE=InnoDB",
		"INSERT INTO items VALUES(1,'POINT(1 2)','source')",
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := inspectFeatureSchema(ctx, db, name, "items", false); err == nil {
		t.Fatal("MySQL 5.5 unexpectedly admitted without opt-in")
	}
	schema, err := inspectFeatureSchema(ctx, db, name, "items", true)
	if err != nil {
		t.Fatal(err)
	}
	if !schema.legacyIdentity || !strings.HasPrefix(schema.physicalID, "mysql55-metadata:") {
		t.Fatalf("legacy identity not explicitly tagged: %q", schema.physicalID)
	}
	f := &featureProfile{schema: schema, id: "id", geometry: "geom", format: "wkt", srid: 4326, filter: "1"}
	layer := Layer{name: "items", tablename: "items", idFieldname: "id", geomFieldname: "geom", srid: 4326, geometryFormat: "wkt"}
	if err := f.resolveOrdinary(layer); err != nil {
		t.Fatal(err)
	}
	f.initializeFilterCatalog()
	layer.feature = f
	p := &Provider{db: db, Database: name, layers: map[string]Layer{"items": layer}, allowLegacyTableIdentity: true}
	var calls int
	_, err = p.QueryFeatures(ctx, "items", provider.FeatureQuery{Limit: 10}, func(feature *provider.Feature) error {
		calls++
		if feature.ID != 1 || feature.Tags["title"] != "source" {
			t.Fatalf("unexpected native row: %#v", feature)
		}
		return nil
	})
	if err != nil || calls != 1 {
		t.Fatalf("legacy snapshot read: calls=%d err=%v", calls, err)
	}
	if _, err := db.ExecContext(ctx, "ALTER TABLE items ADD extra BIGINT"); err != nil {
		t.Fatal(err)
	}
	calls = 0
	_, err = p.QueryFeatures(ctx, "items", provider.FeatureQuery{Limit: 10}, func(*provider.Feature) error { calls++; return nil })
	if err == nil || calls != 0 {
		t.Fatalf("catalog drift reached callback: calls=%d err=%v", calls, err)
	}
}
