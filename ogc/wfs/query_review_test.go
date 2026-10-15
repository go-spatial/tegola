package wfs

import (
	"context"
	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/ogc/wfs/gml"
	"github.com/alexeydott/tegola/provider"
	"strings"
	"testing"
)

func TestReviewFESRejectsAmbiguousStructure(t *testing.T) {
	for _, s := range []string{
		`<Other><PropertyIsEqualTo><ValueReference>x</ValueReference><Literal>1</Literal></PropertyIsEqualTo></Other>`,
		`<Filter><PropertyIsEqualTo><ValueReference>x</ValueReference><Literal>1</Literal></PropertyIsEqualTo><PropertyIsEqualTo><ValueReference>x</ValueReference><Literal>2</Literal></PropertyIsEqualTo></Filter>`,
		`<Filter><PropertyIsEqualTo><ValueReference>x</ValueReference></PropertyIsEqualTo></Filter>`,
		`<Filter><PropertyIsEqualTo><ValueReference>x</ValueReference><Literal>1</Literal><Literal>2</Literal></PropertyIsEqualTo></Filter>`,
		`<Filter><PropertyIsEqualTo matchCase="false"><ValueReference>x</ValueReference><Literal>A</Literal></PropertyIsEqualTo></Filter>`,
		`<Filter><PropertyIsNull><ValueReference>x</ValueReference></PropertyIsNull></Filter>`,
		`<Filter><PropertyIsEqualTo><ValueReference>x</ValueReference><Literal>` + strings.Repeat("x", 17000) + `</Literal></PropertyIsEqualTo></Filter>`,
	} {
		t.Run(s[:min(len(s), 80)], func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("untrusted filter panicked: %v", r)
				}
			}()
			if _, err := ParseFESFilter([]byte(s)); err == nil {
				t.Fatal("accepted unsafe filter")
			}
		})
	}
}
func TestReviewFIDCollectionBinding(t *testing.T) {
	_, ex := ParseGetFeatureKVP(V200, map[string]string{"typenames": "sites", "featureid": "other.1"})
	if len(ex) == 0 {
		t.Fatal("accepted another collection's FID")
	}
}

type reviewQuerySource struct{}

func (reviewQuerySource) TemporalMapping() (provider.TemporalMapping, error) {
	return provider.TemporalMapping{}, nil
}

func (reviewQuerySource) Name() string                 { return "sites" }
func (reviewQuerySource) SRID() uint64                 { return 4326 }
func (reviewQuerySource) GeomType() geom.Geometry      { return geom.Point{} }
func (reviewQuerySource) FeatureQuerySupported() error { return nil }
func (reviewQuerySource) SpatialMetadata() (provider.SpatialMetadata, error) {
	return provider.SpatialMetadata{Dimension: provider.DimensionXY}, nil
}
func (reviewQuerySource) DescribeSchema(context.Context, string) (provider.SchemaDescriptor, error) {
	return provider.SchemaDescriptor{IDColumn: "fid", Columns: []provider.ColumnDescriptor{{Name: "rank", Type: "INTEGER"}}, Geometry: provider.GeometryColumnDescriptor{Name: "geom", SRID: 4326}}, nil
}
func (reviewQuerySource) QueryFeatures(_ context.Context, _ string, q provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	rows := []int{30, 10, 20}
	n := uint64(len(rows))
	r := provider.FeatureQueryResult{NumberMatched: &n}
	for i, v := range rows {
		if uint64(i) < q.Offset || r.NumberReturned >= uint64(q.Limit) {
			continue
		}
		if err := fn(&provider.Feature{ID: uint64(i + 1), SRID: 4326, Geometry: geom.Point{1, 2}, Tags: map[string]any{"rank": v}}); err != nil {
			return r, err
		}
		r.NumberReturned++
	}
	return r, nil
}
func TestReviewGetFeaturePagingAndHits(t *testing.T) {
	src := reviewQuerySource{}
	svc, err := features.NewService([]features.CollectionSource{{ID: "sites", Layer: src, Querier: src}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		req    GetFeatureRequest
		want   []string
		absent string
	}{
		{"sort before page", GetFeatureRequest{Version: V200, TypeName: "sites", MaxFeatures: 1, StartIndex: 1, SortBy: []SortCriterion{{Property: "rank"}}}, []string{`numberMatched="3"`, `numberReturned="1"`, `sites.3`}, `sites.2`},
		{"unsorted offset", GetFeatureRequest{Version: V200, TypeName: "sites", MaxFeatures: 1, StartIndex: 1}, []string{`numberMatched="3"`, `numberReturned="1"`, `sites.2`}, `sites.1`},
		{"hits ignore page", GetFeatureRequest{Version: V200, TypeName: "sites", MaxFeatures: 1, StartIndex: 2, ResultType: "hits"}, []string{`numberMatched="3"`, `numberReturned="0"`}, `sites.1`},
		{"zero count", GetFeatureRequest{Version: V200, TypeName: "sites", MaxFeatures: 0}, []string{`numberReturned="0"`}, `sites.1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, ex := ExecuteGetFeature(context.Background(), svc, &tc.req)
			if len(ex) > 0 {
				t.Fatal(ex)
			}
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Fatalf("missing %s in %s", w, out)
				}
			}
			if strings.Contains(out, tc.absent) {
				t.Fatalf("unexpected %s in %s", tc.absent, out)
			}
		})
	}
}

func TestReviewResourceIDsAndBBox(t *testing.T) {
	for _, q := range []map[string]string{
		{"typenames": "sites", "resourceid": "other.1"},
		{"typenames": "sites", "bbox": "NaN,0,1,1"},
		{"typenames": "sites", "bbox": "2,0,1,1"},
		{"typenames": "sites", "featureid": "sites.1", "bbox": "0,0,1,1"},
	} {
		if _, ex := ParseGetFeatureKVP(V200, q); len(ex) == 0 {
			t.Fatalf("accepted %v", q)
		}
	}
	req, ex := ParseGetFeatureKVP(V200, map[string]string{"typenames": "sites", "resourceid": "sites.2"})
	if len(ex) > 0 || len(req.FeatureIDs) != 1 || req.FeatureIDs[0] != 2 {
		t.Fatalf("resourceid: %v %v", req, ex)
	}
}
func TestReviewFESPreservesLiteralText(t *testing.T) {
	f, err := ParseFESFilter([]byte(`<fes:Filter xmlns:fes="http://www.opengis.net/fes/2.0"><fes:PropertyIsEqualTo><fes:ValueReference>name</fes:ValueReference><fes:Literal>  padded  </fes:Literal></fes:PropertyIsEqualTo></fes:Filter>`))
	if err != nil {
		t.Fatal(err)
	}
	if f.Root().Literal.Text() != "  padded  " {
		t.Fatal("literal whitespace lost")
	}
}

func TestReviewSortPreservesNumericPrecisionAndTextType(t *testing.T) {
	fs := []gml.Feature{{ID: "larger", SortKeys: []string{"9007199254740993"}}, {ID: "smaller", SortKeys: []string{"9007199254740992"}}}
	sortFeatures(fs, []SortCriterion{{Property: "rank", numeric: true}})
	if fs[0].ID != "smaller" {
		t.Fatal("numeric precision lost")
	}
	fs = []gml.Feature{{ID: "two", SortKeys: []string{"2"}}, {ID: "ten", SortKeys: []string{"10"}}}
	sortFeatures(fs, []SortCriterion{{Property: "name"}})
	if fs[0].ID != "ten" {
		t.Fatal("text sorted as numbers")
	}
}
