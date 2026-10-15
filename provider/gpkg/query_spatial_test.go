//go:build cgo

package gpkg

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
)

func TestSpatialQueryMixedExactPaging(t *testing.T) {
	p, _ := queryTestProvider(t, `CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT);
INSERT INTO items VALUES(1,'LINESTRING Z (0 0 0,10 0 10)'),(2,'POINT Z (1 0 9)'),(3,NULL),(4,'POINT (1 0)');`, map[string]interface{}{"spatial_dimension": "mixed_xy_xyz", "vertical_crs": provider.CRS84h})
	q := provider.FeatureQuery{Limit: 10, Bounds3D: []provider.Extent3D{{0, -1, 8, 2, 1, 10}}, BoundsSRID: 4326, BoundsVerticalCRS: provider.CRS84h}
	ids, result := queryIDs(t, p, q)
	if !reflect.DeepEqual(ids, []uint64{2, 3, 4}) || result.NumberMatched == nil || *result.NumberMatched != 3 {
		t.Fatalf("exact IDs %v result%+v", ids, result)
	}
	q.Offset, q.Limit = 1, 1
	ids, result = queryIDs(t, p, q)
	if !reflect.DeepEqual(ids, []uint64{3}) || !result.HasMore || result.NumberMatched != nil {
		t.Fatalf("exact page%v result%+v", ids, result)
	}
	var payload geom.Geometry
	_, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{IDs: []uint64{2}, Limit: 1}, func(f *provider.Feature) error { payload = f.Geometry; return nil })
	if err != nil || !reflect.DeepEqual(payload, geom.PointZ{1, 0, 9}) {
		t.Fatalf("lost height %v %v", payload, err)
	}
	meta, err := p.layers["items"].SpatialMetadata()
	if err != nil || meta.Dimension != provider.DimensionMixedXYXYZ {
		t.Fatalf("metadata%+v %v", meta, err)
	}
	meta.Dimension = provider.DimensionXY
	if p.layers["items"].spatialMetadata.Dimension != provider.DimensionMixedXYXYZ {
		t.Fatal("metadata mutation")
	}
}

func TestSpatialQueryRawRegistrationWithoutGeometryType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "xyz.sqlite")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := db.Exec("CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT); INSERT INTO items VALUES(1,'POINT Z (1 2 3)')"); err != nil {
		t.Fatal(err)
	}
	tiler, err := NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]interface{}{{"name": "items", "tablename": "items", "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "srid": 4326, "spatial_dimension": "xyz", "vertical_crs": provider.CRS84h}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := tiler.(*Provider)
	defer func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	}()
	ids, _ := queryIDs(t, p, provider.FeatureQuery{Limit: 10, Bounds3D: []provider.Extent3D{{1, 2, 3, 1, 2, 3}}, BoundsSRID: 4326, BoundsVerticalCRS: provider.CRS84h})
	if !reflect.DeepEqual(ids, []uint64{1}) {
		t.Fatal(ids)
	}
}

func rawPointBody(z bool, p [3]float64) []byte {
	kind := uint32(1)
	n := 2
	if z {
		kind = 1001
		n = 3
	}
	b := binary.LittleEndian.AppendUint32([]byte{1}, kind)
	for i := 0; i < n; i++ {
		b = binary.LittleEndian.AppendUint64(b, math.Float64bits(p[i]))
	}
	return b
}

