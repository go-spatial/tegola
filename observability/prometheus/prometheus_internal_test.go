package prometheus

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexeydott/tegola/dict"
)

// P6-35 regression: constructing a second observer must not panic. Pre-fix,
// build info (and the http/cache collectors) were MustRegister'd on the global
// default registry, so a second New() panicked with AlreadyRegisteredError.
func TestNewTwiceDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("New() panicked: %v", r)
		}
	}()

	obs1, err := New(dict.Dict{})
	if err != nil {
		t.Fatalf("first New: %v", err)
	}
	obs2, err := New(dict.Dict{})
	if err != nil {
		t.Fatalf("second New: %v", err)
	}

	// each observer must own a private registry
	o1, o2 := obs1.(*observer), obs2.(*observer)
	if o1.registry == o2.registry {
		t.Error("observers share one registry, want a private registry per instance")
	}

	// instrumenting caches on both observers exercises the per-instance cache
	// collector registration too (same metric names, distinct registries)
	o1.InstrumentedCache(&failingPurgeCache{})
	o2.InstrumentedCache(&failingPurgeCache{})

	// both metrics endpoints must serve
	for name, o := range map[string]*observer{"first": o1, "second": o2} {
		rec := httptest.NewRecorder()
		o.Handler("").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s observer /metrics status = %d, want 200", name, rec.Code)
		}
	}
}
