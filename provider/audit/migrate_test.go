//go:build cgo

package audit

import (
	"context"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"testing"
)

func TestMigrationUpgradePreservesRevisionAndRejectsFuture(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	}()
	db.SetMaxOpenConns(1)
	for _, q := range []string{`CREATE TABLE tegola_revisions(collection TEXT,feature_id INTEGER,revision INTEGER,PRIMARY KEY(collection,feature_id))`, `INSERT INTO tegola_revisions VALUES('features',7,12)`, `CREATE TABLE tegola_schema_version(version INTEGER PRIMARY KEY,applied_at TEXT NOT NULL)`, `INSERT INTO tegola_schema_version VALUES(1,'old')`} {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	if err = Migrate(ctx, db, "sqlite"); err != nil {
		t.Fatal(err)
	}
	var revision, incarnation int
	if err = db.QueryRow(`SELECT revision,incarnation FROM tegola_revisions WHERE feature_id=7`).Scan(&revision, &incarnation); err != nil || revision != 12 || incarnation != 0 {
		t.Fatalf("upgrade lost revisions: %d,%d %v", revision, incarnation, err)
	}
	if err = CheckSchemaVersion(ctx, db, "sqlite"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO tegola_schema_version VALUES(3,'future')`); err != nil {
		t.Fatal(err)
	}
	if err = CheckSchemaVersion(ctx, db, "sqlite"); err == nil {
		t.Fatal("future schema accepted")
	}
	if err = Migrate(ctx, db, "sqlite"); err == nil {
		t.Fatal("future schema migrated")
	}
}

func TestParseRevisionRejectsTrailingData(t *testing.T) {
	for _, value := range []string{"1.2junk", "1.2.3", "-1.2", "0.-1", "1x", ""} {
		if _, _, err := ParseRevision(value); err == nil {
			t.Errorf("accepted malformed revision %q", value)
		}
	}
}
