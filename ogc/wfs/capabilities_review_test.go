package wfs

import (
	"encoding/xml"
	"github.com/alexeydott/tegola/ogc/features"
	"strings"
	"testing"
)

func TestReviewCapabilitiesNamespaces(t *testing.T) {
	src := reviewQuerySource{}
	svc, err := features.NewService([]features.CollectionSource{{ID: "sites", Layer: src, Querier: src}})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []Version{V110, V200, V202} {
		var doc struct {
			XMLName xml.Name
			Version string `xml:"version,attr"`
			List    struct {
				XMLName xml.Name
				Types   []struct {
					XMLName xml.Name
					Name    struct{ XMLName xml.Name } `xml:"Name"`
					SRS     struct{ XMLName xml.Name } `xml:",any"`
				} `xml:"FeatureType"`
			} `xml:"FeatureTypeList"`
		}
		raw := Capabilities(v, svc, "http://example.com/wfs", nil)
		if err := xml.Unmarshal([]byte(raw), &doc); err != nil {
			t.Fatal(err)
		}
		ns := "http://www.opengis.net/wfs/2.0"
		if v == V110 {
			ns = "http://www.opengis.net/wfs"
		}
		if doc.Version != string(v) || doc.List.XMLName.Space != ns || len(doc.List.Types) != 1 || doc.List.Types[0].XMLName.Space != ns || doc.List.Types[0].Name.XMLName.Space != ns {
			t.Errorf("%s wrong namespace/version: %s", v, raw)
		}
		if v == V110 && !strings.Contains(raw, "DefaultSRS") {
			t.Error("missing 1.1 DefaultSRS")
		}
	}
}

func TestReviewSchemaMatchesEncoder(t *testing.T) {
	view := &FeatureSchemaView{GeometryName: "geom", GeometryXSDType: "GeometryPropertyType", Properties: []SchemaPropView{{Name: "z", Type: "string"}, {Name: "a", Type: "string", Nullable: true}}}
	for _, v := range []Version{V110, V200} {
		raw := DescribeFeatureType(v, "sites", view)
		var document struct{ XMLName xml.Name }
		if err := xml.Unmarshal([]byte(raw), &document); err != nil {
			t.Fatal(err)
		}
		if document.XMLName.Space != "http://www.w3.org/2001/XMLSchema" || document.XMLName.Local != "schema" {
			t.Fatalf("invalid schema root: %v", document.XMLName)
		}
		if v == V110 && !strings.Contains(raw, `substitutionGroup="gml:_Feature"`) {
			t.Error("incorrect 1.1 substitution group")
		}
		if !(strings.Index(raw, `name="geom"`) < strings.Index(raw, `name="a"`) && strings.Index(raw, `name="a"`) < strings.Index(raw, `name="z"`)) {
			t.Error("schema order differs from encoder")
		}
	}
}

func TestReviewSchemaRejectsInvalidXMLNames(t *testing.T) {
	for _, name := range []string{"9name", "bad name", "bad:name", "geom"} {
		view := &FeatureSchemaView{GeometryName: "geom", Properties: []SchemaPropView{{Name: name}}}
		if err := ValidateSchemaView("sites", view); err == nil {
			t.Fatalf("accepted property %q", name)
		}
	}
	view := &FeatureSchemaView{GeometryName: "geom", GeometryXSDType: "MultiPolygonPropertyType"}
	if !strings.Contains(DescribeFeatureType(V200, "sites", view), `type="gml:MultiSurfacePropertyType"`) {
		t.Fatal("schema type disagrees with multipart encoder")
	}
}
