package mos

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
)

// blobBuilder assembles a MOS blob the same way TMapObjectStructureBase.
// PutToBufInternal does: header, per-subobject counts, quantized points,
// then optional non-geometric tail blocks.
type blobBuilder struct {
	oType         byte
	typeMod       byte
	addFlag       uint16
	ofl           uint16
	counts        []uint32
	points        [][2]int32
	tail          []byte
	tenByteHeader bool
}

func (bb *blobBuilder) build() []byte {
	var buf bytes.Buffer
	buf.WriteByte(bb.oType)
	buf.WriteByte(bb.typeMod)
	_ = binary.Write(&buf, binary.LittleEndian, bb.addFlag)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(len(bb.counts)))
	_ = binary.Write(&buf, binary.LittleEndian, int32(len(bb.points)))
	if !bb.tenByteHeader {
		_ = binary.Write(&buf, binary.LittleEndian, bb.ofl)
	}
	for _, c := range bb.counts {
		_ = binary.Write(&buf, binary.LittleEndian, c)
	}
	for _, p := range bb.points {
		_ = binary.Write(&buf, binary.LittleEndian, int32(p[0]))
		_ = binary.Write(&buf, binary.LittleEndian, int32(p[1]))
	}
	buf.Write(bb.tail)
	return buf.Bytes()
}

