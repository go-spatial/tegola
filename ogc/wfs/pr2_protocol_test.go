package wfs

import (
	"context"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/ogc/features"
)

func TestPR2AdvertisedQualifiedType(t *testing.T) {
	for _, v := range []Version{V110, V200, V202} {
		req, ex := ParseGetFeatureKVP(v, map[string]string{"typenames": "app:sites", "featureid": "sites.1", "propertyname": "app:rank", "sortby": "app:rank A"})
		if len(ex) > 0 || req.TypeName != "sites" {
			t.Fatalf("version %s: %#v, %v", v, req, ex)
		}
	}
	for _, name := range []string{"unknown:sites", "app:sites:extra", ":sites", "app:", "app:bad name"} {
		if _, ex := ParseGetFeatureKVP(V200, map[string]string{"typenames": name}); len(ex) == 0 {
			t.Errorf("accepted %q", name)
		}
	}
	if _, ex := ParseGetFeatureKVP(V200, map[string]string{"typenames": "app:sites", "namespaces": "xmlns(app,http://foreign.example/sites)"}); len(ex) == 0 {
		t.Error("accepted foreign namespace")
	}
}

func TestPR2QualifiedFESProperty(t *testing.T) {
	filter := `<fes:Filter xmlns:fes="http://www.opengis.net/fes/2.0" xmlns:app="http://example.com/tegola/sites"><fes:PropertyIsEqualTo><fes:ValueReference>app:rank</fes:ValueReference><fes:Literal>10</fes:Literal></fes:PropertyIsEqualTo></fes:Filter>`
	req, ex := ParseGetFeatureKVP(V200, map[string]string{"typename": "sites", "filter": filter})
	if len(ex) > 0 {
		t.Fatal(ex)
	}
	if req.Filter.Root().Property != "rank" {
		t.Fatalf("qualified name leaked into neutral query: %q", req.Filter.Root().Property)
	}
	for _, bad := range []string{strings.ReplaceAll(filter, "http://example.com/tegola/sites", "http://foreign.example/sites"), strings.ReplaceAll(filter, "app:rank", "missing:rank"), strings.ReplaceAll(filter, "app:rank", "app:rank:extra")} {
		if _, ex := ParseGetFeatureKVP(V200, map[string]string{"typename": "sites", "filter": bad}); len(ex) == 0 {
			t.Errorf("accepted invalid FES QName: %s", bad)
		}
	}
	between := `<Filter xmlns:p="http://example.com/tegola/sites"><PropertyIsBetween><PropertyName>p:rank</PropertyName><LowerBoundary><Literal>1</Literal></LowerBoundary><UpperBoundary><Literal>20</Literal></UpperBoundary></PropertyIsBetween></Filter>`
	query, ex := ParseGetFeatureKVP(V110, map[string]string{"typename": "app:sites", "filter": between})
	if len(ex) > 0 {
		t.Fatal(ex)
	}
	for _, child := range query.Filter.Root().Children {
		if child.Property != "rank" {
			t.Fatalf("unresolved Between property: %#v", child)
		}
	}
	unbound := strings.ReplaceAll(filter, ` xmlns:app="http://example.com/tegola/sites"`, "")
	if _, ex := ParseGetFeatureKVP(V200, map[string]string{"typename": "app:sites", "filter": unbound}); len(ex) == 0 {
		t.Fatal("accepted undeclared XML app prefix")
	}
}

func TestPR2VersionsAndCapabilities(t *testing.T) {
	src := reviewQuerySource{}
	svc, err := features.NewService([]features.CollectionSource{{ID: "sites", Layer: src, Querier: src}})
	if err != nil {
		t.Fatal(err)
	}
	req, ex := ParseGetPropertyValueKVP(V202, map[string]string{"typename": "sites", "valuereference": "rank"})
	if len(ex) > 0 {
		t.Error(ex)
	} else if _, ex := ExecuteGetPropertyValue(context.Background(), svc, req); len(ex) > 0 {
		t.Error(ex)
	}
	for _, v := range []Version{V200, V202} {
		var response struct {
			Version string `xml:"version,attr"`
		}
		if err := xml.Unmarshal([]byte(TransactionResponse(v, nil)), &response); err != nil {
			t.Fatal(err)
		}
		if response.Version != string(v) {
			t.Errorf("%s response version=%s", v, response.Version)
		}
	}
	for _, v := range []Version{V110, V200} {
		raw := Capabilities(v, svc, "http://example.com/wfs", nil)
		if strings.Contains(raw, `SpatialOperator name="BBOX"`) {
			t.Errorf("version %s advertises rejected XML BBOX", v)
		}
	}
}

