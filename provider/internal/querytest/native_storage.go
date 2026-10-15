package querytest

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

type NativeStorageOperation uint8

const (
	NativeGeometryConstruction NativeStorageOperation = iota + 1
	NativeGeometryInsertion
)

type NativeCorruptionScope uint8

const MalformedNativeWire NativeCorruptionScope = 1

// NativeStorageEvidence is test verification metadata, not a provider flag.
// Adapters must attempt literal bad native wire and check its specific database
// parse error class before returning this evidence. The harness only validates
// shape and case context; reference evidence cannot prove database causation.
type NativeStorageEvidence struct {
	Backend    string
	Operation  NativeStorageOperation
	Scope      NativeCorruptionScope
	FeatureID  uint64
	Error      error
	SQLState   string
	VendorCode int
}

func runNativeStorageOutcome(t *testing.T, instance Instance, c caseSpec) bool {
	t.Helper()
	if instance.StorageRejectedNativeCorruption == nil {
		return false
	}
	t.Run("native storage enforcement evidence", func(t *testing.T) {
		issues := checkNativeStorageEvidence(instance, c)
		for _, issue := range issues {
			t.Error(issue)
		}
		if len(issues) == 0 {
			evidence := instance.StorageRejectedNativeCorruption
			t.Logf("outcome=storage-enforcement backend=%s operation=%d scope=malformed-native-wire; "+
				"query oracle not executed; adapter operation/class evidence requires independent runtime review",
				evidence.Backend, evidence.Operation)
		}
	})
	return true
}

func checkNativeStorageEvidence(instance Instance, c caseSpec) []string {
	if instance.NativeProfileOutcome != nil || c.fixture.NativeProfileCase != 0 {
		return []string{"native corruption evidence overlaps an unrelated profile outcome"}
	}
	e := instance.StorageRejectedNativeCorruption
	if e == nil {
		return []string{"native storage evidence absent"}
	}
	if instance.SetupError != nil {
		return []string{"native storage outcome cannot replace setup failure"}
	}
	if !c.malformed || c.preCancel || c.deadline || c.cancelAfterFirst || c.callbackFailure ||
		c.invalidMapping || c.nilCallback || c.missingLayer || len(c.fixture.Rows) != 1 {
		return []string{"native storage outcome supplied outside native corruption case"}
	}
	if c.query.Limit == 0 || !reflect.DeepEqual(c.query, provider.FeatureQuery{Limit: c.query.Limit}) ||
		c.fixture.InvalidTemporalMapping {
		return []string{"native storage evidence has unrelated query or mapping context"}
	}
	row := c.fixture.Rows[0]
	if row.MalformedGeometry != "native" || row.MissingID || row.Metadata || row.EmptyGeometry ||
		row.Feature.ID != e.FeatureID || e.Scope != MalformedNativeWire {
		return []string{"native storage evidence has wrong fixture identity or wire scope"}
	}
	if e.Operation != NativeGeometryConstruction && e.Operation != NativeGeometryInsertion {
		return []string{"native storage evidence has invalid operation"}
	}
	switch e.Backend {
	case "mysql", "mariadb", "postgis", "hana":
	default:
		return []string{"native storage evidence has invalid backend"}
	}
	if e.Error == nil || errors.Is(e.Error, context.Canceled) || errors.Is(e.Error, context.DeadlineExceeded) ||
		errors.Is(e.Error, fs.ErrPermission) {
		return []string{"native storage evidence has missing or unrelated error"}
	}
	var network net.Error
	if errors.As(e.Error, &network) {
		return []string{"native storage evidence cannot use network failure"}
	}
	if e.VendorCode < 0 || (e.SQLState == "" && e.VendorCode == 0) {
		return []string{"native storage evidence has missing error code"}
	}
	if e.SQLState != "" {
		if len(e.SQLState) != 5 {
			return []string{"native storage evidence has invalid SQLSTATE shape"}
		}
		for _, r := range e.SQLState {
			if !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
				return []string{"native storage evidence has invalid SQLSTATE shape"}
			}
		}
		// These classes describe success, connection, authorization, syntax or
		// permission, transaction setup, resources and cancellation, not bad wire.
		for _, class := range []string{"00", "08", "28", "42", "40", "53", "57"} {
			if strings.HasPrefix(e.SQLState, class) {
				return []string{"native storage evidence uses unrelated SQLSTATE class"}
			}
		}
	}
	if e.Backend == "mysql" || e.Backend == "mariadb" {
		switch e.VendorCode {
		case 1044, 1045, 1142, 1143, 1227, 2002, 2003, 2006, 2013:
			return []string{"native storage evidence uses authentication, permission or connection code"}
		}
	}
	return nil
}
