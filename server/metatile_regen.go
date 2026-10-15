package server

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/alexeydott/tegola/internal/log"
)

// Metatile regeneration runs off the HTTP request path: ?tile=update and
// ?tile=getupdated schedule the full metatile render pass on the bounded
// background scheduler below instead of paying up to metatileSize² tile
// renders while holding the metatile mutation lock. The scheduler provides:
//
//   - single-flight per metatile: concurrent requests for the same metatile
//     join the regeneration that is already scheduled or running, so N
//     requests trigger at most one regeneration (the file cache's staleness
//     fixup relies on this property),
//   - bounded work: at most metatileRegenConcurrency regenerations render at
//     once and at most metatileRegenQueue more wait for a rendering slot;
//     beyond that requests are rejected with 503 instead of queueing
//     unboundedly,
//   - failure retry: a failed regeneration is logged at WARN and forgotten,
//     so the next request starts a fresh attempt; failures are never cached,
//   - graceful shutdown: ShutdownMetatileRegeneration cancels in-flight
//     renders and waits for them, so no regeneration writes to a cache that
//     is being torn down behind it.

const (
	// metatileRegenConcurrency is the number of metatile regenerations that
	// render concurrently. Each regeneration renders its tiles one at a time,
	// so this is also the number of renders the background pool drives:
	// deliberately half of the tile operation gate's default of 4 so
	// foreground traffic keeps headroom.
	metatileRegenConcurrency = 2
	// metatileRegenQueue is the number of regenerations that may wait for a
	// rendering slot before further requests are rejected with 503.
	metatileRegenQueue = 16
	// metatileRegenShutdownTimeout bounds how long process shutdown waits for
	// in-flight regenerations to observe cancellation and return.
	metatileRegenShutdownTimeout = 5 * time.Second
)

// errMetatileRegenUnavailable is returned by enqueue when the scheduler is
// saturated or shutting down. Requests map it to 503 Service Unavailable;
// nothing about the attempt is remembered, so the next request retries.
var errMetatileRegenUnavailable = errors.New("metatile regeneration unavailable: scheduler busy or shutting down")

// metatileRegenCall is the record of one scheduled or running regeneration.
type metatileRegenCall struct{}

// metatileRegenScheduler runs metatile regeneration functions on a bounded
// background pool with at most one in flight per metatile key.
type metatileRegenScheduler struct {
	mu     sync.Mutex
	calls  map[string]*metatileRegenCall
	closed bool

	// admit bounds scheduled regenerations and therefore scheduler goroutines.
	admit chan struct{}
	// run bounds regenerations that are rendering concurrently.
	run chan struct{}

	rootCtx context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newMetatileRegenScheduler() *metatileRegenScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &metatileRegenScheduler{
		calls:   make(map[string]*metatileRegenCall),
		admit:   make(chan struct{}, metatileRegenConcurrency+metatileRegenQueue),
		run:     make(chan struct{}, metatileRegenConcurrency),
		rootCtx: ctx,
		cancel:  cancel,
	}
}

// enqueue schedules fn for the metatile identified by key and returns
// immediately. At most one fn per key is scheduled or running: requests that
// arrive while one is in flight join it (joined=true) and fn is not run for
// them. fn receives a context that is canceled at shutdown. When the scheduler
// is saturated or shutting down, enqueue returns errMetatileRegenUnavailable
// without scheduling anything.
func (s *metatileRegenScheduler) enqueue(key string, fn func(ctx context.Context) error) (joined bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return false, errMetatileRegenUnavailable
	}
	if _, ok := s.calls[key]; ok {
		return true, nil
	}
	// reserve an admission slot before recording the call so the number of
	// scheduler goroutines stays bounded
	select {
	case s.admit <- struct{}{}:
	default:
		return false, errMetatileRegenUnavailable
	}
	call := &metatileRegenCall{}
	s.calls[key] = call
	s.wg.Add(1)

	go s.runCall(key, call, fn)
	return false, nil
}

// runCall executes one scheduled regeneration and releases its bookkeeping.
// Panics are recovered and logged so a faulty regeneration cannot take the
// scheduler down.
func (s *metatileRegenScheduler) runCall(key string, call *metatileRegenCall, fn func(ctx context.Context) error) {
	defer func() {
		// forget the call before releasing waiters: a request arriving after
		// wait() returns must schedule a fresh regeneration, never join a
		// finished one
		s.mu.Lock()
		if s.calls[key] == call {
			delete(s.calls, key)
		}
		s.mu.Unlock()
		s.wg.Done()
		<-s.admit
	}()

	defer func() {
		if p := recover(); p != nil {
			log.Errorf("metatile regeneration panicked for %v: %v", key, p)
		}
	}()

	// hold a rendering slot for the whole regeneration so at most
	// metatileRegenConcurrency full metatile passes render at once
	select {
	case s.run <- struct{}{}:
		defer func() { <-s.run }()
	case <-s.rootCtx.Done():
		log.Debugf("metatile regeneration canceled before start for %v: %v", key, s.rootCtx.Err())
		return
	}

	if err := fn(s.rootCtx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			log.Debugf("metatile regeneration canceled for %v: %v", key, err)
		} else {
			log.Warnf("metatile regeneration failed for %v: %v (will retry on next request)", key, err)
		}
	}
}

// isUpdating includes accepted work waiting for a rendering slot or mutation
// lock, so clients cannot mistake a queued regeneration for completion.
func (s *metatileRegenScheduler) isUpdating(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.calls[key]
	return ok
}

// wait blocks until every scheduled regeneration has completed. Regenerations
// are forgotten before wait returns, so a later request always schedules a
// fresh regeneration.
func (s *metatileRegenScheduler) wait() {
	s.wg.Wait()
}

// shutdown stops accepting work, cancels in-flight regenerations and waits for
// them to return or until ctx expires. A regeneration that ignores
// cancellation delays the return until it finishes; if ctx expires first, the
// waiting goroutine remains until the stragglers return (the scheduler is
// process-scoped, so this cannot repeat unboundedly).
func (s *metatileRegenScheduler) shutdown(ctx context.Context) error {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		s.cancel()
	}
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// metatileRegens is the process-wide regeneration scheduler used by the tile
// operation handlers. Tests drive their own scheduler instances; the singleton
// is only shut down through ShutdownMetatileRegeneration.
var metatileRegens = newMetatileRegenScheduler()

// AwaitMetatileRegeneration blocks until every scheduled metatile regeneration
// has completed. It exists so tests (and embedders that need regeneration to
// finish before proceeding) can wait deterministically instead of sleeping.
func AwaitMetatileRegeneration() {
	metatileRegens.wait()
}

// ShutdownMetatileRegeneration cancels in-flight metatile regenerations and
// waits up to metatileRegenShutdownTimeout for them to return. It is
// registered with gdcmd.OnComplete after server shutdown and before
// observability/provider cleanup, so no background regeneration writes to a
// cache that is being torn down behind it.
func ShutdownMetatileRegeneration() {
	ctx, cancel := context.WithTimeout(context.Background(), metatileRegenShutdownTimeout)
	defer cancel()
	if err := metatileRegens.shutdown(ctx); err != nil {
		log.Warnf("metatile regeneration shutdown: %v", err)
	}
}
