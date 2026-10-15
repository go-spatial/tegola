//go:build cgo

package gpkg

import (
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider"
)

func TestMOSBoundsPruneEverySupportedHeader(t *testing.T) {
	p, db := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,minx REAL,maxx REAL,miny REAL,maxy REAL)", map[string]interface{}{"geometry_format": "mos", "mos_units": "m", "mos_precision": 0})
	p.layers["items"].boundFieldnames = &[4]string{"minx", "maxx", "miny", "maxy"}
	geometries := []geom.Geometry{
		geom.Polygon{{{20, 20}, {21, 20}, {21, 21}, {20, 20}}},
		geom.LineString{{20, 20}, {21, 21}},
		geom.Point{20, 20},
	}
	for i := 0; i < 5; i++ {
		g := geometries[2]
		if i < 3 {
			g = geometries[i]
		}
		blob, err := mos.Encode(g, mos.Options{})
		if err != nil {
			t.Fatal(err)
		}
		blob[0] = byte(i) // Text/image also use their point anchor.
		if _, err := db.Exec("INSERT INTO items VALUES(?,?,20,21,20,21)", i+1, blob); err != nil {
			t.Fatal(err)
		}
	}
	query := provider.FeatureQuery{Limit: 10, BoundsSRID: 4326, Bounds: []geom.Extent{{0, 0, 1, 1}}}
	where, args, _, err := featureCandidatePredicate(p.layers["items"], query)
	if err != nil {
		t.Fatal(err)
	}
	var candidates int
	if err := db.QueryRow("SELECT count(*) FROM items l WHERE "+where, args...).Scan(&candidates); err != nil {
		t.Fatal(err)
	}
	if candidates != 0 {
		t.Fatalf("out-of-bounds valid MOS headers admitted %d candidates: %s args=%v", candidates, where, args)
	}
	// NULL and unsupported headers must still reach strict decoding.
	if _, err := db.Exec("INSERT INTO items VALUES(6,NULL,20,21,20,21),(7,zeroblob(12),20,21,20,21),(8,X'FF0000000100010000000000',20,21,20,21)"); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT count(*) FROM items l WHERE "+where, args...).Scan(&candidates); err != nil || candidates != 3 {
		t.Fatalf("conservative malformed/absent candidates=%d err=%v", candidates, err)
	}
}
