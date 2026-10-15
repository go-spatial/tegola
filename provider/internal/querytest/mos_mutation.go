package querytest

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/test/fixture"
)

// MOSMutationInstance exposes native storage independently of the provider reader.
// Factory implementations use disposable plain tables, EPSG:3857, precision=2,
// units=m and the public properties name and value.
type MOSMutationInstance struct {
	Writer  provider.MutationProvider
	Querier provider.FeatureQuerier
	Raw     func(uint64) []byte
	SetRaw  func(uint64, []byte)
}

func RunMOSMutations(t *testing.T, factory func(*testing.T, string) MOSMutationInstance) {
	outer := [][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}}
	hole := [][2]float64{{4, 4}, {4, 6}, {6, 6}, {6, 4}, {4, 4}}
	other := [][2]float64{{20, 0}, {30, 0}, {30, 10}, {20, 10}, {20, 0}}
	cases := []struct {
		name, kind  string
		input, want geom.Geometry
	}{
		{"point", "point", geom.Point{1.234, -2.346}, geom.Point{1.23, -2.35}},
		{"line", "linestring", geom.LineString{{1.234, 2.346}, {3.456, 4.567}}, geom.LineString{{1.23, 2.35}, {3.46, 4.57}}},
		{"polygon_hole", "polygon", geom.Polygon{outer, hole}, geom.Polygon{outer, hole}},
		{"multipoint", "multipoint", geom.MultiPoint{{1.234, 2.346}, {3.456, 4.567}}, geom.MultiPoint{{1.23, 2.35}, {3.46, 4.57}}},
		{"multiline", "multilinestring", geom.MultiLineString{{{1, 2}, {3, 4}}, {{10, 20}, {30, 40}}}, geom.MultiLineString{{{1, 2}, {3, 4}}, {{10, 20}, {30, 40}}}},
		{"multipolygon_hole", "multipolygon", geom.MultiPolygon{{outer, hole}, {other}}, geom.MultiPolygon{{outer, hole}, {other}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			instance := factory(t, tc.kind)
			ctx := context.Background()
			mutate := func(m provider.Mutation) provider.MutationOutcome {
				t.Helper()
				tx, err := instance.Writer.BeginFeatureTx(ctx, provider.TxOptions{})
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = tx.Rollback(ctx) }()
				out, err := tx.Apply(ctx, m)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				return out
			}
			read := func(q provider.FeatureQuery) []provider.Feature {
				t.Helper()
				rows := []provider.Feature{}
				_, err := instance.Querier.QueryFeatures(ctx, "items", q, func(f *provider.Feature) error { rows = append(rows, *f); return nil })
				if err != nil {
					t.Fatal(err)
				}
				return rows
			}
			raw, err := wkb.EncodeBytes(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			name := provider.MutationValue{Kind: provider.MutationValueString, String: "original"}
			out := mutate(provider.Mutation{Op: provider.MutationInsert, Collection: "items", Properties: map[string]provider.MutationValue{"name": name}, GeometryWKB: raw, GeometrySRID: 3857})
			query := provider.FeatureQuery{Limit: 10, IDs: []uint64{out.FeatureID}}
			rows := read(query)
			if len(rows) != 1 || !reflect.DeepEqual(rows[0].Geometry, tc.want) {
				t.Fatalf("stored geometry: %+v; want %#v", rows, tc.want)
			}
			// Real MOS objects can have annotations after the geometry prefix.
			// An attribute-only write must preserve every one of those bytes.
			encoded := instance.Raw(out.FeatureID)
			// Convert the writer's compatible 12-byte header into native MapplGIS
			// 10-byte form, then append opaque annotation data.
			original := append(append([]byte{}, encoded[:10]...), encoded[12:]...)
			original = append(original, []byte("opaque-source-annotation")...)
			instance.SetRaw(out.FeatureID, original)
			name.String = "patched"
			out = mutate(provider.Mutation{Op: provider.MutationUpdate, Collection: "items", FeatureID: out.FeatureID, IfRevision: out.Revision, Properties: map[string]provider.MutationValue{"name": name}})
			if !bytes.Equal(instance.Raw(out.FeatureID), original) {
				t.Fatal("attribute write changed MOS source bytes")
			}
			literal, err := provider.NewFilterLiteral(provider.FilterString, "patched")
			if err != nil {
				t.Fatal(err)
			}
			filter, err := provider.NewFilterExpression(provider.FilterNode{Kind: provider.FilterCompare, Property: "name", Operator: provider.FilterEqual, Literal: literal})
			if err != nil {
				t.Fatal(err)
			}
			query.Filter = &filter
			if rows := read(query); len(rows) != 1 {
				t.Fatal("attribute filter lost written MOS row")
			}
			query.Filter = nil
			query.Bounds = []geom.Extent{{-100, -100, 100, 100}}
			query.BoundsSRID = 3857
			if rows := read(query); len(rows) != 1 {
				t.Fatal("bbox lost written MOS geometry")
			}
			query.Bounds = []geom.Extent{{1000, 1000, 1001, 1001}}
			if rows := read(query); len(rows) != 0 {
				t.Fatal("bbox admitted disjoint MOS geometry")
			}
			if tc.kind == "polygon" || tc.kind == "multipolygon" {
				query.Bounds = []geom.Extent{{4.5, 4.5, 5.5, 5.5}}
				if rows := read(query); len(rows) != 0 {
					t.Fatal("polygon hole was filled during MOS roundtrip")
				}
			}
			query.Bounds = nil
			query.BoundsSRID = 0
			// A geometry replacement must remain readable and retain type/rings.
			out = mutate(provider.Mutation{Op: provider.MutationReplace, Collection: "items", FeatureID: out.FeatureID, IfRevision: out.Revision, Properties: map[string]provider.MutationValue{"name": name}, GeometryWKB: raw, GeometrySRID: 3857})
			rows = read(query)
			if len(rows) != 1 || !reflect.DeepEqual(rows[0].Geometry, tc.want) {
				t.Fatalf("replacement changed topology: %+v", rows)
			}
			mutate(provider.Mutation{Op: provider.MutationDelete, Collection: "items", FeatureID: out.FeatureID, IfRevision: out.Revision})
			if rows := read(query); len(rows) != 0 {
				t.Fatal("deleted MOS feature remains visible")
			}
		})
	}
	t.Run("source_crs", func(t *testing.T) {
		i := factory(t, "point")
		ctx := context.Background()
		tx, err := i.Writer.BeginFeatureTx(ctx, provider.TxOptions{})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		raw, err := wkb.EncodeBytes(geom.Point{0.0001, 0.0002})
		if err != nil {
			t.Fatal(err)
		}
		out, err := tx.Apply(ctx, provider.Mutation{Op: provider.MutationInsert, Collection: "items", GeometryWKB: raw, GeometrySRID: 4326})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		count := 0
		_, err = i.Querier.QueryFeatures(ctx, "items", provider.FeatureQuery{IDs: []uint64{out.FeatureID}, Limit: 1}, func(f *provider.Feature) error {
			count++
			if !reflect.DeepEqual(f.Geometry, geom.Point{11.13, 22.26}) {
				return fmt.Errorf("source CRS/quantization: got %#v", f.Geometry)
			}
			return nil
		})
		if err != nil || count != 1 {
			t.Fatalf("source CRS read count=%d error=%v", count, err)
		}
	})
}

