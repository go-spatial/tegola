//go:build cgo

package gpkg

import (
	"context"
	"encoding/base64"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

func TestDecodeRowErrorPrecedence(t *testing.T) {
	metadata := make([]byte, 64)
	metadata[0] = 5
	copy(metadata[1:], "Ver 1")
	if !codec.IsSystemInfoValue(metadata) {
		t.Fatal("invalid system-info fixture")
	}
	cases := []struct {
		name   string
		format string
		geom   any
		id     any
	}{
		{name: "null geometry bad id", geom: nil, id: int64(-1)},
		{name: "empty geometry bad id", geom: gpkgHeader(1<<4, 3857), id: int64(-1)},
		{name: "null id bad geometry", format: codec.FormatWKT, geom: "bad geometry", id: nil},
		{name: "metadata bad id", format: codec.FormatMOS, geom: metadata, id: int64(-1)},
	}
	for _, tc := range cases {
		for _, geomFirst := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{false: "/id-first", true: "/geom-first"}[geomFirst], func(t *testing.T) {
				columns := []string{"fid", "geom"}
				values := []any{tc.id, tc.geom}
				if geomFirst {
					columns = []string{"geom", "fid"}
					values = []any{tc.geom, tc.id}
				}
				p := &Provider{}
				layer := &Layer{name: "test", idFieldname: "fid", geomFieldname: "geom", geometryFormat: tc.format}
				_, err := p.decodeRow(context.Background(), layer, columns, values, rowDecodePolicy{})
				if err == nil {
					t.Fatal("excluded row hid a decoding error")
				}
			})
		}
	}
}

func TestDecodeRowDuplicateGeometryAbsence(t *testing.T) {
	for _, absent := range []any{nil, gpkgHeader(1<<4, 3857)} {
		for _, absentFirst := range []bool{false, true} {
			layer := &Layer{name: "test", idFieldname: "fid", geomFieldname: "geom"}
			valid := append(gpkgHeader(1, 3857), wkbPointLE...)
			values := []any{int64(1), valid, absent}
			if absentFirst {
				values = []any{int64(1), absent, valid}
			}
			decoded, err := (&Provider{}).decodeRow(context.Background(), layer, []string{"fid", "geom", "geom"}, values, rowDecodePolicy{})
			if err != nil || !decoded.Representable || !decoded.HadAbsentGeometry || decoded.Feature.Geometry == nil {
				t.Fatalf("duplicate geometry evidence lost: %+v, err=%v", decoded, err)
			}
			// Tile delivery checks the sticky absence evidence even when the
			// final decoded geometry is non-null; raw representation is separate.
		}
	}
	layer := &Layer{name: "test", idFieldname: "fid", geomFieldname: "geom", geometryFormat: codec.FormatWKT}
	decoded, err := (&Provider{}).decodeRow(
		context.Background(), layer, []string{"fid", "geom", "geom"}, []any{int64(1), "POINT (1 2)", "POINT (3 4)"}, rowDecodePolicy{},
	)
	if err != nil || !decoded.Representable || decoded.HadAbsentGeometry {
		t.Fatalf("valid duplicate geometry changed: %+v, err=%v", decoded, err)
	}
	want, decodeErr := codec.DecodeWKT("POINT (3 4)")
	if decodeErr != nil || !reflect.DeepEqual(decoded.Feature.Geometry, want) {
		t.Fatalf("valid duplicates must retain the last geometry: geometry=%v, err=%v", decoded.Feature.Geometry, decodeErr)
	}
}

func TestDecodeRowCompetingErrorsKeepColumnOrder(t *testing.T) {
	layer := &Layer{name: "test", idFieldname: "fid", geomFieldname: "geom", geometryFormat: codec.FormatWKT}
	for _, idFirst := range []bool{false, true} {
		columns := []string{"geom", "fid"}
		values := []any{"bad geometry", int64(-1)}
		if idFirst {
			columns = []string{"fid", "geom"}
			values = []any{int64(-1), "bad geometry"}
		}
		_, err := (&Provider{}).decodeRow(context.Background(), layer, columns, values, rowDecodePolicy{})
		if err == nil {
			t.Fatal("competing invalid values produced no error")
		}
		isIDError := strings.Contains(err.Error(), "feature ID")
		if isIDError != idFirst {
			t.Fatalf("first error changed for idFirst=%v: %v", idFirst, err)
		}
	}
}

