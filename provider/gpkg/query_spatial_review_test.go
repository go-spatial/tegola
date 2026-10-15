//go:build cgo

package gpkg

import (
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
)

func TestReviewSpatialExplicitWhitespaceVerticalRejected(t *testing.T) {
	for _, vertical := range []string{" ", "\t\n"} {
		p := &Provider{}
		layer := &Layer{geometryFormat: GeometryFormatWKT, srid: 4326}
		err := p.registerSpatial(layer, dict.Dict{"spatial_dimension": "xyz", "vertical_crs": vertical})
		var invalid provider.InvalidFeatureQueryError
		if !errors.As(err, &invalid) || invalid.Field != "vertical_crs" {
			t.Fatalf("blank explicit vertical reference must fail registration: %v (capability %v)", err, layer.FeatureQuerySupported())
		}
	}
}

func TestReviewSpatialNaNPointBigEndianAbsence(t *testing.T) {
	p, db := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,minx REAL,maxx REAL,miny REAL,maxy REAL)", map[string]interface{}{"geometry_format": "wkb"})
	for i, pair := range [][2]uint64{{0x7ff0000000000001, 0xfff0000000000001}, {0x7fffffffffffffff, 0xfff8000000000000}, {0xfff8000000000000, 0x7ff0000000000100}} {
		body := binary.BigEndian.AppendUint32([]byte{0}, 1)
		body = binary.BigEndian.AppendUint64(body, pair[0])
		body = binary.BigEndian.AppendUint64(body, pair[1])
		if !math.IsNaN(math.Float64frombits(pair[0])) || !math.IsNaN(math.Float64frombits(pair[1])) {
			t.Fatal("oracle requires NaN tuples")
		}
		if _, err := db.Exec("INSERT INTO items VALUES(?,?,999,999,999,999)", i+1, body); err != nil {
			t.Fatal(err)
		}
	}
	ids, result := queryIDs(t, p, provider.FeatureQuery{Limit: 10, Bounds3D: []provider.Extent3D{{0, 0, 100, 1, 1, 101}}, BoundsSRID: 4326, BoundsVerticalCRS: provider.CRS84h})
	if !reflect.DeepEqual(ids, []uint64{1, 2, 3}) || result.NumberMatched == nil || *result.NumberMatched != 3 {
		t.Fatalf("lost absent NaN points with misleading bounds: %v %+v", ids, result)
	}
	// Ordinary XY geometry selected by the same six-value query has no
	// fabricated height and uses the approved vertical-unconstrained policy.
	if _, err := db.Exec("INSERT INTO items VALUES(4,?,0,0,0,0)", rawPointBody(false, [3]float64{0, 0, 0})); err != nil {
		t.Fatal(err)
	}
	ids, _ = queryIDs(t, p, provider.FeatureQuery{Limit: 10, Bounds: []geom.Extent{{0, 0, 1, 1}}, BoundsSRID: 4326})
	if !reflect.DeepEqual(ids, []uint64{1, 2, 3, 4}) {
		t.Fatal(ids)
	}
}
