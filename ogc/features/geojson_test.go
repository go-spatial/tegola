package features

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/tegola/provider"
)

type testWriter func([]byte) (int, error)

func (w testWriter) Write(p []byte) (int, error) { return w(p) }

func TestGeoJSONGeometryTypesAndRingOwnership(t *testing.T) {
	ring := [][2]float64{{0, 0}, {0, 1}, {1, 1}, {1, 0}, {0, 0}}
	cases := []geom.Geometry{geom.Point{0, 0}, geom.MultiPoint{{0, 0}}, geom.LineString{{0, 0}, {1, 1}}, geom.MultiLineString{{{0, 0}, {1, 1}}}, geom.Polygon{ring}, geom.MultiPolygon{{ring}}, geom.Collection{geom.Point{0, 0}, geom.Collection{geom.Point{1, 1}}}, nil}
	for _, geometry := range cases {
		source := &provider.Feature{ID: 1, SRID: 4326, Geometry: geometry}
		s := newTestService(t, 4326, featureQuerier(source))
		page, err := s.QueryCollectionPage(context.Background(), "public", provider.FeatureQuery{Limit: 1})
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		if err := WriteGeoJSON(context.Background(), &output, page); err != nil {
			t.Fatal(err)
		}
		if !json.Valid(output.Bytes()) {
			t.Fatalf("invalid GeoJSON: %s", output.Bytes())
		}
	}
	if ring[1] != ([2]float64{0, 1}) {
		t.Fatal("polygon orientation changed source ring")
	}
}

func TestGeoJSONWritesErrorsAndEmptyPage(t *testing.T) {
	page := FeatureCollection{Type: "FeatureCollection", Features: []Feature{}}
	var output bytes.Buffer
	if err := WriteGeoJSON(context.Background(), &output, page); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), []byte(`"features":[]`)) {
		t.Fatal(output.String())
	}
	sentinel := errors.New("write failed")
	if err := WriteGeoJSON(context.Background(), testWriter(func([]byte) (int, error) { return 0, sentinel }), page); !errors.Is(err, sentinel) {
		t.Fatalf("lost writer chain: %v", err)
	}
	if err := WriteGeoJSON(context.Background(), testWriter(func(p []byte) (int, error) { return len(p) - 1, nil }), page); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	output.Reset()
	if err := WriteGeoJSON(ctx, &output, page); !errors.Is(err, context.Canceled) || output.Len() != 0 {
		t.Fatalf("cancelled output: %v", err)
	}
	page.NumberReturned = 1
	page.Features = []Feature{{Type: "Feature", ID: 1, Geometry: json.RawMessage(`null`), Properties: map[string]any{"invalid": make(chan int)}}}
	if err := WriteGeoJSON(context.Background(), &output, page); err == nil || output.Len() != 0 {
		t.Fatalf("encoding failure wrote output: %v", err)
	}
}

func TestGeoJSONCancellationBetweenWriteChunks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	page := FeatureCollection{Type: "FeatureCollection", NumberReturned: 1, Features: []Feature{{Type: "Feature", Geometry: json.RawMessage("null"), Properties: map[string]any{"large": strings.Repeat("x", 100000)}}}}
	writes := 0
	err := WriteGeoJSON(ctx, testWriter(func(p []byte) (int, error) { writes++; cancel(); return len(p), nil }), page)
	if !errors.Is(err, context.Canceled) || writes != 1 {
		t.Fatalf("write did not stop after cancellation: %d %v", writes, err)
	}
}

func TestGeoJSONExactZeroAndUnknownCount(t *testing.T) {
	zero := uint64(0)
	for _, matched := range []*uint64{nil, &zero} {
		page := FeatureCollection{Type: "FeatureCollection", NumberMatched: matched}
		var output bytes.Buffer
		if err := WriteGeoJSON(context.Background(), &output, page); err != nil {
			t.Fatal(err)
		}
		present := bytes.Contains(output.Bytes(), []byte(`"numberMatched":0`))
		if present != (matched != nil) {
			t.Fatalf("unknown/zero distinction lost: %s", output.Bytes())
		}
	}
}