// NativeMOSPoint constructs the independent 10-byte MapplGIS header fixture.
func NativeMOSPoint(x, y int32) []byte {
	data := []byte{2, 0, 0, 0, 1, 0, 1, 0, 0, 0}
	data = binary.LittleEndian.AppendUint32(data, 1)
	data = binary.LittleEndian.AppendUint32(data, uint32(x))
	return binary.LittleEndian.AppendUint32(data, uint32(y))
}

// AssertMOSBoundsProfile checks the documented integer-bounds read profile.
// The fixture contains native points (123,235) and (100000,200000), scaled
// with either precision=2/metres or precision=0/centimetres.
func AssertMOSBoundsProfile(t *testing.T, tiler provider.Tiler, querier provider.FeatureQuerier, writer provider.MutationProvider, custom, writableBounds bool) {
	t.Helper()
	ctx := context.Background()
	for _, tc := range []struct {
		bounds geom.Extent
		want   []uint64
	}{
		{geom.Extent{1.20, 2.30, 1.30, 2.40}, []uint64{1}},
		{geom.Extent{2, 3, 3, 4}, []uint64{}},
	} {
		ids := []uint64{}
		tile := fixture.Tile{SRID: 3857, Bounds: &tc.bounds, BufferedBounds: &tc.bounds}
		err := tiler.TileFeatures(ctx, "items", tile, nil, func(f *provider.Feature) error {
			ids = append(ids, f.ID)
			if !reflect.DeepEqual(f.Geometry, geom.Point{1.23, 2.35}) {
				return fmt.Errorf("native MOS units mismatch: %#v", f.Geometry)
			}
			for _, field := range []string{"MINX", "MAXX", "MINY", "MAXY"} {
				if _, ok := f.Tags[field]; ok {
					return fmt.Errorf("private bounds exposed: %s", field)
				}
			}
			return nil
		})
		if err != nil || !reflect.DeepEqual(ids, tc.want) {
			t.Fatalf("bounds-backed tile IDs=%v want=%v err=%v", ids, tc.want, err)
		}
		if !custom {
			ids = []uint64{}
			_, err := querier.QueryFeatures(ctx, "items", provider.FeatureQuery{Limit: 10, BoundsSRID: 3857, Bounds: []geom.Extent{tc.bounds}}, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
			if err != nil || !reflect.DeepEqual(ids, tc.want) {
				t.Fatalf("bounds-backed feature IDs=%v want=%v err=%v", ids, tc.want, err)
			}
		}
	}
	_, admissionErr := writer.DescribeWritable(ctx, "items")
	if wantWritable := !custom && writableBounds; (admissionErr == nil) != wantWritable {
		t.Fatalf("MOS bounds writable=%v, admission error=%v", wantWritable, admissionErr)
	}
	if custom {
		_, err := querier.QueryFeatures(ctx, "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { return nil })
		if !errors.Is(err, provider.ErrUnsupported) {
			t.Fatalf("custom tile SQL incorrectly admitted for feature queries: %v", err)
		}
	}
}
