package wfs

import (
	"fmt"
	"github.com/alexeydott/tegola/ogc/wfs/gml"
	"sort"
	"strings"

	"github.com/alexeydott/tegola/ogc/features"
)

// Capabilities renders WFS GetCapabilities for the version.
func Capabilities(v Version, service *features.Service, baseURL string, writeOps map[string][]string) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	if v == V110 {
		sb.WriteString(`<wfs:WFS_Capabilities xmlns:wfs="http://www.opengis.net/wfs" xmlns:ows="http://www.opengis.net/ows" xmlns:gml="http://www.opengis.net/gml" xmlns:ogc="http://www.opengis.net/ogc" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.opengis.net/wfs http://schemas.opengis.net/wfs/1.1.0/wfs.xsd" version="1.1.0">` + "\n")
	} else {
		sb.WriteString(`<wfs:WFS_Capabilities xmlns:wfs="http://www.opengis.net/wfs/2.0" xmlns:ows="http://www.opengis.net/ows/1.1" xmlns:fes="http://www.opengis.net/fes/2.0" xmlns:gml="http://www.opengis.net/gml/3.2" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.opengis.net/wfs/2.0 http://schemas.opengis.net/wfs/2.0/wfs.xsd" version="` + xmlEscape(string(v)) + `">` + "\n")
	}
	// Service identification.
	sb.WriteString("  <ows:ServiceIdentification>\n")
	sb.WriteString("    <ows:Title>Tegola WFS</ows:Title>\n")
	sb.WriteString("    <ows:ServiceType>WFS</ows:ServiceType>\n")
	sb.WriteString("    <ows:ServiceTypeVersion>" + string(v) + "</ows:ServiceTypeVersion>\n")
	sb.WriteString("  </ows:ServiceIdentification>\n")
	// Operations.
	sb.WriteString("  <ows:OperationsMetadata>\n")
	ops := []string{"GetCapabilities", "DescribeFeatureType", "GetFeature"}
	if len(writeOps) > 0 {
		ops = append(ops, "Transaction")
	}
	for _, op := range ops {
		sb.WriteString("    <ows:Operation name=\"" + op + "\">\n")
		sb.WriteString("      <ows:DCP><ows:HTTP>\n")
		if op != "Transaction" {
			sb.WriteString("        <ows:Get xlink:href=\"" + xmlEscape(baseURL) + "\"/>\n")
		}
		if op == "GetCapabilities" || op == "Transaction" {
			sb.WriteString("        <ows:Post xlink:href=\"" + xmlEscape(baseURL) + "\"/>\n")
		}
		sb.WriteString("      </ows:HTTP></ows:DCP></ows:Operation>\n")
	}
	sb.WriteString("  </ows:OperationsMetadata>\n")
	// Feature types.
	sb.WriteString("  <wfs:FeatureTypeList>\n")
	for _, c := range service.Collections() {
		if !gml.ValidNCName(c.ID) {
			continue
		}
		sb.WriteString("    <wfs:FeatureType>\n")
		sb.WriteString(`      <wfs:Name xmlns:app="` + xmlEscape("http://example.com/tegola/"+c.ID) + `">app:` + xmlEscape(c.ID) + "</wfs:Name>\n")
		sb.WriteString("      <wfs:Title>" + xmlEscape(c.Title) + "</wfs:Title>\n")
		if v == V110 {
			sb.WriteString("      <wfs:DefaultSRS>http://www.opengis.net/gml/srs/epsg.xml#4326</wfs:DefaultSRS>\n")
		} else {
			sb.WriteString("      <wfs:DefaultCRS>urn:ogc:def:crs:EPSG::4326</wfs:DefaultCRS>\n")
		}
		sb.WriteString("    </wfs:FeatureType>\n")
	}
	sb.WriteString("  </wfs:FeatureTypeList>\n")
	// Filter capabilities (advertised subset).
	if v == V110 {
		sb.WriteString("  <ogc:Filter_Capabilities>\n")
		sb.WriteString("    <ogc:Scalar_Capabilities><ogc:LogicalOperators/><ogc:ComparisonOperators><ogc:ComparisonOperator>PropertyIsEqualTo</ogc:ComparisonOperator><ogc:ComparisonOperator>PropertyIsNotEqualTo</ogc:ComparisonOperator></ogc:ComparisonOperators></ogc:Scalar_Capabilities>\n")
		sb.WriteString("    <ogc:Id_Capabilities><ogc:FID/></ogc:Id_Capabilities>\n")
		sb.WriteString("  </ogc:Filter_Capabilities>\n")
	} else {
		sb.WriteString("  <fes:Filter_Capabilities>\n")
		sb.WriteString("    <fes:Conformance><fes:Constraint name=\"ImplementsQuery\"><fes:DefaultValue>true</fes:DefaultValue></fes:Constraint></fes:Conformance>\n")
		sb.WriteString("    <fes:Id_Capabilities><fes:ResourceIdentifier name=\"fes:ResourceId\"/></fes:Id_Capabilities>\n")
		sb.WriteString("  </fes:Filter_Capabilities>\n")
	}
	if v == V110 {
		sb.WriteString("</wfs:WFS_Capabilities>\n")
	} else {
		sb.WriteString("</wfs:WFS_Capabilities>\n")
	}
	return sb.String()
}

