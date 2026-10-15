package gml

import (
	"encoding/xml"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
)

func TestGeometryStructuralBoundaries(t *testing.T) {
	cases := []struct {
		name, raw string
		want      geom.Geometry
	}{
		{"two holes", `<Polygon><exterior><LinearRing><posList>0 0 10 0 10 10 0 0</posList></LinearRing></exterior><interior><LinearRing><posList>1 1 2 1 2 2 1 1</posList></LinearRing></interior><interior><LinearRing><posList>3 3 4 3 4 4 3 3</posList></LinearRing></interior></Polygon>`, geom.Polygon{{{0, 0}, {10, 0}, {10, 10}, {0, 0}}, {{1, 1}, {2, 1}, {2, 2}, {1, 1}}, {{3, 3}, {4, 3}, {4, 4}, {3, 3}}}},
		{"multiline repeated pos", `<MultiLineString><lineStringMember><LineString><pos>1 2</pos><pos>3 4</pos></LineString></lineStringMember><lineStringMember><LineString><pos>5 6</pos><pos>7 8</pos></LineString></lineStringMember></MultiLineString>`, geom.MultiLineString{{{1, 2}, {3, 4}}, {{5, 6}, {7, 8}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseGeometry(tc.raw, "")
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, %v; want %#v", got, err, tc.want)
			}
		})
	}
	for _, raw := range []string{
		`<Point><pos>1 2</pos></Point><Point><pos>3 4</pos></Point>`,
		`<LineString><Curve><posList>1 2 3 4</posList></Curve></LineString>`,
		`<Polygon><posList>0 0 1 0 1 1 0 0</posList></Polygon>`,
		`<LineString><pos>1 2 3 4</pos></LineString>`,
		`<LineString><posList>1 2 3 4</posList><posList>5 6 7 8</posList></LineString>`,
	} {
		if got, err := ParseGeometry(raw, ""); err == nil {
			t.Errorf("accepted malformed geometry %s: %#v", raw, got)
		}
	}
}

func TestFeatureOutputNamespacesAndNull(t *testing.T) {
	for _, v := range []Version{V311, V321} {
		e := Encoder{Version: v, SRID: 4326}
		if err := e.EncodeFeature(Feature{ID: "sites.1", TypeName: "sites", GeometryName: "geom", Properties: map[string]interface{}{"empty": "", "null": nil}}); err != nil {
			t.Fatal(err)
		}
		dec := xml.NewDecoder(strings.NewReader(`<root xmlns:gml="http://www.opengis.net/gml" xmlns:wfs="http://www.opengis.net/wfs/2.0">` + e.String() + `</root>`))
		seenNull := false
		for {
			tok, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if start, ok := tok.(xml.StartElement); ok {
				if start.Name.Local == "sites" || start.Name.Local == "empty" || start.Name.Local == "null" {
					if start.Name.Space != "http://example.com/tegola/sites" {
						t.Fatalf("wrong app namespace: %v", start.Name)
					}
				}
				if start.Name.Local == "null" {
					for _, a := range start.Attr {
						if a.Name.Space == "http://www.w3.org/2001/XMLSchema-instance" && a.Name.Local == "nil" && a.Value == "true" {
							seenNull = true
						}
					}
				}
			}
		}
		if !seenNull {
			t.Fatal("NULL not encoded with xsi:nil")
		}
	}
	for _, name := range []string{"9name", "bad name", "bad:name", "x><evil"} {
		e := Encoder{}
		if err := e.EncodeFeature(Feature{TypeName: "sites", GeometryName: "geom", Properties: map[string]interface{}{name: "value"}}); err == nil {
			t.Fatalf("accepted invalid NCName %q", name)
		}
	}
}

func TestGeometryCRSContracts(t *testing.T) {
	for _, v := range []Version{V311, V321} {
		e := Encoder{Version: v, SRID: 4326}
		want := geom.Point{37, 55}
		if err := e.EncodeGeometry(want); err != nil {
			t.Fatal(err)
		}
		got, srid, err := ParseGeometryWithSRID(e.String(), "")
		if err != nil || srid != 4326 || !reflect.DeepEqual(got, want) {
			t.Fatalf("version %v: got %#v SRID %d err %v", v, got, srid, err)
		}
	}
	for _, srs := range []string{"EPSG:9999", "urn:ogc:def:crs:EPSG::14326"} {
		if _, _, err := ParseGeometryWithSRID(`<Point srsName="`+srs+`"><pos>1 2</pos></Point>`, ""); err == nil {
			t.Fatalf("accepted %s", srs)
		}
	}
	if _, _, err := ParseGeometryWithSRID(`<Point srsName="EPSG:3857"><pos srsName="EPSG:4326">1 2</pos></Point>`, ""); err == nil {
		t.Fatal("accepted mixed CRS")
	}
}
