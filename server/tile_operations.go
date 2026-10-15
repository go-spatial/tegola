package server

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"sync"
	"time"
)

// TileOperationsTokenHeader is the request header carrying the token that
// authorizes cache-maintenance tile operations (?tile=update, ?tile=getupdated,
// ?tile=status and ?dirty regeneration).
const TileOperationsTokenHeader = "X-Tegola-Tile-Operations-Token"

// default tile-operations limits; used whenever the corresponding
// TileOperationsConfig field is zero or negative.
const (
	defaultTileOperationsRatePerMinute = 60
	defaultTileOperationsMaxConcurrent = 4
)

// tileOperationsRateWindow is the window the fixed rate limiter resets on.
const tileOperationsRateWindow = time.Minute

// Errors returned by gateTileOperation and rendered by writeTileOperationDenied.
var (
	// ErrTileOperationsDisabled is returned when a tile operation is requested
	// while TileOperations.Enabled is false (the default).
	ErrTileOperationsDisabled = errors.New("tile operations are disabled; enable webserver.tile_operations to use them")
	// ErrTileOperationsUnauthorized is returned when the tile-operations token
	// is missing or wrong. It is also returned when tile operations are enabled
	// without a configured token: the gate fails closed instead of letting
	// unauthenticated cache maintenance through.
	ErrTileOperationsUnauthorized = errors.New("tile operations require a valid token")
	// ErrTileOperationsRateLimited is returned when the per-minute rate limit
	// for tile operations is exhausted.
	ErrTileOperationsRateLimited = errors.New("tile operations rate limit exceeded")
	// ErrTileOperationsBusy is returned when too many tile operations run
	// concurrently.
	ErrTileOperationsBusy = errors.New("too many concurrent tile operations")
)

// TileOperationsConfig controls access to cache-maintenance tile operations
// (?tile=update, ?tile=getupdated, ?tile=status and ?dirty).
//
// The zero value disables all tile operations; requests carrying these query
// parameters are answered with 403 Forbidden. This default is deliberate:
// unauthenticated tile operations can trigger up to a full metatile
// (metatileSize x metatileSize) of renders and cache writes per request, which
// is a denial-of-service vector.
//
// Wiring from the TOML config (webserver.tile_operations.*) into this struct
// happens alongside the other webserver settings and is documented in
// README.md.
type TileOperationsConfig struct {
	// Enabled turns tile operations on. Off by default: with it off, requests
	// using ?tile=... or ?dirty are refused with 403.
	Enabled bool
	// Token is the shared secret clients must send in the
	// TileOperationsTokenHeader header. Must be non-empty when Enabled is set;
	// otherwise all tile operations are refused (fail closed).
	Token string
	// RatePerMinute caps how many tile operations are accepted per minute
	// (global, not per-client). Defaults to 60 when zero or negative.
	RatePerMinute int
	// MaxConcurrent caps how many mutating tile operations (update, getupdated
	// and ?dirty regeneration) run at the same time. Defaults to 4 when zero
	// or negative.
	MaxConcurrent int
}

// TileOperations holds the effective tile-operations configuration. It is
// populated at startup together with the other webserver settings.
var TileOperations TileOperationsConfig

func (c TileOperationsConfig) ratePerMinute() int {
	if c.RatePerMinute <= 0 {
		return defaultTileOperationsRatePerMinute
	}
	return c.RatePerMinute
}

func (c TileOperationsConfig) maxConcurrent() int {
	if c.MaxConcurrent <= 0 {
		return defaultTileOperationsMaxConcurrent
	}
	return c.MaxConcurrent
}

// tileOperationsGate is a fixed-window rate limiter plus a counting semaphore
// limiting concurrent mutating tile operations. Limits are global (not
// per-client): the operations are cache maintenance, not user traffic, so a
// single global budget is both sufficient and cheap to enforce.
type tileOperationsGate struct {
	mu          sync.Mutex
	windowStart time.Time
	used        int
	running     int
}

var tileOpsGate tileOperationsGate

// reset clears the gate counters. Used by tests.
func (g *tileOperationsGate) reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.windowStart = time.Time{}
	g.used = 0
	g.running = 0
}

// allow charges one operation against the rate limit.
func (g *tileOperationsGate) allow(cfg TileOperationsConfig) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	if g.windowStart.IsZero() || now.Sub(g.windowStart) >= tileOperationsRateWindow {
		g.windowStart = now
		g.used = 0
	}
	if g.used >= cfg.ratePerMinute() {
		return false
	}
	g.used++
	return true
}

// tryAcquire takes a concurrency slot for a mutating operation. The returned
// function releases it and is safe to call more than once.
func (g *tileOperationsGate) tryAcquire(cfg TileOperationsConfig) (func(), bool) {
	g.mu.Lock()
	if g.running >= cfg.maxConcurrent() {
		g.mu.Unlock()
		return nil, false
	}
	g.running++
	g.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			g.running--
			g.mu.Unlock()
		})
	}, true
}

// gateTileOperation authorizes and rate-limits a cache-maintenance tile
// operation. When concurrent is true (mutating operations) a concurrency slot
// is taken as well and returned in the release function; the caller must call
// release (it is safe to call more than once). A non-nil error must be
// rendered with writeTileOperationDenied.
func gateTileOperation(r *http.Request, concurrent bool) (release func(), err error) {
	cfg := TileOperations

	if !cfg.Enabled {
		return nil, ErrTileOperationsDisabled
	}
	if cfg.Token == "" {
		// fail closed: an enabled gate without a token would be equivalent to
		// no authentication at all.
		return nil, ErrTileOperationsUnauthorized
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get(TileOperationsTokenHeader)), []byte(cfg.Token)) != 1 {
		return nil, ErrTileOperationsUnauthorized
	}
	if !tileOpsGate.allow(cfg) {
		return nil, ErrTileOperationsRateLimited
	}
	if concurrent {
		release, ok := tileOpsGate.tryAcquire(cfg)
		if !ok {
			return nil, ErrTileOperationsBusy
		}
		return release, nil
	}
	return func() {}, nil
}

// writeTileOperationDenied renders the error returned by gateTileOperation.
func writeTileOperationDenied(w http.ResponseWriter, err error) {
	status := http.StatusForbidden
	switch {
	case errors.Is(err, ErrTileOperationsRateLimited):
		status = http.StatusTooManyRequests
	case errors.Is(err, ErrTileOperationsBusy):
		status = http.StatusServiceUnavailable
	}
	http.Error(w, err.Error(), status)
}
