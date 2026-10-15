package geometrycodec

import (
	"encoding/binary"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
)

func rawTestWKB(order binary.ByteOrder, code uint32, payload []byte) []byte {
	marker := byte(1)
	if order == binary.BigEndian {
		marker = 0
	}
	header := make([]byte, 5)
	header[0] = marker
	order.PutUint32(header[1:], code)
	return append(header, payload...)
}
func rawTestPositions(order binary.ByteOrder, points ...[]float64) []byte {
	bytes := []byte{}
	for _, point := range points {
		for _, value := range point {
			encoded := make([]byte, 8)
			order.PutUint64(encoded, math.Float64bits(value))
			bytes = append(bytes, encoded...)
		}
	}
	return bytes
}
func rawTestCount(order binary.ByteOrder, count uint32, payload []byte) []byte {
	header := make([]byte, 4)
	order.PutUint32(header, count)
	return append(header, payload...)
}

func TestDecodeRawWKBAllFamiliesAndOrders(t *testing.T) {
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		for _, z := range []bool{false, true} {
			offset := uint32(0)
			point := []float64{1, 2}
			second := []float64{3, 4}
			third := []float64{5, 6}
			var expectedPoint geom.Geometry = geom.Point{1, 2}
			var expectedLine geom.Geometry = geom.LineString{{1, 2}, {3, 4}}
			var expectedPolygon geom.Geometry = geom.Polygon{{{1, 2}, {3, 4}, {5, 6}, {1, 2}}}
			var expectedMultiPoint geom.Geometry = geom.MultiPoint{{1, 2}, {3, 4}}
			var expectedMultiLine geom.Geometry = geom.MultiLineString{{{1, 2}, {3, 4}}}
			var expectedMultiPolygon geom.Geometry = geom.MultiPolygon{{{{1, 2}, {3, 4}, {5, 6}, {1, 2}}}}
			if z {
				offset = 1000
				point = []float64{1, 2, 3}
				second = []float64{3, 4, 5}
				third = []float64{5, 6, 7}
				expectedPoint = geom.PointZ{1, 2, 3}
				expectedLine = geom.LineStringZ{{1, 2, 3}, {3, 4, 5}}
				expectedPolygon = geom.PolygonZ{{{1, 2, 3}, {3, 4, 5}, {5, 6, 7}, {1, 2, 3}}}
				expectedMultiPoint = geom.MultiPointZ{{1, 2, 3}, {3, 4, 5}}
				expectedMultiLine = geom.MultiLineStringZ{{{1, 2, 3}, {3, 4, 5}}}
				expectedMultiPolygon = MultiPolygonZ{{{{1, 2, 3}, {3, 4, 5}, {5, 6, 7}, {1, 2, 3}}}}
			}
			p1 := rawTestWKB(order, offset+1, rawTestPositions(order, point))
			p2 := rawTestWKB(order, offset+1, rawTestPositions(order, second))
			line := rawTestWKB(order, offset+2, rawTestCount(order, 2, rawTestPositions(order, point, second)))
			polygon := rawTestWKB(order, offset+3, rawTestCount(order, 1, rawTestCount(order, 4, rawTestPositions(order, point, second, third, point))))
			fixtures := []struct {
				name  string
				bytes []byte
				want  geom.Geometry
			}{
				{"Point", p1, expectedPoint}, {"Line", line, expectedLine}, {"Polygon", polygon, expectedPolygon},
				{"MultiPoint", rawTestWKB(order, offset+4, rawTestCount(order, 2, append(append([]byte{}, p1...), p2...))), expectedMultiPoint},
				{"MultiLine", rawTestWKB(order, offset+5, rawTestCount(order, 1, line)), expectedMultiLine},
				{"MultiPolygon", rawTestWKB(order, offset+6, rawTestCount(order, 1, polygon)), expectedMultiPolygon},
				{"Collection", rawTestWKB(order, offset+7, rawTestCount(order, 2, append(append([]byte{}, p1...), line...))), geom.Collection{expectedPoint, expectedLine}},
			}
			for _, tc := range fixtures {
				t.Run(order.String()+tc.name+map[bool]string{false: "XY", true: "XYZ"}[z], func(t *testing.T) {
					original := append([]byte{}, tc.bytes...)
					got, err := DecodeRawWKB(tc.bytes)
					if err != nil || !reflect.DeepEqual(got, tc.want) {
						t.Fatalf("got%#v want%#v err%v", got, tc.want, err)
					}
					if !reflect.DeepEqual(original, tc.bytes) {
						t.Fatal("input bytes mutated")
					}
				})
			}
		}
	}
}

