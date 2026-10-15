package server

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/alexeydott/geom/slippy"
	"github.com/alexeydott/tegola/atlas"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

// The tests below exercise the metatile regeneration scheduler with fresh
// instances so the process-wide singleton is never closed mid-suite. All
// coordination uses channels and the scheduler's own wait hook, never sleeps,
// so the tests are deterministic.

// TestMetatileRegenSchedulerSingleFlight asserts that concurrent requests for
// the same metatile join the regeneration already in flight — at most one
// regeneration runs per metatile key — and that a later request starts a
// fresh one.
func TestMetatileRegenSchedulerSingleFlight(t *testing.T) {
	s := newMetatileRegenScheduler()

	var runs int32
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	fn := func(ctx context.Context) error {
		atomic.AddInt32(&runs, 1)
		started <- struct{}{}
		<-release
		return nil
	}

	joined, err := s.enqueue("key", fn)
	if err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	if joined {
		t.Fatal("first enqueue must start a regeneration, not join one")
	}

	// the regeneration is now registered and held inside fn
	<-started

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			joined, err := s.enqueue("key", fn)
			if err != nil {
				t.Errorf("concurrent enqueue: %v", err)
				return
			}
			if !joined {
				t.Error("concurrent enqueue must join the regeneration in flight")
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&runs); got != 1 {
		t.Fatalf("concurrent requests started %d regenerations, want 1", got)
	}

	close(release)
	s.wait()

	if got := atomic.LoadInt32(&runs); got != 1 {
		t.Fatalf("regeneration ran %d times, want 1", got)
	}

	// a request after completion starts a fresh regeneration
	joined, err = s.enqueue("key", func(ctx context.Context) error {
		atomic.AddInt32(&runs, 1)
		return nil
	})
	if err != nil {
		t.Fatalf("re-enqueue: %v", err)
	}
	if joined {
		t.Fatal("re-enqueue after completion must start a fresh regeneration")
	}
	s.wait()

	if got := atomic.LoadInt32(&runs); got != 2 {
		t.Fatalf("fn ran %d times, want 2", got)
	}
}

// TestMetatileRegenSchedulerRetriesAfterFailure asserts that a failed
// regeneration is never remembered: the next request regenerates again.
func TestMetatileRegenSchedulerRetriesAfterFailure(t *testing.T) {
	s := newMetatileRegenScheduler()

	var runs int32
	fn := func(ctx context.Context) error {
		if atomic.AddInt32(&runs, 1) == 1 {
			return errors.New("render failed")
		}
		return nil
	}

	if _, err := s.enqueue("key", fn); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	s.wait()

	joined, err := s.enqueue("key", fn)
	if err != nil {
		t.Fatalf("enqueue after failure: %v", err)
	}
	if joined {
		t.Fatal("a failed regeneration must not linger in flight")
	}
	s.wait()

	if got := atomic.LoadInt32(&runs); got != 2 {
		t.Fatalf("fn ran %d times, want 2 (failure must not be cached)", got)
	}
}