func TestDecodeRowPreservesInputsAndTags(t *testing.T) {
	stamp := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	blob := []byte{0xff, 0x00, 0x01}
	columns := []string{"FID", "Geom", "blob", "stamp", "bound_x", "minx", "Min_Zoom", "unknown"}
	values := []any{int64(9), "POINT (1 2)", blob, stamp, float64(1), float64(7), int64(1), complex(1, 2)}
	layer := &Layer{
		name: "test", idFieldname: "fid", geomFieldname: "geom",
		geometryFormat: codec.FormatWKT, srid: 4326,
		bboxFields: codec.BBoxFields{"bound_x", "bound_y", "bound_x2", "bound_y2"},
	}
	before := *layer
	columnsBefore := append([]string{}, columns...)
	valuesBefore := append([]any{}, values...)
	valuesBefore[2] = append([]byte{}, blob...)
	p := &Provider{}
	decoded, err := p.decodeRow(context.Background(), layer, columns, values, rowDecodePolicy{})
	feature, representable := decoded.Feature, decoded.Representable
	if err != nil || !representable || feature.Geometry == nil || feature.ID != 9 || feature.SRID != 4326 {
		t.Fatalf("unexpected decoded feature: %+v, representable=%v, err=%v", feature, representable, err)
	}
	wantTags := map[string]any{"blob": base64.StdEncoding.EncodeToString(blob), "stamp": stamp.Format(time.RFC3339), "minx": float64(7)}
	if !reflect.DeepEqual(feature.Tags, wantTags) {
		t.Fatalf("tags=%v, want=%v", feature.Tags, wantTags)
	}
	if !reflect.DeepEqual(*layer, before) || !reflect.DeepEqual(columns, columnsBefore) || !reflect.DeepEqual(values, valuesBefore) {
		t.Fatal("decoder mutated inputs or resolved layer")
	}
	if _, warned := p.warned["unexpected-column:unknown"]; !warned {
		t.Fatal("unsupported tag warning missing")
	}
}

func TestDecodeRowNullEmptyAndResolvedSRID(t *testing.T) {
	for _, srid := range []uint64{0, 4326} {
		for _, geometry := range []any{nil, gpkgHeader(1<<4, 26910), append(gpkgHeader(1, 26910), wkbPointLE...)} {
			layer := &Layer{name: "test", idFieldname: "fid", geomFieldname: "geom", srid: srid}
			p := &Provider{}
			decoded, err := p.decodeRow(context.Background(), layer, []string{"geom", "fid"}, []any{geometry, int64(3)}, rowDecodePolicy{})
			feature, representable := decoded.Feature, decoded.Representable
			if err != nil || !representable {
				t.Fatalf("representable=%v, err=%v", representable, err)
			}
			wantSRID := srid
			if wantSRID == 0 {
				wantSRID = DefaultSRID
			}
			if feature.SRID != wantSRID || layer.srid != srid {
				t.Fatalf("header overrode resolved SRID: feature=%d, layer=%d", feature.SRID, layer.srid)
			}
			if geometry == nil && feature.Geometry != nil {
				t.Fatal("null geometry became non-null")
			}
			if raw, ok := geometry.([]byte); ok {
				isEmpty := raw[3]&(1<<4) != 0
				if isEmpty != (feature.Geometry == nil) {
					t.Fatalf("empty-header geometry policy changed: empty=%v, geometry=%v", isEmpty, feature.Geometry)
				}
			}
		}
	}
}