func TestDecodeRawWKBEmptyAndMixed(t *testing.T) {
	order := binary.LittleEndian
	for _, z := range []bool{false, true} {
		offset := uint32(0)
		nan := []float64{math.NaN(), math.NaN()}
		if z {
			offset = 1000
			nan = append(nan, math.NaN())
		}
		emptyPoint := rawTestWKB(order, offset+1, rawTestPositions(order, nan))
		got, err := DecodeRawWKB(emptyPoint)
		if err != nil || got != nil {
			t.Fatalf("empty point: %#v %v", got, err)
		}
		for kind := uint32(2); kind <= 7; kind++ {
			got, err := DecodeRawWKB(rawTestWKB(order, offset+kind, rawTestCount(order, 0, nil)))
			expectedXY := []geom.Geometry{geom.LineString{}, geom.Polygon{}, geom.MultiPoint{}, geom.MultiLineString{}, geom.MultiPolygon{}, geom.Collection{}}
			expectedZ := []geom.Geometry{geom.LineStringZ{}, geom.PolygonZ{}, geom.MultiPointZ{}, geom.MultiLineStringZ{}, MultiPolygonZ{}, geom.Collection{}}
			want := expectedXY[kind-2]
			if z {
				want = expectedZ[kind-2]
			}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("typedempty%d: %#v %v", kind, got, err)
			}
		}
		_, err = DecodeRawWKB(rawTestWKB(order, offset+4, rawTestCount(order, 1, emptyPoint)))
		if err == nil {
			t.Fatal("empty MultiPoint member accepted")
		}
		got, err = DecodeRawWKB(rawTestWKB(order, offset+7, rawTestCount(order, 1, emptyPoint)))
		if err != nil || !reflect.DeepEqual(got, geom.Collection{nil}) {
			t.Fatalf("empty collection member: %#v %v", got, err)
		}
	}
	xy := rawTestWKB(binary.BigEndian, 1, rawTestPositions(binary.BigEndian, []float64{1, 2}))
	xyz := rawTestWKB(order, 1001, rawTestPositions(order, []float64{3, 4, 5}))
	nested := rawTestWKB(order, 7, rawTestCount(order, 2, append(xy, xyz...)))
	got, err := DecodeRawWKB(rawTestWKB(order, 1007, rawTestCount(order, 1, nested)))
	if err != nil || !reflect.DeepEqual(got, geom.Collection{geom.Collection{geom.Point{1, 2}, geom.PointZ{3, 4, 5}}}) {
		t.Fatalf("mixed collection: %#v %v", got, err)
	}
}

