package querytest

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/geometrycodec"
)

func nativeOutcomeProfile() *NativeProfileOptions {
	return &NativeProfileOptions{Backend: "hana", Version: hanaNativeVersion, Cases: []NativeProfileCase{NativeMixedCollection, NativeInvalidChildNormalization}}
}
func nativeOutcomeCase(t *testing.T, id NativeProfileCase) caseSpec {
	t.Helper()
	for _, c := range dimensionalCases() {
		if nativeProfileCaseFor(nativeOutcomeProfile(), c) == id {
			c.fixture.NativeProfileCase = id
			return c
		}
	}
	t.Fatal("static native profile case absent")
	return caseSpec{}
}
func nativeOutcomeEvidence(id NativeProfileCase) *NativeProfileEvidence {
	e := &NativeProfileEvidence{Case: id, Backend: "hana", Version: hanaNativeVersion, Operation: NativeGeometryConstruction}
	if id == NativeMixedCollection {
		e.Kind = NativeProfileStorageRejected
		e.AttemptedWire = literalWire(mixedNativeWire)
		e.Error = errors.New("reference spatial incompatibility")
		e.SQLState = "HY000"
		e.VendorCode = 669
		e.InternalCode = 1600408
	} else {
		e.Kind = NativeProfileStorageNormalized
		e.AttemptedWire = literalWire(invalidChildNativeWire)
		e.ExportedWire = literalWire(normalizedChildNativeWire)
	}
	return e
}
func TestNativeProfileLiteralGoldens(t *testing.T) {
	mixed, err := geometrycodec.DecodeRawWKB(literalWire(mixedNativeWire))
	if err != nil || !reflect.DeepEqual(mixed, geom.Collection{geom.Point{0, 0}, geom.PointZ{5, 5, 100}}) {
		t.Fatal("mixed literal golden differs", err)
	}
	normalized, err := geometrycodec.DecodeRawWKB(literalWire(normalizedChildNativeWire))
	if err != nil || !reflect.DeepEqual(normalized, geom.Collection{geom.PointZ{0, 0, 0}, geom.MultiPointZ{}}) {
		t.Fatal("normalized literal golden differs", err)
	}
	if _, err := geometrycodec.DecodeRawWKB(literalWire(invalidChildNativeWire)); err == nil {
		t.Fatal("NaN literal source accepted as valid raw geometry")
	}
}
func TestNativeProfilePositiveMechanicsAndCleanup(t *testing.T) {
	for _, id := range []NativeProfileCase{NativeMixedCollection, NativeInvalidChildNormalization} {
		c := nativeOutcomeCase(t, id)
		cleaned := 0
		queries := 0
		t.Run(c.name, func(t *testing.T) {
			instance := Instance{NativeProfileOutcome: nativeOutcomeEvidence(id), Cleanup: func() { cleaned++ }}
			registerInstanceCleanup(t, instance)
			runNativeProfileOutcome(t, func(t *testing.T, fixture Fixture) Instance {
				queries++
				if fixture.NativeProfileCase != 0 || !reflect.DeepEqual(fixture.Rows[0].Feature.Geometry, geom.Collection{geom.PointZ{0, 0, 0}, geom.MultiPointZ{}}) {
					t.Fatal("normalized fixture not separate literal valid source")
				}
				valid := normalizedNativeCase(c)
				valid.fixture = fixture
				result := dimensionalControl(valid, false, false)
				result.Cleanup = func() { cleaned++ }
				return result
			}, instance, c, nativeOutcomeProfile())
		})
		expected := 1
		if id == NativeInvalidChildNormalization {
			expected = 2
		}
		if cleaned != expected || queries != expected-1 {
			t.Fatalf("cleanup=%d queries=%d", cleaned, queries)
		}
	}
}
func TestNativeProfileEvidenceNegativeControls(t *testing.T) {
	for _, id := range []NativeProfileCase{NativeMixedCollection, NativeInvalidChildNormalization} {
		c := nativeOutcomeCase(t, id)
		controls := []struct {
			name   string
			change func(*NativeProfileEvidence)
		}{
			{"wrong case", func(e *NativeProfileEvidence) { e.Case = 0 }},
			{"wrong backend", func(e *NativeProfileEvidence) { e.Backend = "postgis" }},
			{"wrong version", func(e *NativeProfileEvidence) { e.Version += "x" }},
			{"wrong operation", func(e *NativeProfileEvidence) { e.Operation = NativeGeometryInsertion }},
			{"wrong kind", func(e *NativeProfileEvidence) { e.Kind = 0 }},
			{"missing attempted wire", func(e *NativeProfileEvidence) { e.AttemptedWire = nil }},
			{"mutated attempted wire", func(e *NativeProfileEvidence) { e.AttemptedWire[len(e.AttemptedWire)-1] ^= 1 }},
			{"context cancellation", func(e *NativeProfileEvidence) { e.Error = context.Canceled }},
			{"deadline", func(e *NativeProfileEvidence) { e.Error = context.DeadlineExceeded }},
			{"permission", func(e *NativeProfileEvidence) { e.Error = fs.ErrPermission }},
			{"network", func(e *NativeProfileEvidence) { e.Error = &net.DNSError{Err: "reference failure"} }},
			{"wrong code", func(e *NativeProfileEvidence) { e.VendorCode = 1045 }},
			{"wrong state", func(e *NativeProfileEvidence) { e.SQLState = "28000" }},
		}
		if id == NativeMixedCollection {
			controls = append(controls, struct {
				name   string
				change func(*NativeProfileEvidence)
			}{"unexpected export", func(e *NativeProfileEvidence) { e.ExportedWire = literalWire(normalizedChildNativeWire) }})
		} else {
			controls = append(controls,
				struct {
					name   string
					change func(*NativeProfileEvidence)
				}{"missing export", func(e *NativeProfileEvidence) { e.ExportedWire = nil }},
				struct {
					name   string
					change func(*NativeProfileEvidence)
				}{"changed sibling coordinate", func(e *NativeProfileEvidence) { e.ExportedWire[14] = 1 }},
				struct {
					name   string
					change func(*NativeProfileEvidence)
				}{"changed empty family", func(e *NativeProfileEvidence) { e.ExportedWire[len(e.ExportedWire)-8] = 0xeb }},
				struct {
					name   string
					change func(*NativeProfileEvidence)
				}{"trailing wire", func(e *NativeProfileEvidence) { e.ExportedWire = append(e.ExportedWire, 0) }},
			)
		}
		for _, control := range controls {
			t.Run(c.name+"/"+control.name, func(t *testing.T) {
				e := nativeOutcomeEvidence(id)
				control.change(e)
				cleaned := false
				t.Run("validator cleanup", func(t *testing.T) {
					instance := Instance{NativeProfileOutcome: e, Cleanup: func() { cleaned = true }}
					registerInstanceCleanup(t, instance)
					if len(checkNativeProfileEvidence(instance, c, nativeOutcomeProfile())) == 0 {
						t.Fatal("false evidence accepted")
					}
				})
				if !cleaned {
					t.Fatal("bad evidence cleanup absent")
				}
			})
		}
		instance := Instance{NativeProfileOutcome: nativeOutcomeEvidence(id)}
		if len(checkNativeProfileEvidence(instance, c, nil)) == 0 {
			t.Fatal("default profile accepted unsolicited evidence")
		}
		if issues, handled := checkSetup(instance, c); !handled || len(issues) == 0 {
			t.Fatal("ordinary runner accepted native evidence")
		}
		unrelated := c
		unrelated.name = "XYZ point preserved"
		if len(checkNativeProfileEvidence(instance, unrelated, nativeOutcomeProfile())) == 0 {
			t.Fatal("unrelated case accepted")
		}
		wrong := c
		wrong.fixture = cloneFixture(c.fixture)
		wrong.fixture.Rows[0].Feature.ID = 99
		if len(checkNativeProfileEvidence(instance, wrong, nativeOutcomeProfile())) == 0 {
			t.Fatal("wrong expected identity accepted")
		}
		wrongQuery := c
		wrongQuery.query = cloneQuery(c.query)
		wrongQuery.query.Limit = 999
		if len(checkNativeProfileEvidence(instance, wrongQuery, nativeOutcomeProfile())) == 0 {
			t.Fatal("unrelated query accepted")
		}
		instance.SetupError = errors.New("fixture failure")
		if len(checkNativeProfileEvidence(instance, c, nativeOutcomeProfile())) == 0 {
			t.Fatal("setup failure replaced")
		}
	}
}
func TestNativeProfileDetachedOptionsAndEvidence(t *testing.T) {
	p := nativeOutcomeProfile()
	copy, err := copyNativeProfileOptions(p)
	if err != nil {
		t.Fatal(err)
	}
	p.Cases[0] = 0
	if copy.Cases[0] != NativeMixedCollection {
		t.Fatal("retained options backing array")
	}
	e := nativeOutcomeEvidence(NativeInvalidChildNormalization)
	snapshot := copyNativeEvidence(e)
	e.AttemptedWire[0] = 0
	e.ExportedWire[0] = 0
	if snapshot.AttemptedWire[0] != 1 || snapshot.ExportedWire[0] != 1 {
		t.Fatal("retained evidence wire backing array")
	}
	for _, p := range []*NativeProfileOptions{{Backend: "hana", Version: "other", Cases: []NativeProfileCase{NativeMixedCollection}}, {Backend: "hana", Version: hanaNativeVersion, Cases: []NativeProfileCase{NativeMixedCollection, NativeMixedCollection}}, {Backend: "hana", Version: hanaNativeVersion, Cases: []NativeProfileCase{99}}} {
		if _, err := copyNativeProfileOptions(p); err == nil {
			t.Fatal("unreviewed profile accepted")
		}
	}
	c := nativeOutcomeCase(t, NativeInvalidChildNormalization)
	valid := normalizedNativeCase(c)
	bad := dimensionalControl(valid, false, false)
	bad.Querier = brokenQuerier{run: func(_ context.Context, _ provider.FeatureQuery, callback func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		f := cloneFeature(valid.fixture.Rows[0].Feature)
		f.Geometry = geom.Collection{geom.PointZ{99, 0, 0}, geom.MultiPointZ{}}
		err := callback(&f)
		n := uint64(1)
		return provider.FeatureQueryResult{NumberReturned: 1, NumberMatched: &n}, err
	}}
	if len(checkCase(bad, valid)) == 0 {
		t.Fatal("wrong normalized query sibling accepted")
	}
	if !c.malformed || !c.sourceData {
		t.Fatal("original invalid source oracle rewritten")
	}
}

func TestNativeProfileCannotHideBehindCorruptionEvidence(t *testing.T) {
	c := nativeStorageCase(t)
	instance := Instance{StorageRejectedNativeCorruption: referenceNativeEvidence(), NativeProfileOutcome: nativeOutcomeEvidence(NativeMixedCollection)}
	if len(checkNativeStorageEvidence(instance, c)) == 0 {
		t.Fatal("legacy corruption accepted new unsolicited evidence")
	}
	instance.NativeProfileOutcome = nil
	c.fixture.NativeProfileCase = NativeMixedCollection
	if len(checkNativeStorageEvidence(instance, c)) == 0 {
		t.Fatal("legacy corruption accepted unrelated profile descriptor")
	}
	c.fixture.NativeProfileCase = 0
	if issues := checkNativeStorageEvidence(instance, c); len(issues) != 0 {
		t.Fatal("unchanged native corruption rejected", issues)
	}
}
