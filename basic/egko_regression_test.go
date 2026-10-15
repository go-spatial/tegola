package basic_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/mvt"
	"github.com/alexeydott/geom/encoding/wkt"
	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/mos"
	"google.golang.org/protobuf/proto"
)

const egkoCRS = "+proj=etmerc +ellps=bessel +towgs84=41,-107.6,-93,0,0,0,0 +x_0=0 +y_0=0 +lon_0=37.5 +k_0=1 +lat_0=55.6666666667 +units=m +no_defs"

const egkoLineWKT = "LINESTRING(37.14490635 55.98925629,37.14500075 55.98920384,37.14519603 55.98912360,37.14529883 55.98913960)"

const egkoPolygonWKT = "POLYGON((37.14500075 55.98920384,37.14519603 55.98912360,37.14529883 55.98913960,37.14515614 55.98913664,37.14500075 55.98920384))"

// These are the same coordinates as the WKT fixtures above, inverse-projected
// to the EGKO Bessel CRS and quantized as integer millimetres in MOS.
var egkoLineMOS = [][2]int32{
	{-22048321, 35932395},
	{-22042460, 35926526},
	{-22030320, 35917530},
	{-22023896, 35919278},
}

var egkoPolygonMOS = [][2]int32{
	{-22042460, 35926526},
	{-22030320, 35917530},
	{-22023896, 35919278},
	{-22032802, 35918995},
	{-22042460, 35926526},
}

func TestEGKORegressionLineMOSMatchesWKT(t *testing.T) {
	srid := registerEGKOCRS(t)
	wktGeometry := decodeWKT(t, egkoLineWKT)
	mosGeometry := decodeEGKOMOS(t, mos.TypePolyline, []uint32{uint32(len(egkoLineMOS))}, egkoLineMOS)

	assertGeometryType(t, wktGeometry, geom.LineString{})
	assertGeometryType(t, mosGeometry, geom.LineString{})
	assertWebMercatorCoordinatesMatch(t, srid, wktGeometry, mosGeometry, 0.1)
	assertMVTQuantization(t, wktGeometry, geom.LineString{}, 4941, 2551, 13)
}

func TestEGKORegressionPolygonMOSMatchesWKT(t *testing.T) {
	srid := registerEGKOCRS(t)
	wktGeometry := decodeWKT(t, egkoPolygonWKT)
	mosGeometry := decodeEGKOMOS(t, mos.TypePolygon, []uint32{uint32(len(egkoPolygonMOS))}, egkoPolygonMOS)

	assertGeometryType(t, wktGeometry, geom.Polygon{})
	assertGeometryType(t, mosGeometry, geom.Polygon{})
	assertWebMercatorCoordinatesMatch(t, srid, wktGeometry, mosGeometry, 0.1)
	assertMVTQuantization(t, wktGeometry, geom.Polygon{}, 4941, 2551, 13)
}

func registerEGKOCRS(t *testing.T) uint64 {
	t.Helper()
	srid, err := basic.RegisterProj4Defn(egkoCRS)
	if err != nil {
		t.Fatalf("register EGKO CRS: %v", err)
	}
	return uint64(srid)
}

func decodeWKT(t *testing.T, value string) geom.Geometry {
	t.Helper()
	geometry, err := wkt.DecodeString(value)
	if err != nil {
		t.Fatalf("decode WKT fixture: %v", err)
	}
	return geometry
}

func decodeEGKOMOS(t *testing.T, objectType byte, counts []uint32, points [][2]int32) geom.Geometry {
	t.Helper()
	blob := buildMOSFixture(objectType, counts, points)
	geometry, err := mos.Decode(blob, mos.Options{Precision: 0, UnitFactor: 0.001})
	if err != nil {
		t.Fatalf("decode MOS fixture: %v", err)
	}
	return geometry
}

func buildMOSFixture(objectType byte, counts []uint32, points [][2]int32) []byte {
	var buf bytes.Buffer
	_ = buf.WriteByte(objectType)
	_ = buf.WriteByte(0)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(len(counts)))
	_ = binary.Write(&buf, binary.LittleEndian, int32(len(points)))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0))
	for _, count := range counts {
		_ = binary.Write(&buf, binary.LittleEndian, count)
	}
	for _, point := range points {
		_ = binary.Write(&buf, binary.LittleEndian, point[0])
		_ = binary.Write(&buf, binary.LittleEndian, point[1])
	}
	return buf.Bytes()
}

func assertGeometryType(t *testing.T, got, want geom.Geometry) {
	t.Helper()
	switch want.(type) {
	case geom.LineString:
		if _, ok := got.(geom.LineString); !ok {
			t.Fatalf("geometry type = %T, want LineString", got)
		}
	case geom.Polygon:
		if _, ok := got.(geom.Polygon); !ok {
			t.Fatalf("geometry type = %T, want Polygon", got)
		}
	}
}

