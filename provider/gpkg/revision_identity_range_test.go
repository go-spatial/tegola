//go:build cgo

package gpkg

import (
	"context"
	"math"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

func TestCurrentRevisionOutsideSQLiteIdentityRange(t *testing.T) {
	p, _ := queryTestProvider(t, `CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT);
 INSERT INTO items VALUES(9223372036854775807,'POINT (1 2)');
 CREATE TABLE tegola_revisions(collection TEXT,feature_id INTEGER,revision INTEGER,incarnation INTEGER);
 INSERT INTO tegola_revisions VALUES('items',9223372036854775807,7,2),('items',-2,99,99);`, nil)
	reader := p.MutationWriter().(provider.RevisionReader)
	for _, id := range []uint64{0, math.MaxInt64, math.MaxInt64 + 1, math.MaxUint64 - 1, math.MaxUint64} {
		want := "0.0"
		if id == math.MaxInt64 {
			want = "2.7"
		}
		got, err := reader.CurrentRevision(context.Background(), "items", id)
		if err != nil || got != want {
			t.Errorf("id=%d revision=%q want=%q err=%v", id, got, want, err)
		}
	}
	ids, _ := queryIDs(t, p, provider.FeatureQuery{IDs: []uint64{math.MaxUint64 - 1}, Limit: 1})
	if len(ids) != 0 {
		t.Fatalf("oversized ID matched %v", ids)
	}
	ids, _ = queryIDs(t, p, provider.FeatureQuery{IDs: []uint64{math.MaxUint64 - 1, math.MaxInt64}, Limit: 2})
	if len(ids) != 1 || ids[0] != math.MaxInt64 {
		t.Fatalf("mixed IDs matched %v", ids)
	}
}
