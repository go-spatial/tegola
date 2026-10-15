package mysql

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

func TestMOSFeatureBoundsReachCandidateSQL(t *testing.T) {
	p, store := newFeatureTestProvider(t, nil)
	f := store.profile
	f.format = GeometryFormatMOS
	f.bounds = [4]string{"MINX", "MAXX", "MINY", "MAXY"}
	f.mos = codec.MOSConfig{UnitFactor: 0.001}
	for _, field := range f.bounds {
		f.schema.columns = append(f.schema.columns, featureColumn{name: field, dataType: "int"})
	}
	_, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{
		Limit: 2, BoundsSRID: 4326, Bounds: []geom.Extent{{1, 2, 3, 4}},
	}, func(*provider.Feature) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range store.queries {
		if strings.HasSuffix(statement, "LIMIT 256") {
			if !strings.Contains(statement, "l.`MAXX`>=?") || !strings.Contains(statement, "l.`MINY`<=?") {
				t.Fatalf("candidate SQL omitted bounds: %s", statement)
			}
			return
		}
	}
	t.Fatal("candidate SQL not executed")
}

func TestMOSFeatureBoundsAdmission(t *testing.T) {
	fields := [4]string{"MINX", "MAXX", "MINY", "MAXY"}
	conf := dict.Dict{"bbox_minx_fieldname": "MINX", "bbox_maxx_fieldname": "MAXX", "bbox_miny_fieldname": "MINY", "bbox_maxy_fieldname": "MAXY"}
	for _, mode := range []string{"valid", "missing config", "missing column", "text column", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			f := testFeatureProfile(t, false)
			f.format = GeometryFormatMOS
			for _, name := range fields {
				f.schema.columns = append(f.schema.columns, featureColumn{name: name, dataType: "int"})
			}
			layer := Layer{bboxFields: fields}
			c := dict.Dict{}
			for k, v := range conf {
				c[k] = v
			}
			switch mode {
			case "missing config":
				delete(c, "bbox_minx_fieldname")
			case "missing column":
				f.schema.columns = f.schema.columns[:len(f.schema.columns)-1]
			case "text column":
				f.schema.columns[len(f.schema.columns)-1].dataType = "text"
			case "duplicate":
				layer.bboxFields[1] = layer.bboxFields[0]
			}
			f.registerFeatureBounds(layer, c)
			if got := f.bounds[0] != ""; got != (mode == "valid") {
				t.Fatalf("bounds admitted=%v", got)
			}
		})
	}
}

func TestMOSFeatureBoundsSameDefinitionAndRounding(t *testing.T) {
	f := testFeatureProfile(t, false)
	f.format, f.crsDeclared = GeometryFormatMOS, true
	f.freezeFeatureCRS()
	f.bounds = [4]string{"MINX", "MAXX", "MINY", "MAXY"}
	f.mos = codec.MOSConfig{Precision: 0, UnitFactor: 0.001}
	q := provider.FeatureQuery{BoundsSRID: 4326, BoundsCRSDefinition: f.crs.Definition,
		Bounds: []geom.Extent{{1.0001, -2.0001, 1.9999, -1.0001}, {3, 4, 5, 6}}}
	sql, args := f.boundsCandidate(q)
	if !reflect.DeepEqual(args, []any{float64(1000), float64(2000), float64(-2001), float64(-1000), float64(3000), float64(5000), float64(4000), float64(6000)}) {
		t.Fatalf("raw outward bounds: %v", args)
	}
	if strings.Count(sql, "l.`MAXX`>=?") != 2 || !strings.Contains(sql, ") OR (") {
		t.Fatalf("bounds not ORed: %s", sql)
	}
	for _, fallback := range []string{"l.`MINX` IS NULL", "l.`geom` IS NULL", "LENGTH(l.`geom`)<18", "SUBSTRING(l.`geom`,5,2)", "SUBSTRING(l.`geom`,7,4)", "l.`MINX`>l.`MAXX`"} {
		if !strings.Contains(sql, fallback) {
			t.Errorf("missing conservative fallback %s", fallback)
		}
	}
	q.BoundsCRSDefinition, _ = crsconfig.CanonicalFeatureDefinition(3857)
	if got, _ := f.boundsCandidate(q); got != "" {
		t.Fatal("same numeric SRID with different definition pruned")
	}
	q.BoundsCRSDefinition = ""
	q.BoundsSRID = 3857
	if got, _ := f.boundsCandidate(q); got != "" {
		t.Fatal("different CRS pruned")
	}
	q.BoundsSRID = f.srid
	f.crs.CanonicalAuthority = ""
	if got, _ := f.boundsCandidate(q); got != "" {
		t.Fatal("custom definition admitted by numeric ID alone")
	}
	q.BoundsCRSDefinition = f.crs.Definition
	f.bounds = [4]string{}
	if got, _ := f.boundsCandidate(q); got != "" {
		t.Fatal("unconfigured bounds inferred")
	}
}
