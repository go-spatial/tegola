package basic

import (
	"strings"
	"testing"
)

func TestEffectiveProj4DefinitionSnapshot(t *testing.T) {
	for _, srid := range []uint64{4326, 3857, 3395, 4087, 32634, 32760} {
		definition, ok := EffectiveProj4Definition(srid)
		if !ok || !strings.Contains(definition, "+proj=") {
			t.Fatalf("missing shipped definition for %d", srid)
		}
	}
	if _, ok := EffectiveProj4Definition(987654321); ok {
		t.Fatal("unknown definition inferred")
	}
	const testSRID = uint64(987654322)
	proj4RegisteredMu.Lock()
	proj4Registered[testSRID] = "+proj=longlat +datum=WGS84"
	proj4RegisteredMu.Unlock()
	t.Cleanup(func() {
		proj4RegisteredMu.Lock()
		delete(proj4Registered, testSRID)
		proj4RegisteredMu.Unlock()
	})
	snapshot, ok := EffectiveProj4Definition(testSRID)
	if !ok || snapshot != "+proj=longlat +datum=WGS84" {
		t.Fatal("managed registration omitted")
	}
	proj4RegisteredMu.Lock()
	proj4Registered[testSRID] = "changed"
	proj4RegisteredMu.Unlock()
	if snapshot != "+proj=longlat +datum=WGS84" {
		t.Fatal("snapshot retained mutable state")
	}
}