func assertWebMercatorCoordinatesMatch(t *testing.T, srid uint64, wktGeometry, mosGeometry geom.Geometry, tolerance float64) {
	t.Helper()
	wktPoints := geometryPoints(wktGeometry)
	mosPoints := geometryPoints(mosGeometry)
	if len(wktPoints) != len(mosPoints) {
		t.Fatalf("point count mismatch: WKT=%d MOS=%d", len(wktPoints), len(mosPoints))
	}
	for i := range wktPoints {
		want, err := basic.ToWebMercator(tegola.WGS84, wktPoints[i])
		if err != nil {
			t.Fatalf("WKT point %d to WebMercator: %v", i, err)
		}
		got, err := basic.ToWebMercator(srid, mosPoints[i])
		if err != nil {
			t.Fatalf("MOS point %d to WebMercator: %v", i, err)
		}
		wantPoint := want.(geom.Point)
		gotPoint := got.(geom.Point)
		if math.Abs(wantPoint[0]-gotPoint[0]) > tolerance || math.Abs(wantPoint[1]-gotPoint[1]) > tolerance {
			t.Fatalf("point %d mismatch: WKT WM=%v, MOS WM=%v, tolerance=%g", i, wantPoint, gotPoint, tolerance)
		}
	}
}

func assertMVTQuantization(t *testing.T, wktGeometry geom.Geometry, wantType geom.Geometry, tileX, tileY, zoom int) {
	t.Helper()
	wmGeometry := transformWGS84Geometry(t, wktGeometry)
	extent := webMercatorTileExtent(tileX, tileY, zoom)
	prepared := mvt.PrepareGeo(wmGeometry, extent, float64(mvt.DefaultExtent))
	features := mvt.NewFeatures(prepared, nil)
	layer := &mvt.Layer{Name: "egko-fixture"}
	layer.AddFeatures(features...)
	tile := &mvt.Tile{}
	if err := tile.AddLayers(layer); err != nil {
		t.Fatalf("add MVT layer: %v", err)
	}
	vectorTile, err := tile.VTile(context.Background())
	if err != nil {
		t.Fatalf("build MVT tile: %v", err)
	}
	encoded, err := proto.Marshal(vectorTile)
	if err != nil {
		t.Fatalf("encode MVT tile: %v", err)
	}
	decoded, err := mvt.DecodeByte(encoded)
	if err != nil {
		t.Fatalf("decode MVT tile: %v", err)
	}
	gotFeatures := decoded.Layers()[0].Features()
	if len(gotFeatures) != 1 {
		t.Fatalf("decoded MVT feature count = %d, want 1", len(gotFeatures))
	}
	assertGeometryType(t, gotFeatures[0].Geometry, wantType)

	wantPoints := geometryPoints(wmGeometry)
	gotPoints := geometryPointsToWebMercator(gotFeatures[0].Geometry, extent)
	if len(gotPoints) != len(wantPoints) {
		t.Fatalf("quantized point count = %d, want %d", len(gotPoints), len(wantPoints))
	}
	pixelSize := extent.XSpan() / float64(mvt.DefaultExtent)
	for i := range wantPoints {
		gotPoint := gotPoints[i]
		if _, polygon := wantType.(geom.Polygon); polygon {
			gotPoint = nearestPoint(wantPoints[i], gotPoints)
		}
		if math.Abs(wantPoints[i][0]-gotPoint[0]) > pixelSize || math.Abs(wantPoints[i][1]-gotPoint[1]) > pixelSize {
			t.Fatalf("quantized point %d error exceeds one pixel (%g m): want=%v got=%v", i, pixelSize, wantPoints[i], gotPoint)
		}
	}
}

func nearestPoint(want geom.Point, points []geom.Point) geom.Point {
	best := points[0]
	bestDistance := math.Inf(1)
	for _, point := range points {
		distance := math.Hypot(want[0]-point[0], want[1]-point[1])
		if distance < bestDistance {
			best, bestDistance = point, distance
		}
	}
	return best
}

func transformWGS84Geometry(t *testing.T, geometry geom.Geometry) geom.Geometry {
	t.Helper()
	transformed, err := basic.ToWebMercator(tegola.WGS84, geometry)
	if err != nil {
		t.Fatalf("transform WKT geometry: %v", err)
	}
	return transformed
}

func geometryPoints(geometry geom.Geometry) []geom.Point {
	switch value := geometry.(type) {
	case geom.LineString:
		points := make([]geom.Point, len(value))
		for i, point := range value {
			points[i] = geom.Point(point)
		}
		return points
	case geom.Polygon:
		var points []geom.Point
		for _, ring := range value {
			limit := len(ring)
			if limit > 1 && ring[0] == ring[limit-1] {
				limit--
			}
			for _, point := range ring[:limit] {
				points = append(points, geom.Point(point))
			}
		}
		return points
	default:
		return nil
	}
}

func geometryPointsToWebMercator(geometry geom.Geometry, extent *geom.Extent) []geom.Point {
	points := geometryPoints(geometry)
	for i, point := range points {
		points[i] = geom.Point{
			extent.MinX() + point.X()/float64(mvt.DefaultExtent)*extent.XSpan(),
			extent.MaxY() - point.Y()/float64(mvt.DefaultExtent)*extent.YSpan(),
		}
	}
	return points
}

func webMercatorTileExtent(tileX, tileY, zoom int) *geom.Extent {
	const worldExtent = 20037508.342789244
	scale := 2 * worldExtent / math.Exp2(float64(zoom))
	minX := -worldExtent + float64(tileX)*scale
	maxY := worldExtent - float64(tileY)*scale
	return geom.NewExtent([2]float64{minX, maxY - scale}, [2]float64{minX + scale, maxY})
}
