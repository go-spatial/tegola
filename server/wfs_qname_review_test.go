//go:build cgo

package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWFSDescribeAdvertisedQName(t *testing.T) {
	_, router := wfsHandler(t, wfsService(t))
	for _, tc := range []struct {
		name   string
		status int
	}{
		{name: "app:wfs_sites", status: http.StatusOK},
		{name: "wfs_sites", status: http.StatusOK},
		{name: "unknown:wfs_sites", status: http.StatusBadRequest},
		{name: "app:app:wfs_sites", status: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
				"/wfs?service=WFS&request=DescribeFeatureType&version=2.0.0&typenames="+tc.name, nil))
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
			if tc.status == http.StatusOK && !strings.Contains(rec.Body.String(), `name="wfs_sites"`) {
				t.Fatalf("missing advertised local type: %s", rec.Body.String())
			}
		})
	}
}
