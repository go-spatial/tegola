package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

type phase09CancelMarshaler struct {
	cancel context.CancelFunc
	calls  *int
}

func (value phase09CancelMarshaler) MarshalJSON() ([]byte, error) {
	*value.calls++
	value.cancel()
	return []byte(`{"source":"must-never-be-published"}`), nil
}

// This controlled encoder test cancels inside MarshalJSON; it does not claim
// that public service normalization admits arbitrary caller marshaler objects.
func TestFeatureEncoderCancellationAcceptance(t *testing.T) {
	const expected = `{"code":"RequestTimeout","description":"Request did not complete"}`
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, format := range []string{"json", "html"} {
			t.Run(method+"_"+format, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				calls := 0
				api := &FeatureAPI{cfg: FeatureAPIConfig{MaxResponseBytes: 4096, QueryTimeout: time.Second}}
				r := httptest.NewRequest(method, "/features?f="+format, nil).WithContext(ctx)
				w := httptest.NewRecorder()
				w.Header().Set("Content-Crs", "private")
				api.protocolHandler(api.negotiate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					api.writeRepresentation(w, r, 200, "application/json", phase09CancelMarshaler{cancel, &calls}, nil)
				}), "application/json")).ServeHTTP(w, r)
				if calls != 1 || w.Code != 408 || w.Header().Get("Content-Type") != "application/json" ||
					w.Header().Get("Content-Crs") != "" || w.Header().Get("Cache-Control") != "no-store" ||
					w.Header().Get("Content-Length") != strconv.Itoa(len(expected)) {
					t.Fatalf("encoding cancellation: calls=%d status=%d headers=%v", calls, w.Code, w.Header())
				}
				if method == http.MethodHead {
					if w.Body.Len() != 0 {
						t.Fatal("HEAD emitted a body")
					}
				} else if w.Body.String() != expected {
					t.Fatal("partial source or unexpected cancellation body")
				}
			})
		}
	}
}
