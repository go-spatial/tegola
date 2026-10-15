package server

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
)

type auditReceiptProvider struct {
	discoveryQuerier
	receipt   provider.CommitReceipt
	commitErr error
	applyErr  error
	commits   int
}

func (p *auditReceiptProvider) DescribeSchema(context.Context, string) (provider.SchemaDescriptor, error) {
	return provider.SchemaDescriptor{
		IDColumn: "id",
		Geometry: provider.GeometryColumnDescriptor{Name: "geom", Type: "point", SRID: 4326, Nullable: true},
		Columns:  []provider.ColumnDescriptor{{Name: "name", Type: "text", Nullable: true}},
	}, nil
}
func (p *auditReceiptProvider) DescribeWritable(context.Context, string) (provider.WriteDescriptor, error) {
	return provider.WriteDescriptor{Domain: "receipt-test"}, nil
}
func (p *auditReceiptProvider) BeginFeatureTx(context.Context, provider.TxOptions) (provider.FeatureTx, error) {
	return p, nil
}
func (p *auditReceiptProvider) Apply(context.Context, provider.Mutation) (provider.MutationOutcome, error) {
	return provider.MutationOutcome{FeatureID: 7, Affected: 1}, p.applyErr
}
func (p *auditReceiptProvider) Commit(context.Context) (provider.CommitReceipt, error) {
	p.commits++
	return p.receipt, p.commitErr
}
func (p *auditReceiptProvider) Rollback(context.Context) error { return nil }

func TestWFSAuditReceiptPreservesOutcome(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     provider.CommitStatus
		commitErr  error
		applyErr   error
		hookPanic  bool
		wantHTTP   int
		wantStatus string
		wantID     string
	}{
		{name: "unknown commit", status: provider.CommitUnknown, commitErr: errors.New("private database details"), wantHTTP: 500, wantStatus: "unknown", wantID: "audit-correlation"},
		{name: "confirmed rejection", status: provider.CommitNotCommitted, commitErr: errors.New("private database details"), wantHTTP: 500, wantStatus: "not-committed", wantID: "audit-correlation"},
		{name: "committed auxiliary failure", status: provider.CommitCommitted, commitErr: errors.New("private database details"), wantHTTP: 200, wantStatus: "committed", wantID: "audit-correlation"},
		{name: "committed callback panic", status: provider.CommitCommitted, hookPanic: true, wantHTTP: 200, wantStatus: "committed", wantID: "audit-correlation"},
		{name: "precommit failure", applyErr: errors.New("private database details"), wantHTTP: 500, wantStatus: "not-committed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &auditReceiptProvider{receipt: provider.CommitReceipt{Status: tc.status, TransactionID: "audit-correlation", Actor: "private actor"}, commitErr: tc.commitErr, applyErr: tc.applyErr}
			svc, err := features.NewService([]features.CollectionSource{{ID: "sites", Layer: discoveryLayer{}, Querier: p}})
			if err != nil {
				t.Fatal(err)
			}
			h := &WFSHandler{Service: svc, Config: config.WFSConfig{Enabled: true}.Resolved(), WriteConfig: config.FeaturesWriteConfig{Enabled: true, AuthMode: "dev", Collections: []config.WriteCollectionConfig{{ID: "sites", Operations: []string{"create"}}}}}
			invalidations := 0
			h.tileCacheInvalidate = func() { invalidations++ }
			if tc.hookPanic {
				h.OnMutate = func([]string) { panic("private callback details") }
			}
			body := `<wfs:Transaction xmlns:wfs="http://www.opengis.net/wfs/2.0" xmlns:app="http://example.com/tegola/sites" service="WFS" version="2.0.0"><wfs:Insert><app:sites><app:name>one</app:name></app:sites></wfs:Insert></wfs:Transaction>`
			r := httptest.NewRequest(http.MethodPost, "/wfs", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/xml")
			w := httptest.NewRecorder()
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Errorf("committed receipt lost to panic: %v", recovered)
					}
				}()
				h.ServeHTTP(w, r)
			}()
			wantInvalidations := 0
			if tc.applyErr == nil && (tc.status == provider.CommitCommitted || tc.status == provider.CommitUnknown) {
				wantInvalidations = 1
			}
			if invalidations != wantInvalidations {
				t.Fatalf("invalidations=%d want=%d", invalidations, wantInvalidations)
			}
			if w.Code != tc.wantHTTP || w.Header().Get("Tegola-Commit-Status") != tc.wantStatus || w.Header().Get("Tegola-Transaction-ID") != tc.wantID {
				t.Fatalf("outcome status=%d headers=%v body=%s", w.Code, w.Header(), w.Body.String())
			}
			if strings.Contains(w.Body.String(), "private") || strings.Contains(w.Body.String(), "Actor") {
				t.Fatal("private receipt or error leaked")
			}
			decoder := xml.NewDecoder(strings.NewReader(w.Body.String()))
			for {
				_, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if tc.applyErr != nil && p.commits != 0 {
				t.Fatal("committed after Apply failure")
			}
			if tc.wantID != "" && !strings.Contains(strings.ToLower(strings.Join(w.Header().Values("Access-Control-Expose-Headers"), ",")), "tegola-transaction-id") {
				t.Fatal("correlation header unavailable to browser")
			}
		})
	}
}
