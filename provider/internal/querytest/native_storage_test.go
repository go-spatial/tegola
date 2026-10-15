package querytest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

func nativeStorageCase(t *testing.T) caseSpec {
	t.Helper()
	for _, c := range contractCases() {
		if c.name == "malformed explicit native" {
			return c
		}
	}
	t.Fatal("native wire corruption case missing")
	return caseSpec{}
}

// A reference error proves validator mechanics, not storage enforcement.
func referenceNativeEvidence() *NativeStorageEvidence {
	return &NativeStorageEvidence{
		Backend: "postgis", Operation: NativeGeometryConstruction,
		Scope: MalformedNativeWire, FeatureID: 10,
		Error: errors.New("reference native parse error"), SQLState: "XX000",
	}
}

func TestNativeStorageEvidenceReferenceOutcome(t *testing.T) {
	c := nativeStorageCase(t)
	cleaned := false
	t.Run("reference shape only", func(t *testing.T) {
		instance := Instance{
			StorageRejectedNativeCorruption: referenceNativeEvidence(),
			Cleanup:                         func() { cleaned = true },
		}
		registerInstanceCleanup(t, instance)
		if !runNativeStorageOutcome(t, instance, c) {
			t.Fatal("native storage alternate outcome not selected")
		}
	})
	if !cleaned {
		t.Fatal("alternate outcome resource cleanup missing")
	}
	if runNativeStorageOutcome(t, Instance{}, c) {
		t.Fatal("ordinary query path replaced without evidence")
	}
}

func TestNativeStorageEvidenceRejectsBrokenDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Instance, *caseSpec)
	}{
		{"missing error", func(i *Instance, _ *caseSpec) { i.StorageRejectedNativeCorruption.Error = nil }},
		{"cancel", func(i *Instance, _ *caseSpec) {
			i.StorageRejectedNativeCorruption.Error = fmt.Errorf("operation: %w", context.Canceled)
		}},
		{"deadline", func(i *Instance, _ *caseSpec) { i.StorageRejectedNativeCorruption.Error = context.DeadlineExceeded }},
		{"permission", func(i *Instance, _ *caseSpec) { i.StorageRejectedNativeCorruption.Error = fs.ErrPermission }},
		{"network", func(i *Instance, _ *caseSpec) {
			i.StorageRejectedNativeCorruption.Error = &net.DNSError{Err: "unavailable"}
		}},
		{"missing code", func(i *Instance, _ *caseSpec) { i.StorageRejectedNativeCorruption.SQLState = "" }},
		{"SQLSTATE short", func(i *Instance, _ *caseSpec) { i.StorageRejectedNativeCorruption.SQLState = "XX" }},
		{"SQLSTATE source data", func(i *Instance, _ *caseSpec) { i.StorageRejectedNativeCorruption.SQLState = "secret-source" }},
		{"SQLSTATE lower", func(i *Instance, _ *caseSpec) { i.StorageRejectedNativeCorruption.SQLState = "xx000" }},
		{"SQLSTATE success", func(i *Instance, _ *caseSpec) { i.StorageRejectedNativeCorruption.SQLState = "00000" }},
		{"SQLSTATE auth", func(i *Instance, _ *caseSpec) { i.StorageRejectedNativeCorruption.SQLState = "28000" }},
		{"SQLSTATE network", func(i *Instance, _ *caseSpec) { i.StorageRejectedNativeCorruption.SQLState = "08006" }},
		{"SQLSTATE setup", func(i *Instance, _ *caseSpec) { i.StorageRejectedNativeCorruption.SQLState = "42000" }},
		{"vendor auth", func(i *Instance, _ *caseSpec) {
			i.StorageRejectedNativeCorruption.Backend = "mysql"
			i.StorageRejectedNativeCorruption.SQLState = ""
			i.StorageRejectedNativeCorruption.VendorCode = 1045
		}},
		{"negative vendor code", func(i *Instance, _ *caseSpec) { i.StorageRejectedNativeCorruption.VendorCode = -1 }},
		{"wrong ID", func(i *Instance, _ *caseSpec) { i.StorageRejectedNativeCorruption.FeatureID = 20 }},
		{"wrong operation", func(i *Instance, _ *caseSpec) { i.StorageRejectedNativeCorruption.Operation = 0 }},
		{"wrong scope", func(i *Instance, _ *caseSpec) { i.StorageRejectedNativeCorruption.Scope = 0 }},
		{"wrong backend", func(i *Instance, _ *caseSpec) { i.StorageRejectedNativeCorruption.Backend = "reference" }},
		{"setup error", func(i *Instance, _ *caseSpec) { i.SetupError = errors.New("setup failed") }},
		{"ordinary fixture", func(_ *Instance, c *caseSpec) { c.malformed = false }},
		{"raw WKB substitution", func(_ *Instance, c *caseSpec) { c.fixture.Rows[0].MalformedGeometry = "wkb" }},
		{"missing fixture ID", func(_ *Instance, c *caseSpec) { c.fixture.Rows[0].MissingID = true }},
		{"unrelated query", func(_ *Instance, c *caseSpec) { c.query.IDs = []uint64{20} }},
		{"cancellation case", func(_ *Instance, c *caseSpec) { c.preCancel = true }},
		{"multiple rows", func(_ *Instance, c *caseSpec) { c.fixture.Rows = append(c.fixture.Rows, c.fixture.Rows[0]) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := nativeStorageCase(t)
			instance := Instance{StorageRejectedNativeCorruption: referenceNativeEvidence()}
			tc.change(&instance, &c)
			cleaned := false
			t.Run("bad evidence cleanup", func(t *testing.T) {
				instance.Cleanup = func() { cleaned = true }
				registerInstanceCleanup(t, instance)
				if len(checkNativeStorageEvidence(instance, c)) == 0 {
					t.Fatal("invalid declaration accepted")
				}
			})
			if !cleaned {
				t.Fatal("bad declaration resources not cleaned")
			}
		})
	}
}

func TestNativeStorageEvidenceVendorCodeShape(t *testing.T) {
	e := referenceNativeEvidence()
	e.Backend, e.SQLState, e.VendorCode = "mysql", "", 3037
	if issues := checkNativeStorageEvidence(Instance{StorageRejectedNativeCorruption: e}, nativeStorageCase(t)); len(issues) != 0 {
		t.Fatal(issues)
	}
	// The evidence validator does not weaken the old raw corruption oracle.
	c := nativeStorageCase(t)
	instance := Instance{Querier: brokenQuerier{run: func(context.Context, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		return provider.FeatureQueryResult{}, nil
	}}}
	if len(checkCase(instance, c)) == 0 {
		t.Fatal("native corruption accepted by unchanged query oracle")
	}
}