// TestMetatileRegenSchedulerBoundedConcurrency asserts that the scheduler
// never renders more than metatileRegenConcurrency regenerations at once and
// sheds load beyond its admission capacity instead of spawning goroutines
// without bound.
func TestMetatileRegenSchedulerBoundedConcurrency(t *testing.T) {
	s := newMetatileRegenScheduler()

	const capacity = metatileRegenConcurrency + metatileRegenQueue

	release := make(chan struct{})
	started := make(chan struct{}, capacity)
	var running, maxRunning, completed int32

	fn := func(ctx context.Context) error {
		cur := atomic.AddInt32(&running, 1)
		for {
			old := atomic.LoadInt32(&maxRunning)
			if cur <= old || atomic.CompareAndSwapInt32(&maxRunning, old, cur) {
				break
			}
		}
		started <- struct{}{}
		<-release
		atomic.AddInt32(&running, -1)
		atomic.AddInt32(&completed, 1)
		return nil
	}

	// admit exactly the scheduler capacity on distinct keys
	for i := 0; i < capacity; i++ {
		if _, err := s.enqueue(fmt.Sprintf("key%d", i), fn); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	// beyond capacity requests are shed with a retryable error
	joined, err := s.enqueue("overflow", fn)
	if !errors.Is(err, errMetatileRegenUnavailable) {
		t.Fatalf("overflow enqueue: err = %v, want %v", err, errMetatileRegenUnavailable)
	}
	if joined {
		t.Fatal("overflow enqueue must not join anything")
	}

	// exactly metatileRegenConcurrency regenerations run at once
	for i := 0; i < metatileRegenConcurrency; i++ {
		<-started
	}
	select {
	case <-started:
		t.Fatal("more than metatileRegenConcurrency regenerations running at once")
	default:
	}

	close(release)
	s.wait()

	if got := atomic.LoadInt32(&maxRunning); got != int32(metatileRegenConcurrency) {
		t.Fatalf("max concurrent regenerations = %d, want %d", got, metatileRegenConcurrency)
	}
	if got := atomic.LoadInt32(&completed); got != int32(capacity) {
		t.Fatalf("completed regenerations = %d, want %d", got, capacity)
	}
}

// TestMetatileRegenSchedulerShutdownCancelsInFlight asserts that shutdown
// cancels the regeneration in flight and does not return until it has stopped.
func TestMetatileRegenSchedulerShutdownCancelsInFlight(t *testing.T) {
	s := newMetatileRegenScheduler()

	started := make(chan struct{}, 1)
	var finished int32

	if _, err := s.enqueue("key", func(ctx context.Context) error {
		started <- struct{}{}
		<-ctx.Done()
		atomic.AddInt32(&finished, 1)
		return ctx.Err()
	}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	<-started

	if err := s.shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if got := atomic.LoadInt32(&finished); got != 1 {
		t.Fatalf("shutdown returned with %d of 1 regenerations finished", got)
	}

	joined, err := s.enqueue("key", func(ctx context.Context) error { return nil })
	if !errors.Is(err, errMetatileRegenUnavailable) {
		t.Fatalf("enqueue after shutdown: err = %v, want %v", err, errMetatileRegenUnavailable)
	}
	if joined {
		t.Fatal("enqueue after shutdown must not join anything")
	}
}

// TestMetatileRegenSchedulerShutdownWaitsForWork asserts that shutdown waits
// for work that does not honor cancellation instead of leaking it.
func TestMetatileRegenSchedulerShutdownWaitsForWork(t *testing.T) {
	s := newMetatileRegenScheduler()

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var done int32

	if _, err := s.enqueue("key", func(ctx context.Context) error {
		started <- struct{}{}
		<-release // deliberately ignores cancellation
		atomic.AddInt32(&done, 1)
		return nil
	}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	<-started

	shutdownErr := make(chan error, 1)
	go func() { shutdownErr <- s.shutdown(context.Background()) }()

	// shutdown must not report completion while the regeneration runs
	select {
	case err := <-shutdownErr:
		t.Fatalf("shutdown returned before work finished: %v", err)
	default:
	}

	close(release)
	if err := <-shutdownErr; err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if got := atomic.LoadInt32(&done); got != 1 {
		t.Fatalf("regeneration done = %d, want 1", got)
	}
}

// TestMetatileRegenSchedulerRecoversPanic asserts that a panicking
// regeneration is contained, forgotten, and does not poison later calls.
func TestMetatileRegenSchedulerRecoversPanic(t *testing.T) {
	s := newMetatileRegenScheduler()

	if _, err := s.enqueue("key", func(ctx context.Context) error {
		panic("render exploded")
	}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	s.wait()

	var ran atomic.Bool
	joined, err := s.enqueue("key", func(ctx context.Context) error {
		ran.Store(true)
		return nil
	})
	if err != nil {
		t.Fatalf("enqueue after panic: %v", err)
	}
	if joined {
		t.Fatal("a panicked regeneration must not linger in flight")
	}
	s.wait()

	if !ran.Load() {
		t.Fatal("the scheduler must keep serving regenerations after a panic")
	}
}

func TestMetatileRegenQueuedWorkIsUpdating(t *testing.T) {
	s := newMetatileRegenScheduler()
	old := metatileRegens
	metatileRegens = s
	defer func() { metatileRegens = old }()
	req := HandleMapLayerZXY{Atlas: &atlas.Atlas{}, mapName: "queued"}
	tile := slippy.Tile{Z: 4, X: 0, Y: 0}
	key := req.metatileLockKey(tile)
	release := make(chan struct{})
	started := make(chan struct{}, metatileRegenConcurrency)
	defer func() { close(release); s.wait() }()
	for i := 0; i < metatileRegenConcurrency; i++ {
		_, err := s.enqueue(fmt.Sprintf("busy-%d", i), func(context.Context) error { started <- struct{}{}; <-release; return nil })
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < metatileRegenConcurrency; i++ {
		<-started
	}
	if _, err := s.enqueue(key, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if !s.isUpdating(key) {
		t.Fatal("accepted queued regeneration must report updating")
	}
	w := httptest.NewRecorder()
	if err := req.serveTileOperation(w, httptest.NewRequest("GET", "/?tile=status", nil), atlas.Map{}, tile, tileOperationStatus); err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := zr.Close(); err != nil {
			t.Errorf("close gzip reader: %v", err)
		}
	}()
	var status tileStatusResponse
	if err := json.NewDecoder(zr).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if !status.Updating {
		t.Fatal("HTTP status must report accepted queued job as updating")
	}
	if s.isUpdating("absent") {
		t.Fatal("absent regeneration reported updating")
	}
}
