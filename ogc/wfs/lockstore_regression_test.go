//go:build cgo

package wfs

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestSQLLockStoreFailsClosedAndReleasesMembers(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close fixture database: %v", err)
		}
	})
	ctx := context.Background()
	s, err := NewSQLLockStore(ctx, db, "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	l, err := s.Acquire(ctx, "sites", []uint64{1, 2}, "user", time.Minute)
	if err != nil || l == nil {
		t.Fatalf("acquire: %v %v", l, err)
	}
	if err = s.Release(ctx, l.ID); err != nil {
		t.Fatal(err)
	}
	l, err = s.Acquire(ctx, "sites", []uint64{1, 2}, "user", time.Minute)
	if err != nil || l == nil {
		t.Fatalf("reacquire: %v %v", l, err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database for failure probe: %v", err)
	}
	if !s.IsLocked(ctx, "sites", 3) {
		t.Fatal("database failure allows unlocked mutation")
	}
}