func TestGeoJSONClosesWKBPolygonRingsOnDetachedCopies(t *testing.T) {
	closed := geom.Polygon{
		{{0, 0}, {0, 4}, {4, 4}, {4, 0}, {0, 0}},
		{{1, 1}, {3, 1}, {3, 3}, {1, 3}, {1, 1}},
	}
	raw, err := wkb.EncodeBytes(closed)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := wkb.DecodeBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	polygon := decoded.(geom.Polygon)
	if len(polygon[0]) != 4 || polygon[0][0] == polygon[0][len(polygon[0])-1] {
		t.Fatal("WKB fixture did not produce implicit closure")
	}
	before, err := json.Marshal(polygon)
	if err != nil {
		t.Fatal(err)
	}
	for _, geometry := range []geom.Geometry{polygon, geom.MultiPolygon{polygon}, geom.Collection{geom.Collection{polygon}, geom.MultiPolygon{polygon}}} {
		s := newTestService(t, 4326, featureQuerier(&provider.Feature{ID: 1, SRID: 4326, Geometry: geometry}))
		result, err := s.QueryFeature(context.Background(), "public", 1)
		if err != nil {
			t.Fatal(err)
		}
		var inspect func(map[string]any)
		inspect = func(g map[string]any) {
			switch g["type"] {
			case "GeometryCollection":
				for _, child := range g["geometries"].([]any) {
					inspect(child.(map[string]any))
				}
			case "MultiPolygon":
				for _, coordinates := range g["coordinates"].([]any) {
					inspect(map[string]any{"type": "Polygon", "coordinates": coordinates})
				}
			case "Polygon":
				rings := g["coordinates"].([]any)
				if len(rings) != 2 {
					t.Fatal("hole lost")
				}
				for i, value := range rings {
					ring := value.([]any)
					if len(ring) != 5 || !reflect.DeepEqual(ring[0], ring[4]) {
						t.Fatalf("ring not closed: %v", ring)
					}
					var area float64
					for j := 1; j < len(ring); j++ {
						a, b := ring[j-1].([]any), ring[j].([]any)
						area += a[0].(float64)*b[1].(float64) - b[0].(float64)*a[1].(float64)
					}
					if (i == 0 && area <= 0) || (i != 0 && area >= 0) {
						t.Fatalf("incorrect ring winding: %v", ring)
					}
				}
			}
		}
		geometryObject := map[string]any{}
		if err := json.Unmarshal(result.Geometry, &geometryObject); err != nil {
			t.Fatal(err)
		}
		inspect(geometryObject)
		after, err := json.Marshal(polygon)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("provider-owned rings changed")
		}
	}
}

func TestGeoJSONTinyTranslatedPolygonAndHole(t *testing.T) {
	// Absolute-coordinate products cancel to zero at this ordinary location.
	tiny := [][2]float64{{100, 50}, {100, 50 + 1e-8}, {100 + 1e-8, 50}}
	hole := [][2]float64{{100 + 2e-9, 50 + 2e-9}, {100 + 4e-9, 50 + 2e-9}, {100 + 2e-9, 50 + 4e-9}}
	source := geom.Polygon{tiny, hole}
	before, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	s := newTestService(t, 4326, featureQuerier(&provider.Feature{ID: 1, SRID: 4326, Geometry: source}))
	feature, err := s.QueryFeature(context.Background(), "public", 1)
	if err != nil {
		t.Fatal(err)
	}
	var geometry struct{ Coordinates [][][2]float64 }
	if err := json.Unmarshal(feature.Geometry, &geometry); err != nil {
		t.Fatal(err)
	}
	for i, ring := range geometry.Coordinates {
		if len(ring) != 4 || ring[0] != ring[3] {
			t.Fatalf("tiny ring not closed: %v", ring)
		}
		sign := ringAreaSign(ring)
		if (i == 0 && sign != 1) || (i == 1 && sign != -1) {
			t.Fatalf("tiny ring winding: %d %v", sign, ring)
		}
	}
	after, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("tiny polygon input mutated")
	}
}

func TestPolygonAreaSignExactFallback(t *testing.T) {
	cases := []struct {
		name string
		ring [][2]float64
		sign int
	}{
		{name: "collinear", ring: [][2]float64{{100, 50}, {101, 51}, {102, 52}, {100, 50}}, sign: 0},
		{name: "near collinear", ring: [][2]float64{{0, 0}, {1, 1}, {2, math.Nextafter(2, 3)}, {0, 0}}, sign: 1},
		{name: "subnormal product", ring: [][2]float64{{0, 0}, {math.SmallestNonzeroFloat64, 0}, {0, math.SmallestNonzeroFloat64}, {0, 0}}, sign: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if sign := ringAreaSign(tc.ring); sign != tc.sign {
				t.Fatalf("sign %d want %d", sign, tc.sign)
			}
		})
	}
	if err := normalizePolygon(geom.Polygon{cases[0].ring}); err == nil {
		t.Fatal("true-zero polygon accepted")
	}
}
