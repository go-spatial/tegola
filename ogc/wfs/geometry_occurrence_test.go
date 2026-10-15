package wfs

import (
	"encoding/xml"
	"fmt"
	"testing"
)

func TestPR3GeometrySchemaOccurrence(t *testing.T) {
	for _, version := range []Version{V110, V200, V202} {
		for _, nullable := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/nullable=%v", version, nullable), func(t *testing.T) {
				view := &FeatureSchemaView{GeometryName: "geom", GeometryXSDType: "PointPropertyType", GeometryNullable: nullable}
				var schema struct {
					Elements []struct {
						Name      string `xml:"name,attr"`
						MinOccurs string `xml:"minOccurs,attr"`
						MaxOccurs string `xml:"maxOccurs,attr"`
						Nillable  bool   `xml:"nillable,attr"`
					} `xml:"complexType>complexContent>extension>sequence>element"`
				}
				if err := xml.Unmarshal([]byte(DescribeFeatureType(version, "sites", view)), &schema); err != nil {
					t.Fatal(err)
				}
				if len(schema.Elements) != 1 {
					t.Fatalf("elements=%#v", schema.Elements)
				}
				wantMin := "1"
				if nullable {
					wantMin = "0"
				}
				got := schema.Elements[0]
				if got.Name != "geom" || got.MinOccurs != wantMin || got.MaxOccurs != "1" || got.Nillable != nullable {
					t.Fatalf("geometry declaration=%#v; want minOccurs=%s, nillable=%v", got, wantMin, nullable)
				}
			})
		}
	}
}
