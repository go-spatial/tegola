package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/alexeydott/tegola/cache"
)

const EditorActiveHeader = "X-Tegola-Editor-Active"

type writableTileCacheContextKey struct{}
type writableTileCacheSnapshot struct {
	namespace string
	editor    bool
}

// Each writable router starts cold. Old generations remain in the configured
// backend until its normal TTL/eviction removes them; they are never read again.
type writableTileCacheState struct {
	startup    string
	generation atomic.Uint64
}

func newWritableTileCacheState() *writableTileCacheState {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic(fmt.Errorf("tile cache namespace: %w", err))
	}
	return &writableTileCacheState{startup: hex.EncodeToString(id[:])}
}
func (s *writableTileCacheState) Invalidate() { s.generation.Add(1) }
func (s *writableTileCacheState) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := strings.TrimSpace(r.Header.Get(EditorActiveHeader))
		snapshot := writableTileCacheSnapshot{namespace: fmt.Sprintf("writable-%s-%d", s.startup, s.generation.Load()), editor: value == "1" || strings.EqualFold(value, "true")}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), writableTileCacheContextKey{}, snapshot)))
	})
}

// The wrapper captures a generation rather than consulting mutable state during
// Get/Set. A render started before a commit can only write its old namespace.
type generationTileCache struct {
	cache.Interface
	namespace string
}

func (c generationTileCache) physical(key *cache.Key) *cache.Key {
	copy := *key
	copy.MapName = c.namespace + "-" + key.MapName
	return &copy
}
func (c generationTileCache) Get(ctx context.Context, key *cache.Key) ([]byte, bool, error) {
	return c.Interface.Get(ctx, c.physical(key))
}
func (c generationTileCache) Set(ctx context.Context, key *cache.Key, val []byte) error {
	return c.Interface.Set(ctx, c.physical(key), val)
}
func (c generationTileCache) Purge(ctx context.Context, key *cache.Key) error {
	return c.Interface.Purge(ctx, c.physical(key))
}
func requestTileCache(ctx context.Context, fallback cache.Interface) cache.Interface {
	if fallback == nil {
		return nil
	}
	if snapshot, ok := ctx.Value(writableTileCacheContextKey{}).(writableTileCacheSnapshot); ok {
		return generationTileCache{Interface: fallback, namespace: snapshot.namespace}
	}
	return fallback
}
func editorTileCacheBypass(r *http.Request) bool {
	snapshot, _ := r.Context().Value(writableTileCacheContextKey{}).(writableTileCacheSnapshot)
	if !snapshot.editor {
		return false
	}
	if _, operation := r.URL.Query()[QueryKeyTile]; operation {
		return false
	}
	query := r.URL.Query()
	if value, ok := query[QueryKeyDirty]; ok && len(value) > 0 && (value[0] == "" || value[0] == "1" || strings.EqualFold(value[0], "true")) {
		return false
	}
	return true
}
func cacheCoordinationKey(c cache.Interface, key *cache.Key) string {
	if generation, ok := c.(generationTileCache); ok {
		return generation.physical(key).String()
	}
	return key.String()
}
func cacheMetatileLockKey(c cache.Interface, key *cache.Key) string {
	if generation, ok := c.(generationTileCache); ok {
		return metatileLockKeyForCacheKey(generation.physical(key))
	}
	return metatileLockKeyForCacheKey(key)
}
