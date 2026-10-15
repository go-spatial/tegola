package gcs

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"cloud.google.com/go/storage"
)

// P5-13 regression: Purge must be idempotent. Deleting an object that is
// already gone is a success, not an error.
func TestPurgeMissingObjectIsIdempotent(t *testing.T) {
	for name, delErr := range map[string]error{
		"bare": storage.ErrObjectNotExist,
		// the client wraps the sentinel around the raw 404/NotFound
		// (storage.formatObjectErr), so wrapped matches must also succeed
		"wrapped": fmt.Errorf("gcs: delete: %w", storage.ErrObjectNotExist),
	} {
		t.Run(name, func(t *testing.T) {
			c := &GCSCache{
				deleteObject: func(ctx context.Context, key string) error {
					return delErr
				},
			}

			// purging twice (already-missing then still-missing) must succeed
			if err := c.Purge(context.Background(), testKey()); err != nil {
				t.Fatalf("Purge of missing object returned %v, want nil (Purge must be idempotent)", err)
			}
			if err := c.Purge(context.Background(), testKey()); err != nil {
				t.Fatalf("second Purge of missing object returned %v, want nil", err)
			}
		})
	}
}

// A genuine backend failure during purge must still be surfaced.
func TestPurgePropagatesBackendError(t *testing.T) {
	c := &GCSCache{
		deleteObject: func(ctx context.Context, key string) error {
			return errTransient
		},
	}

	err := c.Purge(context.Background(), testKey())
	if !errors.Is(err, errTransient) {
		t.Fatalf("Purge returned %v, want the backend error %v", err, errTransient)
	}
}

// The happy path deletes exactly once and succeeds.
func TestPurgeSuccess(t *testing.T) {
	var deletedKeys []string
	c := &GCSCache{
		Basepath: "mybase",
		deleteObject: func(ctx context.Context, key string) error {
			deletedKeys = append(deletedKeys, key)
			return nil
		},
	}

	if err := c.Purge(context.Background(), testKey()); err != nil {
		t.Fatalf("Purge returned %v, want nil", err)
	}
	if len(deletedKeys) != 1 {
		t.Fatalf("deleted %d objects, want 1 (%v)", len(deletedKeys), deletedKeys)
	}
	if deletedKeys[0] != "mybase/test-map/test-layer/0/0/0" {
		t.Errorf("deleted key = %q, want %q (forward-slash object key)", deletedKeys[0], "mybase/test-map/test-layer/0/0/0")
	}
}