func TestPR2TransactionQNameBindings(t *testing.T) {
	valid := `<wfs:Transaction xmlns:wfs="http://www.opengis.net/wfs/2.0" xmlns:app="http://example.com/tegola/sites" xmlns:fes="http://www.opengis.net/fes/2.0" version="2.0.0"><wfs:Update typeName="app:sites"><wfs:Property><wfs:ValueReference>app:rank</wfs:ValueReference><wfs:Value>12</wfs:Value></wfs:Property><fes:Filter><fes:ResourceId rid="app:sites.1"/></fes:Filter></wfs:Update></wfs:Transaction>`
	actions, _, _, err := ParseTransaction(V200, []byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].TypeName != "sites" || actions[0].Properties["rank"] != "12" {
		t.Fatalf("unresolved action: %#v", actions)
	}
	for _, bad := range []string{
		strings.ReplaceAll(valid, `xmlns:app="http://example.com/tegola/sites"`, `xmlns:app="http://foreign.example/sites"`),
		strings.ReplaceAll(valid, "app:sites", "missing:sites"),
		strings.ReplaceAll(valid, "app:rank", "missing:rank"),
		strings.ReplaceAll(valid, "app:rank", "app:rank:extra"),
		strings.ReplaceAll(valid, "wfs:ValueReference", "evil:ValueReference"),
		strings.ReplaceAll(valid, "fes:ResourceId", "evil:ResourceId"),
		strings.ReplaceAll(valid, `<wfs:ValueReference>`, `<wfs:ValueReference xmlns:app="http://foreign.example/sites">`),
	} {
		if _, _, _, err := ParseTransaction(V200, []byte(bad)); err == nil {
			t.Errorf("accepted invalid binding: %s", bad)
		}
	}
	insert := `<wfs:Transaction xmlns:wfs="http://www.opengis.net/wfs/2.0" xmlns:app="http://foreign.example/sites"><wfs:Insert><app:sites><app:rank>12</app:rank></app:sites></wfs:Insert></wfs:Transaction>`
	if _, _, _, err := ParseTransaction(V200, []byte(insert)); err == nil {
		t.Error("accepted foreign Insert namespace")
	}
	plain := `<Transaction><Insert><sites><Name> name with spaces </Name><ValueReference>app:literal</ValueReference></sites></Insert></Transaction>`
	parsed, _, _, err := ParseTransaction(V200, []byte(plain))
	if err != nil || parsed[0].Properties["Name"] != " name with spaces " || parsed[0].Properties["ValueReference"] != "app:literal" {
		t.Fatalf("feature values interpreted as protocol refs: %#v %v", parsed, err)
	}
	multiple := `<wfs:Transaction xmlns:wfs="http://www.opengis.net/wfs/2.0" xmlns:a="http://example.com/tegola/sites" xmlns:b="http://example.com/tegola/roads"><wfs:Insert><a:sites><a:rank>1</a:rank></a:sites><b:roads><b:rank>2</b:rank></b:roads></wfs:Insert></wfs:Transaction>`
	parsed, _, _, err = ParseTransaction(V200, []byte(multiple))
	if err != nil || len(parsed) != 2 || parsed[0].TypeName != "sites" || parsed[1].TypeName != "roads" {
		t.Fatalf("multi-namespace Insert: %#v %v", parsed, err)
	}
}

func TestPR2KVPBindingsAndStoredQuery(t *testing.T) {
	for _, binding := range []string{"xmlns(t,http://example.com/tegola/sites)", "xmlns(t=http://example.com/tegola/sites)"} {
		q := map[string]string{"typenames": "t:sites", "namespaces": binding, "resourceid": "t:sites.2", "propertyname": "t:rank", "sortby": "t:rank D"}
		req, ex := ParseGetFeatureKVP(V202, q)
		if len(ex) > 0 {
			t.Fatal(ex)
		}
		if req.TypeName != "sites" || req.FeatureIDs[0] != 2 || req.PropertyNames[0] != "rank" || req.SortBy[0].Property != "rank" {
			t.Fatalf("bad normalized request: %#v", req)
		}
	}
	req, ex := ParseGetFeatureKVP(V202, map[string]string{"storedquery_id": "urn:ogc:def:query:OGC-WFS::GetFeatureById", "typename": "app:sites", "id": "app:sites.2"})
	if len(ex) > 0 || req.TypeName != "sites" || req.Version != V202 || req.FeatureIDs[0] != 2 {
		t.Fatalf("stored query: %#v %v", req, ex)
	}
	for _, field := range []string{"propertyname", "sortby", "valuereference"} {
		q := map[string]string{"typename": "sites", field: "unknown:rank"}
		if field == "valuereference" {
			if _, ex := ParseGetPropertyValueKVP(V202, q); len(ex) == 0 {
				t.Errorf("accepted unknown %s prefix", field)
			}
			continue
		}
		if _, ex := ParseGetFeatureKVP(V200, q); len(ex) == 0 {
			t.Errorf("accepted unknown %s prefix", field)
		}
	}
}

func TestPR2GeometrySchemaNullability(t *testing.T) {
	for _, nullable := range []bool{false, true} {
		view := &FeatureSchemaView{GeometryName: "geom", GeometryXSDType: "PointPropertyType", GeometryNullable: nullable}
		var schema struct {
			Types []struct {
				Content struct {
					Extension struct {
						Sequence struct {
							Elements []struct {
								Name     string `xml:"name,attr"`
								Nillable bool   `xml:"nillable,attr"`
							} `xml:"element"`
						} `xml:"sequence"`
					} `xml:"extension"`
				} `xml:"complexContent"`
			} `xml:"complexType"`
		}
		if err := xml.Unmarshal([]byte(DescribeFeatureType(V200, "sites", view)), &schema); err != nil {
			t.Fatal(err)
		}
		got := schema.Types[0].Content.Extension.Sequence.Elements[0]
		if got.Name != "geom" || got.Nillable != nullable {
			t.Fatalf("nullable=%v got %#v", nullable, got)
		}
	}
}
