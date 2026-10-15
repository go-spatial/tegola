package geometrycodec

import (
	"encoding/binary"
	"math"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider/crsconfig"
)

func TestMOSStorageEncodedGrid(t *testing.T) {
	for _, tc := range []struct {
		name    string
		point   geom.Point
		opts    mos.Options
		raw     [2]int32
		decoded geom.Point
	}{
		{"negative-half", geom.Point{-1.5, 2.5}, mos.Options{UnitFactor: 1}, [2]int32{-2, 3}, geom.Point{-2, 3}},
		{"offsets", geom.Point{19, -41}, mos.Options{UnitFactor: 2, OffsetX: 10, OffsetY: -20}, [2]int32{-1, -1}, geom.Point{18, -42}},
		{"mm0", geom.Point{1.234, -2.345}, mos.Options{UnitFactor: .001}, [2]int32{1234, -2345}, geom.Point{1.234, -2.345}},
		{"cm0", geom.Point{1.23, -2.34}, mos.Options{UnitFactor: .01}, [2]int32{123, -234}, geom.Point{1.23, -2.34}},
		{"m2", geom.Point{1.23, -2.34}, mos.Options{UnitFactor: 1, Precision: 2}, [2]int32{123, -234}, geom.Point{1.23, -2.34}},
		{"precision12-cap", geom.Point{.0000000123, -.0000000234}, mos.Options{UnitFactor: 1, Precision: 12}, [2]int32{123, -234}, geom.Point{.0000000123, -.0000000234}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EncodeMOSStorage(tc.point, tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			wantBounds := [4]int32{tc.raw[0], tc.raw[0], tc.raw[1], tc.raw[1]}
			if got.RawBounds != wantBounds {
				t.Fatalf("bounds=%v want %v", got.RawBounds, wantBounds)
			}
			// Independently inspect the MOS point payload, not the decoder-derived bounds.
			if len(got.Blob) != 24 {
				t.Fatalf("point blob length=%d", len(got.Blob))
			}
			raw := [2]int32{int32(binary.LittleEndian.Uint32(got.Blob[16:20])), int32(binary.LittleEndian.Uint32(got.Blob[20:24]))}
			if raw != tc.raw {
				t.Fatalf("persisted grid=%v want %v", raw, tc.raw)
			}
			p, ok := got.Geometry.(geom.Point)
			if !ok {
				t.Fatalf("decoded type %T", got.Geometry)
			}
			for i := range p {
				if math.Abs(p[i]-tc.decoded[i]) > 1e-12 {
					t.Fatalf("decoded=%v want %v", p, tc.decoded)
				}
			}
		})
	}
}

