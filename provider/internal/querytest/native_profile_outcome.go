package querytest

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"net"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider/geometrycodec"
)

type NativeProfileCase uint8

const (
	NativeMixedCollection NativeProfileCase = iota + 1
	NativeInvalidChildNormalization
)

type NativeProfileOutcomeKind uint8

const (
	NativeProfileStorageRejected NativeProfileOutcomeKind = iota + 1
	NativeProfileStorageNormalized
)

// NativeProfileOptions declares an independently measured native storage profile.
// These options never relax raw wire or provider query expectations.
type NativeProfileOptions struct {
	Backend, Version string
	Cases            []NativeProfileCase
}

// NativeProfileEvidence records an actual constructor/export operation. Reference
// controls establish validator mechanics only, never database causation.
type NativeProfileEvidence struct {
	Case                        NativeProfileCase
	Kind                        NativeProfileOutcomeKind
	Backend, Version            string
	Operation                   NativeStorageOperation
	AttemptedWire, ExportedWire []byte
	Error                       error
	SQLState                    string
	VendorCode, InternalCode    int
}

const hanaNativeVersion = "2.00.088.00.1760424921"
const mixedNativeWire = "01070000000200000001010000000000000000000000000000000000000001e9030000000000000000144000000000000014400000000000005940"
const invalidChildNativeWire = "01ef0300000200000001e903000000000000000000000000000000000000000000000000000001e9030000000000000000f03f010000000000f87f0000000000000040"
const normalizedChildNativeWire = "01ef0300000200000001e903000000000000000000000000000000000000000000000000000001ec03000000000000"