func TestDecodeRawWKBRejectsMalformed(t *testing.T) {
	o := binary.LittleEndian
	point := rawTestWKB(o, 1001, rawTestPositions(o, []float64{1, 2, 3}))
	fixtures := [][]byte{{}, {2, 1, 0, 0, 0}, append(append([]byte{}, point...), 0), rawTestWKB(o, 1002, rawTestCount(o, math.MaxUint32, nil)), rawTestWKB(o, 1002, rawTestCount(o, 1, rawTestPositions(o, []float64{1, 2, 3}))), rawTestWKB(o, 1003, rawTestCount(o, 1, rawTestCount(o, 2, rawTestPositions(o, []float64{1, 2, 3}, []float64{2, 3, 4}))))}
	for _, code := range []uint32{0, 8, 1008, 2001, 3001, 0x80000001, 0x20000001} {
		fixtures = append(fixtures, rawTestWKB(o, code, nil))
	}
	for axis := range 3 {
		for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
			p := []float64{1, 2, 3}
			p[axis] = bad
			fixtures = append(fixtures, rawTestWKB(o, 1001, rawTestPositions(o, p)))
		}
	}
	fixtures = append(fixtures, rawTestWKB(o, 1004, rawTestCount(o, 1, rawTestWKB(o, 1, rawTestPositions(o, []float64{1, 2})))))
	for i, fixture := range fixtures {
		if _, err := DecodeRawWKB(fixture); err == nil {
			t.Fatalf("malformed fixture%d accepted", i)
		}
	}
	for length := 0; length < len(point); length++ {
		if _, err := DecodeRawWKB(point[:length]); err == nil {
			t.Fatalf("truncated prefix%d accepted", length)
		}
	}
	for depth := 0; depth < rawMaxDepth; depth++ {
		point = rawTestWKB(o, 7, rawTestCount(o, 1, point))
	}
	if _, err := DecodeRawWKB(point); err != nil {
		t.Fatalf("alloweddepth rejected: %v", err)
	}
	point = rawTestWKB(o, 7, rawTestCount(o, 1, point))
	if _, err := DecodeRawWKB(point); err == nil {
		t.Fatal("depth overflow accepted")
	}
	if _, err := DecodeRawWKB("notbytes"); err == nil {
		t.Fatal("wrong source type accepted")
	}
	if got, err := DecodeRawWKB(nil); err != nil || got != nil {
		t.Fatal("SQL NULL rejected")
	}
}

func TestRawWKBElementBudgetAndSignedZero(t *testing.T) {
	d := rawWKBDecoder{data: binary.LittleEndian.AppendUint32(nil, 1), elements: rawMaxElements}
	if _, err := d.count(binary.LittleEndian, 0); err == nil {
		t.Fatal("cumulative element budget exceeded")
	}
	input := rawTestWKB(binary.LittleEndian, 1001, rawTestPositions(binary.LittleEndian, []float64{math.Copysign(0, -1), 2, -3}))
	g, err := DecodeRawWKB(input)
	if err != nil {
		t.Fatal(err)
	}
	if !math.Signbit(g.(geom.PointZ)[0]) {
		t.Fatal("signed zero lost")
	}
	// Deep paths are rejected with a bounded explicit error, never recursion panic.
	if _, err := DecodeRawWKB([]byte{1}); err == nil || !strings.Contains(err.Error(), "raw WKB") {
		t.Fatal("decode context missing")
	}
}

func TestDecodeRawWKBByteBudgetAndDetachedCoordinates(t *testing.T) {
	if _, err := DecodeRawWKB(make([]byte, rawMaxInputBytes+1)); err == nil {
		t.Fatal("input bytebudget ignored")
	}
	input := rawTestWKB(binary.LittleEndian, 1002, rawTestCount(binary.LittleEndian, 2, rawTestPositions(binary.LittleEndian, []float64{1, 2, 3}, []float64{4, 5, 6})))
	geometry, err := DecodeRawWKB(input)
	if err != nil {
		t.Fatal(err)
	}
	for i := range input {
		input[i] = 0
	}
	if !reflect.DeepEqual(geometry, geom.LineStringZ{{1, 2, 3}, {4, 5, 6}}) {
		t.Fatal("decodedcoordinates retained callerbytes")
	}
}

func TestDecodeRawWKBRejectsUnclosedWireRings(t *testing.T) {
	o := binary.LittleEndian
	for _, points := range [][][]float64{
		{{0, 0}, {1, 0}, {1, 1}, {0, 1}},
		{{0, 0}, {1, 1}, {0, 0}},
		{{0, 0, 0}, {1, 0, 1}, {1, 1, 1}, {0, 1, 0}},
		{{0, 0, 0}, {1, 1, 1}, {0, 0, 0}},
		{{0, 0, 0}, {1, 0, 1}, {1, 1, 1}, {0, 0, 9}},
	} {
		code := uint32(3)
		if len(points[0]) == 3 {
			code = 1003
		}
		payload := rawTestCount(o, 1, rawTestCount(o, uint32(len(points)), rawTestPositions(o, points...)))
		if _, err := DecodeRawWKB(rawTestWKB(o, code, payload)); err == nil {
			t.Fatal("invalid wire ring closure/cardinality accepted")
		}
	}
}
