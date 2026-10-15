package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTileOperationsGateRateLimit(t *testing.T) {
	cfg := TileOperationsConfig{Enabled: true, Token: "t", RatePerMinute: 2}

	g := &tileOperationsGate{}
	g.reset()

	if !g.allow(cfg) {
		t.Fatal("first operation must pass the rate limit")
	}
	if !g.allow(cfg) {
		t.Fatal("second operation must pass the rate limit")
	}
	if g.allow(cfg) {
		t.Fatal("third operation must be rate limited")
	}

	// resetting the window makes the limiter usable again
	g.reset()
	if !g.allow(cfg) {
		t.Fatal("operation must pass after the rate window is reset")
	}

	// zero / negative limits fall back to the documented default
	g.reset()
	for i := 0; i < defaultTileOperationsRatePerMinute; i++ {
		if !g.allow(TileOperationsConfig{}) {
			t.Fatalf("operation %d must pass with the default rate limit", i)
		}
	}
	if g.allow(TileOperationsConfig{}) {
		t.Fatal("the default rate limit must eventually refuse")
	}
}

func TestTileOperationsGateConcurrency(t *testing.T) {
	cfg := TileOperationsConfig{Enabled: true, Token: "t", MaxConcurrent: 1}

	g := &tileOperationsGate{}
	g.reset()

	release, ok := g.tryAcquire(cfg)
	if !ok {
		t.Fatal("first concurrency slot must be granted")
	}
	if _, ok := g.tryAcquire(cfg); ok {
		t.Fatal("second concurrency slot must be refused while the first is held")
	}

	release()
	// release is idempotent
	release()
	release()

	release2, ok := g.tryAcquire(cfg)
	if !ok {
		t.Fatal("concurrency slot must be free again after release")
	}
	release2()
}

func TestGateTileOperationAuthorization(t *testing.T) {
	type tcase struct {
		cfg        TileOperationsConfig
		token      string // token header value sent with the request
		concurrent bool
		wantErr    error
	}

	tests := map[string]tcase{
		"disabled": {
			cfg:     TileOperationsConfig{Enabled: false, Token: "secret"},
			token:   "secret",
			wantErr: ErrTileOperationsDisabled,
		},
		"enabled without configured token fails closed": {
			cfg:     TileOperationsConfig{Enabled: true, Token: ""},
			token:   "anything",
			wantErr: ErrTileOperationsUnauthorized,
		},
		"missing token header": {
			cfg:     TileOperationsConfig{Enabled: true, Token: "secret"},
			token:   "",
			wantErr: ErrTileOperationsUnauthorized,
		},
		"wrong token": {
			cfg:     TileOperationsConfig{Enabled: true, Token: "secret"},
			token:   "wrong",
			wantErr: ErrTileOperationsUnauthorized,
		},
		"valid token": {
			cfg:   TileOperationsConfig{Enabled: true, Token: "secret", RatePerMinute: 100, MaxConcurrent: 10},
			token: "secret",
		},
		"valid token concurrent operation": {
			cfg:        TileOperationsConfig{Enabled: true, Token: "secret", RatePerMinute: 100, MaxConcurrent: 10},
			token:      "secret",
			concurrent: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			old := TileOperations
			defer func() { TileOperations = old }()
			TileOperations = tc.cfg
			tileOpsGate.reset()

			r := httptest.NewRequest(http.MethodGet, "/maps/test-map/test-layer/4/2/3.pbf?tile=update", nil)
			if tc.token != "" {
				r.Header.Set(TileOperationsTokenHeader, tc.token)
			}

			release, err := gateTileOperation(r, tc.concurrent)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("gateTileOperation() error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("gateTileOperation() unexpected error: %v", err)
			}
			release()
		})
	}
}

func TestGateTileOperationRateLimited(t *testing.T) {
	old := TileOperations
	defer func() { TileOperations = old }()
	TileOperations = TileOperationsConfig{Enabled: true, Token: "secret", RatePerMinute: 1, MaxConcurrent: 10}
	tileOpsGate.reset()

	r := httptest.NewRequest(http.MethodGet, "/maps/test-map/test-layer/4/2/3.pbf?tile=update", nil)
	r.Header.Set(TileOperationsTokenHeader, "secret")

	release, err := gateTileOperation(r, true)
	if err != nil {
		t.Fatalf("first operation must pass: %v", err)
	}
	release()

	_, err = gateTileOperation(r, true)
	if !errors.Is(err, ErrTileOperationsRateLimited) {
		t.Fatalf("second operation error = %v, want ErrTileOperationsRateLimited", err)
	}
}

func TestGateTileOperationBusy(t *testing.T) {
	old := TileOperations
	defer func() { TileOperations = old }()
	TileOperations = TileOperationsConfig{Enabled: true, Token: "secret", RatePerMinute: 100, MaxConcurrent: 1}
	tileOpsGate.reset()

	r := httptest.NewRequest(http.MethodGet, "/maps/test-map/test-layer/4/2/3.pbf?tile=update", nil)
	r.Header.Set(TileOperationsTokenHeader, "secret")

	release, err := gateTileOperation(r, true)
	if err != nil {
		t.Fatalf("first concurrent operation must pass: %v", err)
	}

	// the only concurrency slot is taken
	if _, err := gateTileOperation(r, true); !errors.Is(err, ErrTileOperationsBusy) {
		t.Fatalf("second concurrent operation error = %v, want ErrTileOperationsBusy", err)
	}

	// status operations do not take a concurrency slot and still pass
	releaseStatus, err := gateTileOperation(r, false)
	if err != nil {
		t.Fatalf("non-concurrent operation must pass while the gate is busy: %v", err)
	}
	releaseStatus()

	release()
	if _, err := gateTileOperation(r, true); err != nil {
		t.Fatalf("concurrency slot must be usable again after release: %v", err)
	}
}

func TestWriteTileOperationDeniedStatusCodes(t *testing.T) {
	tests := map[string]struct {
		err        error
		wantStatus int
	}{
		"disabled":     {err: ErrTileOperationsDisabled, wantStatus: http.StatusForbidden},
		"unauthorized": {err: ErrTileOperationsUnauthorized, wantStatus: http.StatusForbidden},
		"rate limited": {err: ErrTileOperationsRateLimited, wantStatus: http.StatusTooManyRequests},
		"busy":         {err: ErrTileOperationsBusy, wantStatus: http.StatusServiceUnavailable},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeTileOperationDenied(w, tc.err)
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}
		})
	}
}