func TestDecodeRowMOSAndCancellation(t *testing.T) {
	metadata := make([]byte, 64)
	metadata[0] = 5
	copy(metadata[1:], "Ver 1")
	for _, tc := range []struct {
		name   string
		format string
		values []any
		key    string
	}{
		{name: "metadata", format: codec.FormatMOS, values: []any{metadata, int64(2)}, key: "late-system-info:test"},
		{name: "null id", format: codec.FormatWKT, values: []any{"POINT (1 2)", nil}, key: "null-feature-id:test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layer := &Layer{name: "test", idFieldname: "fid", geomFieldname: "geom", geometryFormat: tc.format}
			before := *layer
			p := &Provider{}
			decoded, err := p.decodeRow(context.Background(), layer, []string{"geom", "fid"}, tc.values, rowDecodePolicy{})
			representable := decoded.Representable
			if err != nil || representable || !reflect.DeepEqual(*layer, before) {
				t.Fatalf("skip/mutation policy: representable=%v, err=%v", representable, err)
			}
			if _, warned := p.warned[tc.key]; !warned {
				t.Fatalf("missing warning %q", tc.key)
			}
		})
	}
	for _, sampled := range []bool{false, true} {
		layer := &Layer{name: "test", idFieldname: "fid", geomFieldname: "geom", geometryFormat: codec.FormatMOS}
		if sampled {
			layer.mapplSource = codec.MapplGISSQLSample
		}
		p := &Provider{}
		decoded, err := p.decodeRow(context.Background(), layer, []string{"geom", "fid"}, []any{[]byte{0xff}, int64(2)}, rowDecodePolicy{tolerateAutoDetectedMOS: true})
		representable := decoded.Representable
		if sampled {
			if err != nil || representable {
				t.Fatalf("sample malformed MOS policy: representable=%v, err=%v", representable, err)
			}
		} else if err == nil {
			t.Fatal("explicit malformed MOS did not fail")
		}
		if sampled {
			_, err = p.decodeRow(context.Background(), layer, []string{"geom", "fid"}, []any{[]byte{0xff}, int64(-1)}, rowDecodePolicy{tolerateAutoDetectedMOS: true})
			if err == nil {
				t.Fatal("sample geometry skip hid invalid ID")
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (&Provider{}).decodeRow(ctx, &Layer{}, []string{}, []any{}, rowDecodePolicy{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation chain lost: %v", err)
	}
	deadlineCtx, deadlineCancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer deadlineCancel()
	_, err = (&Provider{}).decodeRow(deadlineCtx, &Layer{}, []string{}, []any{}, rowDecodePolicy{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline chain lost: %v", err)
	}
}

func TestDecodeRowStrictAutoDetectedMOS(t *testing.T) {
	layer := &Layer{
		name: "test", idFieldname: "fid", geomFieldname: "geom",
		geometryFormat: codec.FormatMOS, mapplSource: codec.MapplGISSQLSample,
	}
	malformed := []byte{0xff}
	_, _, expectedErr := decodeGeometryValue(malformed, layer.geometryFormat, layer.mosConfig)
	_, err := (&Provider{}).decodeRow(
		context.Background(),
		layer,
		[]string{"geom", "fid"},
		[]any{malformed, int64(2)},
		rowDecodePolicy{},
	)
	if expectedErr == nil || err == nil || err.Error() != expectedErr.Error() {
		t.Fatalf("strict policy discarded original decode error: got=%v, want=%v", err, expectedErr)
	}
	// Strict mode preserves column-order precedence rather than classifying the
	// malformed geometry as metadata or a NULL-ID skip.
	_, err = (&Provider{}).decodeRow(
		context.Background(),
		layer,
		[]string{"geom", "fid"},
		[]any{malformed, int64(-1)},
		rowDecodePolicy{},
	)
	if err == nil || err.Error() != expectedErr.Error() {
		t.Fatalf("strict first-error precedence changed: %v", err)
	}
	validLayer := &Layer{name: "valid", idFieldname: "fid", geomFieldname: "geom", geometryFormat: codec.FormatWKT}
	var previous decodedRow
	for _, policy := range []rowDecodePolicy{{}, {tolerateAutoDetectedMOS: true}} {
		decoded, decodeErr := (&Provider{}).decodeRow(
			context.Background(),
			validLayer,
			[]string{"fid", "geom"},
			[]any{int64(2), "POINT (1 2)"},
			policy,
		)
		if decodeErr != nil || !decoded.Representable {
			t.Fatalf("valid geometry rejected: %+v, %v", decoded, decodeErr)
		}
		if previous.Representable && !reflect.DeepEqual(previous, decoded) {
			t.Fatal("valid geometry depends on decode policy")
		}
		previous = decoded
	}
}
