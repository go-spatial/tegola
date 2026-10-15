package mysql

import "testing"

// TestResolveDeferredHeaderSRIDMixedSRIDs (audit P5-9): deferred custom-SQL
// layers without an explicit CRS establish their canonical CRS from the
// first non-zero native geometry header SRID. Rows carrying a different SRID
// must be skipped (not silently mislabeled); rows without a header SRID (0)
// always process and never lock the CRS. Explicit CRS config always wins.
func TestResolveDeferredHeaderSRIDMixedSRIDs(t *testing.T) {
	t.Parallel()

	t.Run("first non-zero header SRID adopts and locks", func(t *testing.T) {
		t.Parallel()
		l := &Layer{deferredInspection: true}
		if got := l.resolveDeferredHeaderSRID(4326); got != deferredSRIDAdopt {
			t.Fatalf("first header row: got %v, want adopt", got)
		}
		if l.srid != 4326 {
			t.Fatalf("adopted srid = %d, want 4326", l.srid)
		}
		if !l.deferredSRIDLocked {
			t.Fatal("CRS should be locked after first non-zero header SRID")
		}
	})

	t.Run("first header row matching provisional SRID still locks", func(t *testing.T) {
		t.Parallel()
		l := &Layer{deferredInspection: true, srid: 3857}
		if got := l.resolveDeferredHeaderSRID(3857); got != deferredSRIDProcess {
			t.Fatalf("matching first row: got %v, want process", got)
		}
		if !l.deferredSRIDLocked {
			t.Fatal("CRS should be locked after first non-zero header SRID")
		}
		// a later different row must now be skipped, not re-adopted
		if got := l.resolveDeferredHeaderSRID(4326); got != deferredSRIDSkip {
			t.Fatalf("mixed row: got %v, want skip", got)
		}
		if l.srid != 3857 {
			t.Fatalf("canonical srid mutated to %d, want 3857", l.srid)
		}
	})

	t.Run("mixed sequence skips mismatches and accepts header-less rows", func(t *testing.T) {
		t.Parallel()
		l := &Layer{deferredInspection: true}
		rows := []uint64{0, 4326, 4326, 3857, 0, 4326}
		want := []deferredSRIDAction{
			deferredSRIDProcess, // no header SRID: process, never locks
			deferredSRIDAdopt,   // first non-zero header SRID wins
			deferredSRIDProcess, // same SRID
			deferredSRIDSkip,    // different SRID: skip + warn
			deferredSRIDProcess, // no header SRID: accepted
			deferredSRIDProcess, // canonical SRID again
		}
		for i, srid := range rows {
			if got := l.resolveDeferredHeaderSRID(srid); got != want[i] {
				t.Errorf("row %d (srid %d): got %v, want %v", i, srid, got, want[i])
			}
		}
		if l.srid != 4326 {
			t.Fatalf("canonical srid = %d, want 4326", l.srid)
		}
	})

	t.Run("header-less rows never lock the CRS", func(t *testing.T) {
		t.Parallel()
		l := &Layer{deferredInspection: true}
		if got := l.resolveDeferredHeaderSRID(0); got != deferredSRIDProcess {
			t.Fatalf("srid 0: got %v, want process", got)
		}
		if l.deferredSRIDLocked {
			t.Fatal("srid 0 must not lock the CRS")
		}
		if got := l.resolveDeferredHeaderSRID(4326); got != deferredSRIDAdopt {
			t.Fatalf("first non-zero after srid 0 rows: got %v, want adopt", got)
		}
	})

	t.Run("explicit CRS is authoritative", func(t *testing.T) {
		t.Parallel()
		l := &Layer{deferredInspection: true, crsExplicit: true, srid: 3857}
		if got := l.resolveDeferredHeaderSRID(4326); got != deferredSRIDProcess {
			t.Fatalf("explicit CRS row: got %v, want process", got)
		}
		if l.srid != 3857 {
			t.Fatalf("explicit srid mutated to %d, want 3857", l.srid)
		}
		if l.deferredSRIDLocked {
			t.Fatal("explicit CRS must not lock deferred header state")
		}
	})

	t.Run("non-deferred layers are untouched", func(t *testing.T) {
		t.Parallel()
		l := &Layer{srid: 3857}
		if got := l.resolveDeferredHeaderSRID(4326); got != deferredSRIDProcess {
			t.Fatalf("non-deferred row: got %v, want process", got)
		}
		if l.srid != 3857 {
			t.Fatalf("srid mutated to %d, want 3857", l.srid)
		}
	})
}
