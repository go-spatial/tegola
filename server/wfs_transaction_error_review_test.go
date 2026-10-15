package server

import (
	"fmt"
	"github.com/alexeydott/tegola/ogc/wfs"
	"github.com/alexeydott/tegola/provider"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWFSTransactionErrorsDoNotLeakPrivateDetails(t *testing.T) {
	const secret = "private_table postgres://secret-user:secret-password@internal-host/database"
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"storage", fmt.Errorf("SELECT * FROM %s", secret), 500},
		{"schema", &provider.MutationError{Kind: provider.MutationErrSchemaViolation, Reason: secret}, 400},
		{"denied", &provider.MutationError{Kind: provider.MutationErrDenied, Reason: secret}, 403},
		{"CAS", fmt.Errorf("wrapped: %w", &provider.MutationError{Kind: provider.MutationErrPreconditionFailed, Reason: secret}), 412},
		{"commit unknown", &provider.MutationError{Kind: provider.MutationErrCommitUnknown, Reason: secret}, 500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &WFSHandler{}
			rec := httptest.NewRecorder()
			h.writeTransactionError(rec, httptest.NewRequest(http.MethodPost, "/wfs", nil), wfs.V200, tc.err)
			if strings.Contains(rec.Body.String(), secret) || strings.Contains(rec.Body.String(), "secret-password") {
				t.Fatalf("private error leaked: %s", rec.Body.String())
			}
			if rec.Code != tc.status {
				t.Fatalf("status=%d want=%d", rec.Code, tc.status)
			}
		})
	}
}