func TestDecodePolyline(t *testing.T) {
	// two subobjects: quantized with precision 3 (millimetres)
	bb := &blobBuilder{
		oType:  TypePolyline,
		counts: []uint32{3, 2},
		points: [][2]int32{
			{1000, 2000}, {3000, 4000}, {3000, 4000}, // dup point dropped
			{5000, 6000}, {7000, 8000},
		},
		tail: bytes.Repeat([]byte{0xAA}, 64), // labels/markers/etc, must be ignored
	}
	g, err := Decode(bb.build(), Options{Precision: 3})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := geom.MultiLineString{
		{{1, 2}, {3, 4}},
		{{5, 6}, {7, 8}},
	}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestDecodePolylineSingleSubObject(t *testing.T) {
	bb := &blobBuilder{
		oType:  TypePolyline,
		counts: []uint32{3},
		points: [][2]int32{{0, 0}, {100, 0}, {100, 100}},
	}
	g, err := Decode(bb.build(), Options{Precision: 2})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := geom.LineString{{0, 0}, {1, 0}, {1, 1}}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestDecodePolygonRingClosure(t *testing.T) {
	// ring stored with the final vertex repeating the first one: the reader
	// drops it and the geometry is closed again. A second ring becomes a hole.
	bb := &blobBuilder{
		oType:  TypePolygon,
		counts: []uint32{4, 4},
		points: [][2]int32{
			{0, 0}, {10000, 0}, {10000, 10000}, {0, 10000}, // closed ring as stored
			{2000, 2000}, {4000, 2000}, {4000, 4000}, {2000, 2000}, // closed ring
		},
	}
	g, err := Decode(bb.build(), Options{Precision: 0})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := geom.Polygon{
		{{0, 0}, {10000, 0}, {10000, 10000}, {0, 10000}, {0, 0}},
		{{2000, 2000}, {4000, 2000}, {4000, 4000}, {2000, 2000}},
	}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestDecodePolygonOpenRing(t *testing.T) {
	// ring stored without the repeating final vertex
	bb := &blobBuilder{
		oType:  TypePolygon,
		counts: []uint32{3},
		points: [][2]int32{{0, 0}, {10, 0}, {10, 10}},
	}
	g, err := Decode(bb.build(), Options{Precision: 0})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := geom.Polygon{{{0, 0}, {10, 0}, {10, 10}, {0, 0}}}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestDecodeMultiPolygonTwoIslands(t *testing.T) {
	// two disjoint contours (neither contains the other): both are exterior
	// rings, so ExportToWKB would emit wkbMultiPolygon (GeomType=6) and the
	// decoder must return a MultiPolygon.
	bb := &blobBuilder{
		oType:  TypePolygon,
		counts: []uint32{4, 4},
		points: [][2]int32{
			{0, 0}, {10, 0}, {10, 10}, {0, 0}, // island 1 (closed)
			{20, 0}, {30, 0}, {30, 10}, {20, 0}, // island 2 (closed)
		},
	}
	g, err := Decode(bb.build(), Options{Precision: 0})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := geom.MultiPolygon{
		{{{0, 0}, {10, 0}, {10, 10}, {0, 0}}},
		{{{20, 0}, {30, 0}, {30, 10}, {20, 0}}},
	}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestDecodeMultiPolygonIslandInHole(t *testing.T) {
	// outer square with a hole, and a small island inside the hole:
	// exterior count is 2 -> MultiPolygon; the middle ring is a hole of the
	// first, the island is its own exterior.
	bb := &blobBuilder{
		oType:  TypePolygon,
		counts: []uint32{5, 4, 4},
		points: [][2]int32{
			{0, 0}, {100, 0}, {100, 100}, {0, 100}, {0, 0}, // outer (closed)
			{30, 30}, {70, 30}, {70, 70}, {30, 30}, // hole (closed)
			{45, 45}, {55, 45}, {55, 55}, {45, 45}, // island in hole
		},
	}
	g, err := Decode(bb.build(), Options{Precision: 0})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := geom.MultiPolygon{
		{
			{{0, 0}, {100, 0}, {100, 100}, {0, 100}, {0, 0}},
			{{30, 30}, {70, 30}, {70, 70}, {30, 30}},
		},
		{
			{{45, 45}, {55, 45}, {55, 55}, {45, 45}},
		},
	}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestDecodePolygonDegenerateContourSkipped(t *testing.T) {
	// a 2-point contour is invalid for a polygon (ValidateSubObjectSpatialData
	// requires >= 3) and must be skipped
	bb := &blobBuilder{
		oType:  TypePolygon,
		counts: []uint32{4, 2},
		points: [][2]int32{
			{0, 0}, {10, 0}, {10, 10}, {0, 0},
			{50, 50}, {60, 60}, // degenerate
		},
	}
	g, err := Decode(bb.build(), Options{Precision: 0})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := geom.Polygon{{{0, 0}, {10, 0}, {10, 10}, {0, 0}}}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestDecodePolylineDegenerateContourSkipped(t *testing.T) {
	// a 1-point contour is invalid for a polyline (>= 2 required)
	bb := &blobBuilder{
		oType:  TypePolyline,
		counts: []uint32{2, 1, 3},
		points: [][2]int32{
			{0, 0}, {10, 0},
			{99, 99}, // degenerate
			{20, 20}, {30, 30}, {40, 40},
		},
	}
	g, err := Decode(bb.build(), Options{Precision: 0})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := geom.MultiLineString{
		{{0, 0}, {10, 0}},
		{{20, 20}, {30, 30}, {40, 40}},
	}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestDecodeLineSingleSubObjectIsLineString(t *testing.T) {
	// one contour -> ExportToWKB emits wkbLineString (GeomType=2)
	bb := &blobBuilder{
		oType:  TypePolyline,
		counts: []uint32{4},
		points: [][2]int32{{0, 0}, {100, 0}, {100, 100}, {200, 100}},
	}
	g, err := Decode(bb.build(), Options{Precision: 0})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := geom.LineString{{0, 0}, {100, 0}, {100, 100}, {200, 100}}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestDecodePointSingleIsPointMultiIsMultiPoint(t *testing.T) {
	// one contour -> wkbPoint (GeomType=1), several -> wkbMultiPoint (4)
	single := &blobBuilder{
		oType:  TypePoint,
		counts: []uint32{1},
		points: [][2]int32{{123, 456}},
	}
	g, err := Decode(single.build(), Options{Precision: 2})
	if err != nil {
		t.Fatalf("decode single point: %v", err)
	}
	if want := (geom.Point{1.23, 4.56}); !reflect.DeepEqual(g, want) {
		t.Errorf("single: got %v, want %v", g, want)
	}

	multi := &blobBuilder{
		oType:  TypePoint,
		counts: []uint32{1, 1, 1},
		points: [][2]int32{{100, 200}, {300, 400}, {500, 600}},
	}
	g, err = Decode(multi.build(), Options{Precision: 2})
	if err != nil {
		t.Fatalf("decode multi point: %v", err)
	}
	want := geom.MultiPoint{{1, 2}, {3, 4}, {5, 6}}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("multi: got %v, want %v", g, want)
	}
}

func TestDecodeNativeTenBytePointAndCarrier(t *testing.T) {
	// Native Mappl blobs use the 10-byte geometry prefix. A previous reader
	// consumed the first two bytes of the first coordinate as a flags field,
	// shifted the point counts, and dropped otherwise valid point blobs.
	point := &blobBuilder{
		oType:         TypePoint,
		counts:        []uint32{1},
		points:        [][2]int32{{1234, 2345}},
		tenByteHeader: true,
	}
	g, err := Decode(point.build(), Options{Precision: 0, UnitFactor: 0.001})
	if err != nil {
		t.Fatalf("decode native point: %v", err)
	}
	if want := (geom.Point{1.234, 2.345}); !reflect.DeepEqual(g, want) {
		t.Fatalf("native point: got %v, want %v", g, want)
	}

	// The EGKO query uses a two-point, 1 mm carrier so a source ObjectType=2
	// row can travel through the line-oriented path. The ObjectType column
	// remains a separate feature attribute and is normalized by the client.
	carrier := &blobBuilder{
		oType:         TypePolyline,
		counts:        []uint32{2},
		points:        [][2]int32{{1000, 2000}, {1001, 2001}},
		tenByteHeader: true,
	}
	g, err = Decode(carrier.build(), Options{Precision: 0, UnitFactor: 0.001})
	if err != nil {
		t.Fatalf("decode native carrier: %v", err)
	}
	want := geom.LineString{{1, 2}, {1.001, 2.001}}
	gotLine, ok := g.(geom.LineString)
	if !ok || len(gotLine) != len(want) {
		t.Fatalf("native carrier: got %T %v, want %v", g, g, want)
	}
	for i := range want {
		for axis := 0; axis < 2; axis++ {
			if math.Abs(gotLine[i][axis]-want[i][axis]) > 1e-12 {
				t.Fatalf("native carrier: got %v, want %v", g, want)
			}
		}
	}

	h, err := DecodeHeader(point.build())
	if err != nil {
		t.Fatalf("decode native point header: %v", err)
	}
	if h.ObjectType != TypePoint || h.SubObjectsCount != 1 || h.PointsCount != 1 {
		t.Fatalf("unexpected native point header: %+v", h)
	}
}

func TestDecodeTextAndImageAnchors(t *testing.T) {
	for _, oType := range []byte{TypeText, TypeImage} {
		bb := &blobBuilder{
			oType:  oType,
			counts: []uint32{2},
			points: [][2]int32{{1500, 2500}, {3500, 4500}},
		}
		g, err := Decode(bb.build(), Options{Precision: 2})
		if err != nil {
			t.Fatalf("oType %v: %v", oType, err)
		}
		if want := (geom.Point{15, 25}); !reflect.DeepEqual(g, want) {
			t.Errorf("oType %v: got %v, want %v", oType, g, want)
		}
	}
}

func TestDecodeMetersPrecision2(t *testing.T) {
	// the production test-server profile: metre coordinates quantized with
	// precision 2 (centimetre resolution), CRS is spherical-ish Mercator in
	// metres — the decoder only dequantizes, projection is downstream.
	bb := &blobBuilder{
		oType:  TypePolyline,
		counts: []uint32{3},
		points: [][2]int32{
			{41758231, 7319485},
			{41758500, 7319700},
			{41758800, 7319999},
		},
	}
	g, err := Decode(bb.build(), Options{Precision: 2})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := geom.LineString{
		{417582.31, 73194.85},
		{417585.00, 73197.00},
		{417588.00, 73199.99},
	}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestDecodePointAndText(t *testing.T) {
	// point object with extended (DataVer=1) icon params in the tail
	pointTail := []byte{
		0, 0, 0, 0, // angle
		0, 0, 0, 0, // icon offset x
		0, 0, 0, 0, // icon offset y
		0,    // flMirrorIcon
		1,    // DataVer=1 -> TBlobPointParamExt (58 bytes)
		0, 0, // wFree
	}
	pointTail = append(pointTail, bytes.Repeat([]byte{0}, 58-16)...)

	bb := &blobBuilder{
		oType:  TypePoint,
		counts: []uint32{1, 1},
		points: [][2]int32{{123, 456}, {789, 12}},
		ofl:    FlagFixLabel,
		tail:   pointTail,
	}
	g, err := Decode(bb.build(), Options{Precision: 1})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := geom.MultiPoint{{12.3, 45.6}, {78.9, 1.2}}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}

	// text object: anchor point of the first subobject
	textBlob := (&blobBuilder{
		oType:  TypeText,
		counts: []uint32{2},
		points: [][2]int32{{100, 200}, {300, 400}},
	}).build()
	g, err = Decode(textBlob, Options{Precision: 0})
	if err != nil {
		t.Fatalf("decode text: %v", err)
	}
	if want := (geom.Point{100, 200}); !reflect.DeepEqual(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestDecodeOffsets(t *testing.T) {
	bb := &blobBuilder{
		oType:  TypePoint,
		counts: []uint32{1},
		points: [][2]int32{{100, 200}},
	}
	g, err := Decode(bb.build(), Options{Precision: 0, OffsetX: 5000, OffsetY: -7000})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if want := (geom.Point{5100, -6800}); !reflect.DeepEqual(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestDecodeErrors(t *testing.T) {
	cases := []struct {
		name string
		buf  []byte
		opts Options
	}{
		{"short buffer", []byte{1, 2, 3}, Options{}},
		{"unsupported object type", func() []byte {
			bb := &blobBuilder{oType: 5}
			return bb.build()
		}(), Options{}},
		{"counts mismatch", func() []byte {
			bb := &blobBuilder{oType: TypePolyline, counts: []uint32{3, 2}, points: [][2]int32{{1, 1}, {2, 2}}}
			return bb.build()
		}(), Options{}},
		{"truncated points", func() []byte {
			buf := (&blobBuilder{oType: TypePolyline, counts: []uint32{3}, points: [][2]int32{{1, 1}, {2, 2}, {3, 3}}}).build()
			return buf[:len(buf)-4] // cut the last y
		}(), Options{}},
		// Delphi GetKPrecision clamps: MIN(aPrecision,10) then MAX(...,0).
		// Negative and >10 precisions are clamped, not errors.
		{"fractional precision", (&blobBuilder{oType: TypePoint, counts: []uint32{1}, points: [][2]int32{{1, 1}}}).build(), Options{Precision: 1.5}},
		{"infinite precision", (&blobBuilder{oType: TypePoint, counts: []uint32{1}, points: [][2]int32{{1, 1}}}).build(), Options{Precision: math.Inf(1)}},
	}
	for _, tc := range cases {
		if _, err := Decode(tc.buf, tc.opts); err == nil {
			t.Errorf("%v: expected error, got nil", tc.name)
		}
	}
}

func TestDecodeHeader(t *testing.T) {
	bb := &blobBuilder{
		oType:   TypePolygon,
		typeMod: 7,
		addFlag: 0xBEEF,
		ofl:     FlagFixLabel | FlagBezier,
		counts:  []uint32{2, 3},
		points:  [][2]int32{{1, 1}, {2, 2}, {3, 3}, {4, 4}, {5, 5}},
	}
	h, err := DecodeHeader(bb.build())
	if err != nil {
		t.Fatalf("DecodeHeader: %v", err)
	}
	if h.ObjectType != TypePolygon || h.TypeModification != 7 || h.AddFlag != 0xBEEF {
		t.Errorf("unexpected header fields: %+v", h)
	}
	if h.SubObjectsCount != 2 || h.PointsCount != 5 {
		t.Errorf("unexpected counts: %+v", h)
	}
	if h.Flags != FlagFixLabel|FlagBezier {
		t.Errorf("unexpected flags: %v", h.Flags)
	}

	if _, err := DecodeHeader([]byte{1}); err == nil {
		t.Error("expected error for short header, got nil")
	}
}

func TestKPrecisionClamp(t *testing.T) {
	// Delphi GetKPrecision: IntPower(10, Max(MIN(aPrecision,10), 0))
	type tc struct {
		precision float64
		want      float64
	}
	cases := []tc{
		{-5, 1},
		{0, 1},
		{2, 100},
		{10, 1e10},
		{15, 1e10},
		{308, 1e10},
	}
	for i := range cases {
		opts := Options{Precision: cases[i].precision}
		got, err := opts.kPrecision()
		if err != nil {
			t.Errorf("precision %v: unexpected error %v", cases[i].precision, err)
			continue
		}
		if got != cases[i].want {
			t.Errorf("precision %v: got %v, want %v", cases[i].precision, got, cases[i].want)
		}
	}
	bad := []float64{1.5, math.NaN(), math.Inf(1), math.Inf(-1)}
	for i := range bad {
		opts := Options{Precision: bad[i]}
		if _, err := opts.kPrecision(); err == nil {
			t.Errorf("precision %v: expected error, got nil", bad[i])
		}
	}
}
