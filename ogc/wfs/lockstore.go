package wfs

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// A10/A11: persistent lock store.
//
// Locks were held in a process-global map, invisible to other server
// instances and lost on restart. This file defines the LockStore
// interface and two implementations:
//   - memoryLockStore: the previous in-memory behavior (default).
//   - sqlLockStore: DB-backed leases shared across processes.
//
// A lock lease carries: opaque token, normalized type name, feature IDs,
// owner, acquisition time, and expiry. The token is crypto-random, not
// time-based.

// LockStore persists feature locks.
type LockStore interface {
	// Acquire locks features; returns nil on conflict.
	Acquire(ctx context.Context, typeName string, ids []uint64, owner string, expiry time.Duration) (*FeatureLock, error)
	// Release removes a lock by ID.
	Release(ctx context.Context, id string) error
	// Check verifies that lockID covers (typeName, featureID).
	Check(ctx context.Context, lockID, typeName string, featureID uint64) *Exception
	// IsLocked reports whether any non-expired lock covers the feature.
	IsLocked(ctx context.Context, typeName string, featureID uint64) bool
}

// newLockID generates a crypto-random opaque token.
func newLockID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return "lock-" + hex.EncodeToString(b[:])
}

// normalizeTypeName strips namespace prefixes for comparison.
// TODO(A09): replace with namespace catalog lookup.
func normalizeTypeName(tn string) string {
	return stripPrefix(tn)
}

// --- SQL-backed store ---

// SQLLockStoreDDL creates the lock tables (R06: separate leases and members).
// One lease (token) can cover multiple features without PK violation.
const SQLLockStoreDDL = `
CREATE TABLE IF NOT EXISTS tegola_lock_leases (
	lock_id TEXT PRIMARY KEY,
	owner TEXT NOT NULL DEFAULT '',
	acquired_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS tegola_lock_members (
	lock_id TEXT NOT NULL REFERENCES tegola_lock_leases(lock_id) ON DELETE CASCADE,
	type_name TEXT NOT NULL,
	feature_id INTEGER NOT NULL,
	PRIMARY KEY (lock_id, type_name, feature_id)
);
CREATE INDEX IF NOT EXISTS idx_tegola_lock_members_lookup ON tegola_lock_members(type_name, feature_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tegola_lock_members_guard ON tegola_lock_members(type_name, feature_id);
`

// MySQLLockStoreDDL is the MySQL variant (R06).
const MySQLLockStoreDDL = `
CREATE TABLE IF NOT EXISTS tegola_lock_leases (
	lock_id VARCHAR(64) PRIMARY KEY,
	owner VARCHAR(255) NOT NULL DEFAULT '',
	acquired_at BIGINT NOT NULL,
	expires_at BIGINT NOT NULL
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS tegola_lock_members (
	lock_id VARCHAR(64) NOT NULL,
	type_name VARCHAR(255) NOT NULL,
	feature_id BIGINT NOT NULL,
	PRIMARY KEY (lock_id, type_name, feature_id),
	UNIQUE KEY uq_lock_member_guard (type_name, feature_id),
	INDEX idx_lock_member_lookup (type_name, feature_id),
	CONSTRAINT fk_lock_member_lease FOREIGN KEY (lock_id) REFERENCES tegola_lock_leases(lock_id) ON DELETE CASCADE
) ENGINE=InnoDB;
`

// sqlLockStore is a DB-backed LockStore.
type sqlLockStore struct {
	db      *sql.DB
	dialect string // "sqlite" or "mysql"
}

