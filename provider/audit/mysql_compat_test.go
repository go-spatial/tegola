package audit

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
)

// This native gate creates and drops only its uniquely named service database.
// On MySQL 5.5 it also exercises the original 767-byte InnoDB index limit.
func TestMySQLServiceSchemaUnicodeKeys(t *testing.T) {
	dsn := os.Getenv("TEGOLA_REVIEW_MYSQL_ADMIN_DSN")
	if dsn == "" {
		t.Skip("TEGOLA_REVIEW_MYSQL_ADMIN_DSN not set")
	}
	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid native test DSN")
	}
	cfg.DBName = ""
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	cfg.Params["charset"] = "utf8mb4"
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
	name := fmt.Sprintf("tegola_audit_compat_%d", time.Now().UnixNano())
	quoted := "`" + name + "`"
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+quoted+" CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE " + quoted); err != nil {
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
	if err := Migrate(ctx, db, "mysql"); err != nil {
		t.Fatalf("fresh utf8mb4 migration: %v", err)
	}
	if err := CheckTableEngine(ctx, db, "mysql"); err != nil {
		t.Fatal(err)
	}
	// Physical names may be case-distinct and contain 64 BMP characters.
	// Neither case folding nor a prefix PRIMARY KEY may merge these rows.
	names := []string{"Road", "road", strings.Repeat("路", 63) + "甲", strings.Repeat("路", 63) + "乙"}
	for i, name := range names {
		if _, err := db.ExecContext(ctx, "INSERT INTO tegola_revisions(collection,feature_id,revision) VALUES(?,1,?)", name, i+1); err != nil {
			t.Fatalf("distinct physical key %d: %v", i, err)
		}
	}
	for i, name := range names {
		var revision int
		if err := db.QueryRowContext(ctx, "SELECT revision FROM tegola_revisions WHERE collection=? AND feature_id=1", name).Scan(&revision); err != nil || revision != i+1 {
			t.Fatalf("physical key lookup %d: revision=%d error=%v", i, revision, err)
		}
	}
	// Public names/actors remain full Unicode; only the audit lookup index
	// may use a non-unique prefix, never the stored value or revision PK.
	public := strings.Repeat("😀", 255)
	if _, err := db.ExecContext(ctx, "INSERT INTO tegola_audit(ts,actor,collection,operation,feature_id) VALUES('now',?,?,'insert',1)", "оператор 😀", public); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO tegola_outbox(ts,event_type,collection,feature_id) VALUES('now','changed',?,1)", public); err != nil {
		t.Fatal(err)
	}
	var got string
	if err := db.QueryRowContext(ctx, "SELECT collection FROM tegola_audit LIMIT 1").Scan(&got); err != nil || got != public {
		t.Fatalf("Unicode public collection lost: %v", err)
	}
	// Re-running startup migration preserves exact revision values.
	if err := Migrate(ctx, db, "mysql"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tegola_revisions").Scan(&count); err != nil || count != len(names) {
		t.Fatalf("migration lost revision keys: count=%d error=%v", count, err)
	}
	// Existing service layouts are never narrowed implicitly. This legacy
	// ASCII layout can contain historical keys outside today's table-name
	// contract; an idempotent startup must not truncate or reinterpret them.
	if _, err := db.ExecContext(ctx, "DROP TABLE tegola_revisions"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "CREATE TABLE tegola_revisions(collection VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,feature_id BIGINT NOT NULL,revision BIGINT NOT NULL DEFAULT 0,incarnation BIGINT NOT NULL DEFAULT 0,PRIMARY KEY(collection,feature_id)) ENGINE=InnoDB"); err != nil {
		t.Fatal(err)
	}
	oldKey := strings.Repeat("a", 200)
	if _, err := db.ExecContext(ctx, "INSERT INTO tegola_revisions VALUES(?,1,73,4)", oldKey); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db, "mysql"); err != nil {
		t.Fatal(err)
	}
	var oldRevision, oldIncarnation int
	if err := db.QueryRowContext(ctx, "SELECT collection,revision,incarnation FROM tegola_revisions").Scan(&got, &oldRevision, &oldIncarnation); err != nil || got != oldKey || oldRevision != 73 || oldIncarnation != 4 {
		t.Fatalf("existing layout changed: revision=%d incarnation=%d err=%v", oldRevision, oldIncarnation, err)
	}
}