// DescribeFeatureType renders the XSD application schema for a collection.
func DescribeFeatureType(v Version, collectionID string, schema *FeatureSchemaView) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	targetNS := xmlEscape("http://example.com/tegola/" + collectionID)
	if v == V110 {
		sb.WriteString(`<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:gml="http://www.opengis.net/gml" xmlns:tegola="` + targetNS + `" targetNamespace="` + targetNS + `" elementFormDefault="qualified">` + "\n")
		sb.WriteString(`  <xsd:import namespace="http://www.opengis.net/gml" schemaLocation="http://schemas.opengis.net/gml/3.1.1/base/gml.xsd"/>` + "\n")
	} else {
		sb.WriteString(`<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:gml="http://www.opengis.net/gml/3.2" xmlns:tegola="` + targetNS + `" targetNamespace="` + targetNS + `" elementFormDefault="qualified">` + "\n")
		sb.WriteString(`  <xsd:import namespace="http://www.opengis.net/gml/3.2" schemaLocation="http://schemas.opengis.net/gml/3.2.1/gml.xsd"/>` + "\n")
	}
	substitution := "gml:AbstractFeature"
	if v == V110 {
		substitution = "gml:_Feature"
	}
	sb.WriteString(`  <xsd:element name="` + xmlEscape(collectionID) + `" type="tegola:` + xmlEscape(collectionID) + `Type" substitutionGroup="` + substitution + `"/>` + "\n")
	sb.WriteString(`  <xsd:complexType name="` + xmlEscape(collectionID) + `Type">` + "\n")
	sb.WriteString(`    <xsd:complexContent><xsd:extension base="gml:AbstractFeatureType"><xsd:sequence>` + "\n")
	geometryType := schema.GeometryXSDType
	// The encoder uses the GML 3 aggregate names in both wire versions.
	switch geometryType {
	case "MultiLineStringPropertyType":
		geometryType = "MultiCurvePropertyType"
	case "MultiPolygonPropertyType":
		geometryType = "MultiSurfacePropertyType"
	}
	geometryNillable := ""
	geometryMinOccurs := "1"
	if schema.GeometryNullable {
		geometryNillable = ` nillable="true"`
		geometryMinOccurs = "0"
	}
	sb.WriteString(`      <xsd:element name="` + xmlEscape(schema.GeometryName) + `" type="gml:` +
		xmlEscape(geometryType) + `" minOccurs="` + geometryMinOccurs + `" maxOccurs="1"` + geometryNillable + `/>` + "\n")
	props := append([]SchemaPropView(nil), schema.Properties...)
	sort.Slice(props, func(i, j int) bool { return props[i].Name < props[j].Name })
	for _, p := range props {
		xsdType := map[string]string{
			"integer": "xsd:long", "decimal": "xsd:decimal", "string": "xsd:string",
			"boolean": "xsd:boolean", "datetime": "xsd:dateTime",
		}[p.Type]
		if xsdType == "" {
			xsdType = "xsd:string"
		}
		min := "1"
		if p.Nullable {
			min = "0"
		}
		nillable := ""
		if p.Nullable {
			nillable = ` nillable="true"`
		}
		sb.WriteString(fmt.Sprintf(`      <xsd:element name="%s" type="%s" minOccurs="%s" maxOccurs="1"%s/>`+"\n",
			xmlEscape(p.Name), xsdType, min, nillable))
	}

	sb.WriteString(`    </xsd:sequence></xsd:extension></xsd:complexContent>` + "\n")
	sb.WriteString(`  </xsd:complexType>` + "\n")
	sb.WriteString(`</xsd:schema>` + "\n")
	return sb.String()
}

// FeatureSchemaView is the DescribeFeatureType input.
type FeatureSchemaView struct {
	Properties       []SchemaPropView
	GeometryName     string
	GeometryXSDType  string
	GeometryNullable bool
}

// SchemaPropView is one property for XSD generation.
type SchemaPropView struct {
	Name     string
	Type     string
	Nullable bool
}

// ValidateSchemaView rejects schemas which cannot be represented as WFS XML.
func ValidateSchemaView(collectionID string, schema *FeatureSchemaView) error {
	if !gml.ValidNCName(collectionID) || schema == nil || !gml.ValidNCName(schema.GeometryName) {
		return fmt.Errorf("unsupported WFS XML name")
	}
	seen := map[string]bool{schema.GeometryName: true}
	for _, p := range schema.Properties {
		if !gml.ValidNCName(p.Name) || seen[p.Name] {
			return fmt.Errorf("unsupported or duplicate WFS property name %q", p.Name)
		}
		seen[p.Name] = true
	}
	return nil
}