func TestSpatialNativeEnvelopeSchemaPolicy(t *testing.T) {
	layer := &Layer{geometryFormat: GeometryFormatGPKG, spatialMetadata: provider.SpatialMetadata{Dimension: provider.DimensionXYZ, VerticalCRS: provider.CRS84h}, nativeZ: 1}
	for _, flag := range []byte{1, 3, 5} {
		header := binary.LittleEndian.AppendUint32([]byte{'G', 'P', 0, flag}, 4326)
		for i := 0; i < int(headerFlags(flag).Envelope().NumberOfElements()); i++ {
			header = binary.LittleEndian.AppendUint64(header, math.Float64bits(float64(i/2)))
		}
		g, err := decodeRawGeometryValue(append(header, rawPointBody(true, [3]float64{1, 2, 3})...), layer)
		if err != nil || !reflect.DeepEqual(g, geom.PointZ{1, 2, 3}) {
			t.Fatalf("envelope flag%d: %v %v", flag, g, err)
		}
	}
	for _, tc := range []struct {
		flag  byte
		z     bool
		point [3]float64
	}{
		{5, false, [3]float64{1, 2, 0}}, {7, true, [3]float64{1, 2, 3}}, {9, true, [3]float64{1, 2, 3}},
		{17, true, [3]float64{1, 2, 3}}, {1, true, [3]float64{math.NaN(), math.NaN(), math.NaN()}},
	} {
		header := binary.LittleEndian.AppendUint32([]byte{'G', 'P', 0, tc.flag}, 4326)
		for i := 0; i < int(headerFlags(tc.flag).Envelope().NumberOfElements()); i++ {
			header = binary.LittleEndian.AppendUint64(header, 0)
		}
		if _, err := decodeRawGeometryValue(append(header, rawPointBody(tc.z, tc.point)...), layer); err == nil {
			t.Fatalf("accepted conflicting flag%d", tc.flag)
		}
	}
	empty := binary.LittleEndian.AppendUint32([]byte{'G', 'P', 0, 17}, 4326)
	if g, err := decodeRawGeometryValue(append(empty, rawPointBody(true, [3]float64{math.NaN(), math.NaN(), math.NaN()})...), layer); g != nil || err != nil {
		t.Fatalf("empty XYZ %v %v", g, err)
	}
}

func TestSpatialQueryGuards(t *testing.T) {
	p, _ := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT); INSERT INTO items VALUES(1,'POINT Z (0 0 1)')", map[string]interface{}{})
	if _, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { return nil }); err == nil {
		t.Fatal("XY declaration silently accepted XYZ")
	}
	p2, _ := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT); INSERT INTO items VALUES(1,'POINT Z (0 0 1)')", map[string]interface{}{"spatial_dimension": "xyz", "vertical_crs": provider.CRS84h})
	if err := p2.db.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := p2.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 1, Bounds3D: []provider.Extent3D{{0, 0, 0, 1, 1, 1}}, BoundsSRID: 27700, BoundsVerticalCRS: provider.CRS84h}, func(*provider.Feature) error { return nil })
	if !errors.Is(err, provider.ErrUnsupported) {
		t.Fatalf("unsupported CRS must fail before I/O: %v", err)
	}
	p3, _ := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT)", map[string]interface{}{"spatial_dimension": "xyz", "vertical_crs": provider.CRS84h, "crs_defn": "+proj=longlat +datum=WGS84"})
	if !errors.Is(p3.layers["items"].FeatureQuerySupported(), provider.ErrUnsupported) {
		t.Fatal("custom source definition admitted to canonical height profile")
	}
}

func TestRawNaNPointAbsenceCandidate(t *testing.T) {
	p, db := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,minx REAL,maxx REAL,miny REAL,maxy REAL)", map[string]interface{}{"geometry_format": "wkb"})
	for _, bits := range []uint64{0x7ff8000000000000, 0x7ff0000000000001, 0xfff0000000000001} {
		b := rawPointBody(false, [3]float64{math.Float64frombits(bits), math.Float64frombits(bits), 0})
		if _, err := db.Exec("INSERT INTO items VALUES(?, ?, 999,999,999,999)", int64(bits&0xffff)+int64(bits>>63)+1, b); err != nil {
			t.Fatal(err)
		}
	}
	ids, _ := queryIDs(t, p, provider.FeatureQuery{Limit: 10, Bounds: []geom.Extent{{0, 0, 1, 1}}, BoundsSRID: 4326})
	if len(ids) != 3 {
		t.Fatalf("lost NaN absence with misleading bounds: %v", ids)
	}
}