// NewSQLLockStore creates a DB-backed lock store.
// NewSQLLockStore creates a DB-backed LockStore, ensuring the lock tables
// exist (A10: persistent physical guards). Returns an error if the schema
// cannot be created.
func NewSQLLockStore(ctx context.Context, db *sql.DB, dialect string) (LockStore, error) {
	var ddl string
	switch dialect {
	case "mysql":
		ddl = MySQLLockStoreDDL
	case "sqlite":
		ddl = SQLLockStoreDDL
	default:
		return nil, fmt.Errorf("wfs: unsupported lock store dialect %q", dialect)
	}
	for _, statement := range strings.Split(ddl, ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return nil, fmt.Errorf("wfs: lock store schema: %w", err)
		}
	}
	return &sqlLockStore{db: db, dialect: dialect}, nil
}

// NewSQLLockStoreOrPanic is like NewSQLLockStore but panics on error.
// For use in tests and simple setups.
func NewSQLLockStoreOrPanic(db *sql.DB, dialect string) LockStore {
	s, err := NewSQLLockStore(context.Background(), db, dialect)
	if err != nil {
		panic(err)
	}
	return s
}

func (s *sqlLockStore) Acquire(ctx context.Context, typeName string, ids []uint64, owner string, expiry time.Duration) (*FeatureLock, error) {
	if expiry <= 0 {
		expiry = 5 * time.Minute
	}
	now := time.Now().UTC()
	exp := now.Add(expiry)
	norm := normalizeTypeName(typeName)

	// R06: atomic acquire with new schema. Clean expired, check conflicts,
	// insert lease + members in one tx. Fail-closed on storage errors.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	// Cleanup must preserve the operation/commit error; after Commit the
	// rollback returns sql.ErrTxDone.
	defer func() { _ = tx.Rollback() }()

	nowNano := now.UnixNano()
	expNano := exp.UnixNano()

	// SQLite foreign key enforcement is connection-specific, so explicitly
	// remove members before their leases as well as declaring the FK.
	if _, err := tx.ExecContext(ctx, `DELETE FROM tegola_lock_members WHERE lock_id IN
		(SELECT lock_id FROM tegola_lock_leases WHERE expires_at < ?)`, nowNano); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM tegola_lock_leases WHERE expires_at < ?`, nowNano); err != nil {
		return nil, err
	}

	// Check conflicts via the unique guard.
	for _, id := range ids {
		var existing string
		err := tx.QueryRowContext(ctx,
			`SELECT m.lock_id FROM tegola_lock_members m
			 JOIN tegola_lock_leases l ON l.lock_id = m.lock_id
			 WHERE m.type_name = ? AND m.feature_id = ? AND l.expires_at >= ? LIMIT 1`,
			norm, id, nowNano).Scan(&existing)
		if err == nil {
			return nil, nil // conflict: already locked
		}
		if err != sql.ErrNoRows {
			return nil, err // R06: fail-closed, not fail-open
		}
	}

	lockID := newLockID()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO tegola_lock_leases (lock_id, owner, acquired_at, expires_at) VALUES (?, ?, ?, ?)`,
		lockID, owner, nowNano, expNano); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO tegola_lock_members (lock_id, type_name, feature_id) VALUES (?, ?, ?)`,
			lockID, norm, id); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &FeatureLock{
		ID:         lockID,
		TypeName:   typeName,
		FeatureIDs: ids,
		Owner:      owner,
		Acquired:   now,
		Expires:    exp,
	}, nil
}

func (s *sqlLockStore) Release(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// Cleanup must preserve the operation/commit error; after Commit the
	// rollback returns sql.ErrTxDone.
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `DELETE FROM tegola_lock_members WHERE lock_id = ?`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM tegola_lock_leases WHERE lock_id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *sqlLockStore) Check(ctx context.Context, lockID, typeName string, featureID uint64) *Exception {
	// R06: verify membership in the lease, not just lock_id existence.
	now := time.Now().UTC().UnixNano()
	norm := normalizeTypeName(typeName)
	var one int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM tegola_lock_members m
		 JOIN tegola_lock_leases l ON l.lock_id = m.lock_id
		 WHERE m.lock_id = ? AND m.type_name = ? AND m.feature_id = ? AND l.expires_at >= ?`,
		lockID, norm, featureID, now).Scan(&one)
	if err != nil {
		if err == sql.ErrNoRows {
			return &Exception{Code: ExceptionInvalidParameterValue, Locator: "lockId", Text: fmt.Sprintf("lock %q does not cover %s.%d or expired", lockID, typeName, featureID)}
		}
		// R06: storage error -> fail-closed.
		return &Exception{Code: ExceptionNoApplicableCode, Locator: "lockId", Text: fmt.Sprintf("lock store error: %v", err)}
	}
	return nil
}