func TestMOSStorageRejectsLossyGeometry(t *testing.T) {
	for _, tc := range []struct {
		name string
		g    geom.Geometry
		opts mos.Options
	}{
		{"nil", nil, mos.Options{}},
		{"empty-points", geom.MultiPoint{}, mos.Options{}},
		{"empty-line", geom.LineString{}, mos.Options{}},
		{"empty-polygon", geom.Polygon{}, mos.Options{}},
		{"z", geom.PointZ{1, 2, 3}, mos.Options{}},
		{"collapsed-line", geom.LineString{{.1, .1}, {.2, .2}}, mos.Options{UnitFactor: 1}},
		{"collapsed-child-line", geom.MultiLineString{{{1, 1}, {2, 2}}, {{.1, .1}, {.2, .2}}}, mos.Options{UnitFactor: 1}},
		{"collapsed-ring", geom.Polygon{{{.1, .1}, {.2, .1}, {.2, .2}, {.1, .1}}}, mos.Options{UnitFactor: 1}},
		{"collapsed-hole", geom.Polygon{
			{{-2, -2}, {2, -2}, {2, 2}, {-2, 2}, {-2, -2}},
			{{.1, .1}, {.1, .2}, {.2, .2}, {.2, .1}, {.1, .1}},
		}, mos.Options{UnitFactor: 1}},
		{"overflow", geom.Point{math.MaxInt32 + 1, 0}, mos.Options{UnitFactor: 1}},
		{"underflow", geom.Point{math.MinInt32 - 1, 0}, mos.Options{UnitFactor: 1}},
		{"nan-coordinate", geom.Point{math.NaN(), 0}, mos.Options{}},
		{"infinite-coordinate", geom.Point{0, math.Inf(1)}, mos.Options{}},
		{"negative-factor", geom.Point{1, 2}, mos.Options{UnitFactor: -1}},
		{"nan-factor", geom.Point{1, 2}, mos.Options{UnitFactor: math.NaN()}},
		{"infinite-factor", geom.Point{1, 2}, mos.Options{UnitFactor: math.Inf(1)}},
		{"nan-offset", geom.Point{1, 2}, mos.Options{OffsetX: math.NaN()}},
		{"infinite-offset", geom.Point{1, 2}, mos.Options{OffsetY: math.Inf(-1)}},
		{"nan-precision", geom.Point{1, 2}, mos.Options{Precision: math.NaN()}},
		{"infinite-precision", geom.Point{1, 2}, mos.Options{Precision: math.Inf(1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := EncodeMOSStorage(tc.g, tc.opts); err == nil {
				t.Fatal("invalid storage geometry accepted")
			}
		})
	}
}

func TestMOSStorageSmallRingAtInt32Edge(t *testing.T) {
	const m = float64(math.MaxInt32)
	polygon := geom.Polygon{{{m - 1, m - 1}, {m, m - 1}, {m, m}, {m - 1, m}, {m - 1, m - 1}}}
	got, err := EncodeMOSStorage(polygon, mos.Options{UnitFactor: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got.RawBounds != [4]int32{math.MaxInt32 - 1, math.MaxInt32, math.MaxInt32 - 1, math.MaxInt32} {
		t.Fatalf("bounds=%v", got.RawBounds)
	}
	if !reflect.DeepEqual(got.Geometry, polygon) {
		t.Fatalf("geometry changed: %#v", got.Geometry)
	}
}

func TestMOSStorageBoundsColumnOrder(t *testing.T) {
	got, err := EncodeMOSStorage(geom.LineString{{-2.5, 7.5}, {4.5, -6.5}}, mos.Options{UnitFactor: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got.RawBounds != [4]int32{-3, 5, -7, 8} {
		t.Fatalf("bounds must use minX,maxX,minY,maxY: %v", got.RawBounds)
	}
}

func TestMOSStorageRawScale(t *testing.T) {
	for _, tc := range []struct{ precision, factor, want float64 }{{0, .001, 1000}, {0, .01, 100}, {2, 1, 100}, {12, 1, 1e10}} {
		got, err := MOSRawScale(MOSConfig{Precision: tc.precision, UnitFactor: tc.factor})
		if err != nil || got != tc.want {
			t.Fatalf("scale(%v,%v)=%v,%v want %v", tc.precision, tc.factor, got, err, tc.want)
		}
	}
	for _, cfg := range []MOSConfig{{UnitFactor: 0}, {UnitFactor: -1}, {UnitFactor: math.NaN()}, {UnitFactor: math.Inf(1)}, {UnitFactor: 1, Precision: math.NaN()}, {UnitFactor: 1, Precision: math.Inf(1)}} {
		if _, err := MOSRawScale(cfg); err == nil {
			t.Fatalf("invalid scale accepted: %+v", cfg)
		}
	}
}

func TestTransformMOSStorageGeometry(t *testing.T) {
	definition, ok := crsconfig.CanonicalFeatureDefinition(3857)
	if !ok {
		t.Fatal("missing canonical Mercator")
	}
	mercator, err := crsconfig.NewFeatureProjection(definition)
	if err != nil {
		t.Fatal(err)
	}
	got, err := TransformStorageGeometry(geom.Point{10, 0}, 4326, 3857, mercator)
	if err != nil {
		t.Fatal(err)
	}
	point := got.(geom.Point)
	if math.Abs(point[0]-1113194.9079327357) > 1e-6 || math.Abs(point[1]) > 1e-6 {
		t.Fatalf("Mercator point=%v", point)
	}
	geographic, err := crsconfig.NewFeatureProjection("+proj=longlat +datum=WGS84")
	if err != nil {
		t.Fatal(err)
	}
	got, err = TransformStorageGeometry(point, 3857, 4326, geographic)
	if err != nil {
		t.Fatal(err)
	}
	point = got.(geom.Point)
	if math.Abs(point[0]-10) > 1e-9 || math.Abs(point[1]) > 1e-9 {
		t.Fatalf("geographic point=%v", point)
	}
	custom, err := crsconfig.NewFeatureProjection("+proj=etmerc +lat_0=50 +lon_0=30 +k=1 +x_0=1000 +y_0=2000 +datum=WGS84 +units=m")
	if err != nil {
		t.Fatal(err)
	}
	got, err = TransformStorageGeometry(geom.Point{30, 50}, 4326, 990001, custom)
	if err != nil {
		t.Fatal(err)
	}
	point = got.(geom.Point)
	if math.Abs(point[0]-1000) > 1e-6 || math.Abs(point[1]-2000) > 1e-6 {
		t.Fatalf("custom projection origin=%v", point)
	}
	for _, srid := range []uint64{0, 990001} {
		input := geom.Point{123, 456}
		got, err := TransformStorageGeometry(input, srid, 990001, nil)
		if err != nil || !reflect.DeepEqual(got, input) {
			t.Fatalf("storage identity=%v,%v", got, err)
		}
	}
	if _, err := TransformStorageGeometry(geom.Point{1, 2}, 123456, 990001, custom); err == nil {
		t.Fatal("unsupported request CRS accepted")
	}
	if _, err := TransformStorageGeometry(geom.Point{1, 2}, 4326, 990001, nil); err == nil {
		t.Fatal("unpinned target accepted")
	}
	if _, err := TransformStorageGeometry(geom.PointZ{1, 2, 3}, 0, 990001, custom); err == nil {
		t.Fatal("unsupported XYZ input accepted")
	}
}