func TestSpatialRegistrationSchemaAndExplicitShape(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		z, m, srid           int
		conf                 dict.Dict
		invalid, unsupported bool
	}{
		{"inferred height lacks binding", 1, 0, 4326, dict.Dict{}, false, true},
		{"explicit height lacks binding", 1, 0, 4326, dict.Dict{"spatial_dimension": "xyz"}, true, false},
		{"schema override disagrees", 1, 0, 4326, dict.Dict{"spatial_dimension": "xy"}, true, false},
		{"optional measures no actual M", 1, 2, 4326, dict.Dict{"vertical_crs": provider.CRS84h}, false, false},
		{"mandatory measures", 1, 1, 4326, dict.Dict{"vertical_crs": provider.CRS84h}, false, true},
		{"invalid Z schema", 3, 0, 4326, dict.Dict{}, false, true},
		{"optional Z mixed", 2, 0, 4326, dict.Dict{"vertical_crs": provider.CRS84h}, false, false},
		{"unknown horizontal source", 1, 0, 0, dict.Dict{"vertical_crs": provider.CRS84h}, false, true},
		{"unsupported vertical reference", 1, 0, 4326, dict.Dict{"vertical_crs": "urn:example:orthometric"}, false, true},
		{"empty explicit vertical", 1, 0, 4326, dict.Dict{"vertical_crs": ""}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "schema.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := db.Close(); err != nil {
					t.Error(err)
				}
			}()
			if _, err := db.Exec("CREATE TABLE gpkg_geometry_columns(table_name TEXT,column_name TEXT,z INTEGER,m INTEGER,srs_id INTEGER); INSERT INTO gpkg_geometry_columns VALUES('items','geom',?,?,?)", tc.z, tc.m, tc.srid); err != nil {
				t.Fatal(err)
			}
			p := &Provider{db: db}
			l := &Layer{tablename: "items", geomFieldname: "geom", srid: 4326}
			err = p.registerSpatial(l, tc.conf)
			var invalid provider.InvalidFeatureQueryError
			if errors.As(err, &invalid) != tc.invalid {
				t.Fatalf("invalid shape %v", err)
			}
			if !tc.invalid && err != nil {
				t.Fatal(err)
			}
			if !tc.invalid && errors.Is(l.FeatureQuerySupported(), provider.ErrUnsupported) != tc.unsupported {
				t.Fatalf("capability %v", l.FeatureQuerySupported())
			}
		})
	}
}

func TestSpatialCanonicalProjectionAndNonplanarity(t *testing.T) {
	p, _ := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT); INSERT INTO items VALUES(1,'POINT Z (0 0 55)')", map[string]interface{}{"srid": 3857, "spatial_dimension": "xyz", "vertical_crs": provider.CRS84h})
	ids, _ := queryIDs(t, p, provider.FeatureQuery{Limit: 10, Bounds3D: []provider.Extent3D{{0, 0, 55, 0, 0, 55}}, BoundsSRID: 4326, BoundsVerticalCRS: provider.CRS84h})
	if !reflect.DeepEqual(ids, []uint64{1}) {
		t.Fatal(ids)
	}
	p2, _ := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT); INSERT INTO items VALUES(1,'POLYGON Z ((0 0 0,1 0 1,1 1 2,0 0.5 0.5,0 0 0))')", map[string]interface{}{"spatial_dimension": "xyz", "vertical_crs": provider.CRS84h})
	callbacks := 0
	_, err := p2.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10, Bounds3D: []provider.Extent3D{{-1, -1, -1, 200000, 200000, 10}}, BoundsSRID: 3857, BoundsVerticalCRS: provider.CRS84h}, func(*provider.Feature) error { callbacks++; return nil })
	if !errors.Is(err, provider.ErrUnsupported) || callbacks != 0 {
		t.Fatalf("transformed nonplanar surface %v callbacks%d", err, callbacks)
	}
}