func copyNativeProfileOptions(p *NativeProfileOptions) (*NativeProfileOptions, error) {
	if p == nil {
		return nil, nil
	}
	if p.Backend != "hana" || p.Version != hanaNativeVersion || len(p.Cases) == 0 {
		return nil, fmt.Errorf("querytest: unreviewed native profile")
	}
	out := *p
	out.Cases = append([]NativeProfileCase(nil), p.Cases...)
	seen := map[NativeProfileCase]bool{}
	for _, id := range out.Cases {
		if id < NativeMixedCollection || id > NativeInvalidChildNormalization || seen[id] {
			return nil, fmt.Errorf("querytest: invalid native profile case")
		}
		seen[id] = true
	}
	return &out, nil
}
func nativeProfileCaseFor(p *NativeProfileOptions, c caseSpec) NativeProfileCase {
	if p == nil {
		return 0
	}
	for _, id := range p.Cases {
		if id == NativeMixedCollection && c.name == "mixed XY has unconstrained vertical" {
			return id
		}
		if id == NativeInvalidChildNormalization && c.name == "later invalid XYZ child cannot hide behind intersection" {
			return id
		}
	}
	return 0
}
func literalWire(value string) []byte {
	out, err := hex.DecodeString(value)
	if err != nil {
		panic("querytest: invalid static wire golden")
	}
	return out
}
func copyNativeEvidence(e *NativeProfileEvidence) *NativeProfileEvidence {
	if e == nil {
		return nil
	}
	out := *e
	out.AttemptedWire = append([]byte(nil), e.AttemptedWire...)
	out.ExportedWire = append([]byte(nil), e.ExportedWire...)
	return &out
}
func nativeProfileFixtureValid(c caseSpec, id NativeProfileCase) bool {
	if len(c.fixture.Rows) != 1 || c.fixture.Rows[0].Feature.ID != 10 || c.fixture.NativeProfileCase != id {
		return false
	}
	row := c.fixture.Rows[0]
	if c.preCancel || c.deadline || c.cancelAfterFirst || c.callbackFailure || c.invalidMapping || c.nilCallback || c.missingLayer || c.fixture.InvalidTemporalMapping {
		return false
	}
	expectedName := "mixed XY has unconstrained vertical"
	if id == NativeInvalidChildNormalization {
		expectedName = "later invalid XYZ child cannot hide behind intersection"
	}
	contextMatches := false
	for _, expected := range dimensionalCases() {
		if expected.name == expectedName && reflect.DeepEqual(c.query, expected.query) && reflect.DeepEqual(c.fixture.Spatial, expected.fixture.Spatial) {
			contextMatches = true
			break
		}
	}
	if !contextMatches {
		return false
	}
	if row.Metadata || row.MissingID || row.EmptyGeometry || row.MalformedGeometry != "" || row.RawEmptyGeometry != nil || row.RawTemporal != nil {
		return false
	}
	if id == NativeMixedCollection {
		return c.name == "mixed XY has unconstrained vertical" && !c.malformed && reflect.DeepEqual(row.Feature.Geometry, geom.Collection{geom.Point{0, 0}, geom.PointZ{5, 5, 100}})
	}
	g, ok := row.Feature.Geometry.(geom.Collection)
	if !ok || len(g) != 2 || !reflect.DeepEqual(g[0], geom.PointZ{0, 0, 0}) {
		return false
	}
	point, ok := g[1].(geom.PointZ)
	return c.name == "later invalid XYZ child cannot hide behind intersection" && c.malformed && c.sourceData && ok && point[0] == 1 && math.Float64bits(point[1]) == 0x7ff8000000000001 && point[2] == 2
}
func checkNativeProfileEvidence(instance Instance, c caseSpec, p *NativeProfileOptions) []string {
	e := copyNativeEvidence(instance.NativeProfileOutcome)
	id := nativeProfileCaseFor(p, c)
	if e == nil || p == nil || id == 0 || e.Case != id || !nativeProfileFixtureValid(c, id) {
		return []string{"native profile evidence has unrelated fixture/profile context"}
	}
	if instance.SetupError != nil || instance.StorageRejectedNativeCorruption != nil || e.Backend != p.Backend || e.Version != p.Version || e.Operation != NativeGeometryConstruction {
		return []string{"native profile evidence has wrong setup/backend/version/operation"}
	}
	if id == NativeMixedCollection {
		if e.Kind != NativeProfileStorageRejected || !bytes.Equal(e.AttemptedWire, literalWire(mixedNativeWire)) || len(e.ExportedWire) != 0 || e.SQLState != "HY000" || e.VendorCode != 669 || e.InternalCode != 1600408 {
			return []string{"native rejection does not match measured literal operation"}
		}
		if e.Error == nil || errors.Is(e.Error, context.Canceled) || errors.Is(e.Error, context.DeadlineExceeded) || errors.Is(e.Error, fs.ErrPermission) {
			return []string{"native rejection has unrelated error"}
		}
		var network net.Error
		if errors.As(e.Error, &network) {
			return []string{"native rejection has network error"}
		}
		return nil
	}
	if e.Kind != NativeProfileStorageNormalized || e.Error != nil || e.SQLState != "" || e.VendorCode != 0 || e.InternalCode != 0 || !bytes.Equal(e.AttemptedWire, literalWire(invalidChildNativeWire)) || !bytes.Equal(e.ExportedWire, literalWire(normalizedChildNativeWire)) {
		return []string{"native normalization differs from measured literal wires"}
	}
	decoded, err := geometrycodec.DecodeRawWKB(e.ExportedWire)
	expected := geom.Collection{geom.PointZ{0, 0, 0}, geom.MultiPointZ{}}
	if err != nil || !reflect.DeepEqual(decoded, expected) {
		return []string{"native normalized export lost exact independent structure"}
	}
	return nil
}
func normalizedNativeCase(c caseSpec) caseSpec {
	fixture := cloneFixture(c.fixture)
	fixture.NativeProfileCase = 0
	fixture.Rows[0].Feature.Geometry = geom.Collection{geom.PointZ{0, 0, 0}, geom.MultiPointZ{}}
	return caseSpec{name: "declared valid native normalized collection query", fixture: fixture, query: c.query, ids: []uint64{10}, matched: 1}
}
func runNativeProfileOutcome(t *testing.T, factory Factory, instance Instance, c caseSpec, p *NativeProfileOptions) {
	t.Helper()
	issues := checkNativeProfileEvidence(instance, c, p)
	for _, issue := range issues {
		t.Error(issue)
	}
	if len(issues) != 0 {
		return
	}
	if c.fixture.NativeProfileCase == NativeMixedCollection {
		t.Log("outcome=native-storage-capability-rejection; original query oracle not executed; actual operation causation requires independent receipt review")
		return
	}
	t.Log("outcome=native-storage-normalization; invalid original source is not counted as a query pass")
	t.Run("separate declared valid normalized query", func(t *testing.T) {
		normalized := normalizedNativeCase(c)
		next := factory(t, cloneFixture(normalized.fixture))
		registerInstanceCleanup(t, next)
		if issues, handled := checkSetup(next, normalized); handled {
			for _, issue := range issues {
				t.Error(issue)
			}
			return
		}
		if next.CountMode > OptionalCount {
			t.Fatal("invalid count mode")
		}
		for _, issue := range checkCase(next, normalized) {
			t.Error(issue)
		}
	})
}