func (s *sqlLockStore) IsLocked(ctx context.Context, typeName string, featureID uint64) bool {
	// R06: fail-closed. Storage error -> treat as locked.
	now := time.Now().UTC().UnixNano()
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT m.lock_id FROM tegola_lock_members m
		 JOIN tegola_lock_leases l ON l.lock_id = m.lock_id
		 WHERE m.type_name = ? AND m.feature_id = ? AND l.expires_at >= ? LIMIT 1`,
		normalizeTypeName(typeName), featureID, now).Scan(&id)
	return err != sql.ErrNoRows
}

// --- In-memory store (default, single process) ---

type memoryLockStore struct {
	mu    sync.Mutex
	locks map[string]*FeatureLock
}

// NewMemoryLockStore creates the default in-memory lock store.
func NewMemoryLockStore() LockStore {
	return &memoryLockStore{locks: map[string]*FeatureLock{}}
}

func (s *memoryLockStore) Acquire(_ context.Context, typeName string, ids []uint64, owner string, expiry time.Duration) (*FeatureLock, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	// Expire old locks.
	for id, l := range s.locks {
		if now.After(l.Expires) {
			delete(s.locks, id)
		}
	}
	// Check conflicts (normalized type names).
	norm := normalizeTypeName(typeName)
	for _, l := range s.locks {
		if normalizeTypeName(l.TypeName) != norm {
			continue
		}
		for _, id := range ids {
			for _, locked := range l.FeatureIDs {
				if id == locked {
					return nil, nil // conflict
				}
			}
		}
	}
	if expiry <= 0 {
		expiry = 5 * time.Minute
	}
	lock := &FeatureLock{
		ID:         newLockID(),
		TypeName:   typeName,
		FeatureIDs: ids,
		Owner:      owner,
		Acquired:   now,
		Expires:    now.Add(expiry),
	}
	s.locks[lock.ID] = lock
	return lock, nil
}

func (s *memoryLockStore) Release(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.locks, id)
	return nil
}

func (s *memoryLockStore) Check(_ context.Context, lockID, typeName string, featureID uint64) *Exception {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.locks[lockID]
	if !ok {
		return &Exception{Code: ExceptionInvalidParameterValue, Locator: "lockId", Text: "unknown lock " + lockID}
	}
	if time.Now().UTC().After(l.Expires) {
		delete(s.locks, lockID)
		return &Exception{Code: ExceptionInvalidParameterValue, Locator: "lockId", Text: "lock expired"}
	}
	if normalizeTypeName(l.TypeName) != normalizeTypeName(typeName) {
		return &Exception{Code: ExceptionInvalidParameterValue, Locator: "lockId", Text: "lock does not cover this type"}
	}
	for _, id := range l.FeatureIDs {
		if id == featureID {
			return nil
		}
	}
	return &Exception{Code: ExceptionInvalidParameterValue, Locator: "lockId", Text: "lock does not cover this feature"}
}

func (s *memoryLockStore) IsLocked(_ context.Context, typeName string, featureID uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	norm := normalizeTypeName(typeName)
	for _, l := range s.locks {
		if now.After(l.Expires) {
			continue
		}
		if normalizeTypeName(l.TypeName) != norm {
			continue
		}
		for _, id := range l.FeatureIDs {
			if id == featureID {
				return true
			}
		}
	}
	return false
}
