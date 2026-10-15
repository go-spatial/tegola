package wfs

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// LockFeature implements WFS 1.1 and 2.0 feature locking.
//
// A11: WFS 2.0 DOES define LockFeature (the previous comment claiming
// removal was wrong). Both versions are supported; wire differences are
// handled in the HTTP adapters.
//
// A10: locks are held via a LockStore. The default is in-memory (single
// process). Deployments with multiple instances MUST configure a shared
// SQL-backed store via SetLockStore.

// FeatureLock is one held lock. Owner identifies the locker (A10).
type FeatureLock struct {
	ID         string
	TypeName   string
	FeatureIDs []uint64
	Owner      string
	Acquired   time.Time
	Expires    time.Time
}

var (
	lockStoreMu sync.RWMutex
	lockStore   LockStore = NewMemoryLockStore()
	lockExpiry            = 5 * time.Minute // default expiry
)

// SetLockStore installs a shared lock store (A10). Must be called before
// serving traffic.
func SetLockStore(s LockStore) {
	lockStoreMu.Lock()
	defer lockStoreMu.Unlock()
	if s != nil {
		lockStore = s
	}
}

func getLockStore() LockStore {
	lockStoreMu.RLock()
	defer lockStoreMu.RUnlock()
	return lockStore
}

// AcquireLock locks features and returns the lock, or nil on conflict.
func AcquireLock(typeName string, ids []uint64, expiry time.Duration) *FeatureLock {
	if expiry <= 0 {
		expiry = lockExpiry
	}
	lock, err := getLockStore().Acquire(context.Background(), typeName, ids, "", expiry)
	if err != nil || lock == nil {
		return nil
	}
	return lock
}

// ReleaseLock releases a lock by ID.
func ReleaseLock(id string) bool {
	err := getLockStore().Release(context.Background(), id)
	return err == nil
}

// CheckLock verifies that a lock ID covers the given feature.
// Returns nil if the lock is valid for the feature.
func CheckLock(lockID, typeName string, featureID uint64) *Exception {
	return getLockStore().Check(context.Background(), lockID, typeName, featureID)
}

// IsLocked reports whether a feature is currently locked (by any lock).
func IsLocked(typeName string, featureID uint64) bool {
	return getLockStore().IsLocked(context.Background(), typeName, featureID)
}

// LockFeatureResponse renders the WFS 1.1 LockFeature response.
// LockFeatureResponse builds the version-specific response (R07).
func LockFeatureResponse(lock *FeatureLock, version string) string {
	var sb strings.Builder
	ns := "http://www.opengis.net/wfs"
	if version == "2.0.0" || version == "2.0" {
		ns = "http://www.opengis.net/wfs/2.0"
	}
	sb.WriteString(fmt.Sprintf(`<wfs:LockFeatureResponse xmlns:wfs="%s">`, ns))
	sb.WriteString(fmt.Sprintf(`<wfs:LockId>%s</wfs:LockId>`, xmlEscape(lock.ID)))
	sb.WriteString(`</wfs:LockFeatureResponse>`)
	return sb.String()
}
