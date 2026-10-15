// Package gml encodes simple features as GML 3.1.1 (WFS 1.1) and
// GML 3.2.1 (WFS 2.0). Only the encoders needed for GetFeature output
// are implemented; parsing belongs to the WFS-T input path.
package gml

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/alexeydott/geom"
)

// Version selects the GML version.
type Version int

const (
	V311 Version = iota // GML 3.1.1 for WFS 1.1.0
	V321                // GML 3.2.1 for WFS 2.0
)

func (v Version) namespace() string {
	if v == V321 {
		return "http://www.opengis.net/gml/3.2"
	}
	return "http://www.opengis.net/gml"
}

// srsName returns the CRS identifier. For EPSG:4326, GML 3.2.1 uses the
// URN form with lat,lon axis order per the standard; 3.1.1 keeps the
// stored lon,lat order.
func (v Version) srsName(srid uint64) string {
	if srid == 4326 {
		if v == V311 {
			return "http://www.opengis.net/gml/srs/epsg.xml#4326"
		}
		return "urn:ogc:def:crs:EPSG::4326"
	}
	return fmt.Sprintf("urn:ogc:def:crs:EPSG::%d", srid)
}

// axisSwap reports whether coordinates must be emitted lat,lon.
func (v Version) axisSwap(srid uint64) bool {
	return v == V321 && srid == 4326
}

// Encoder writes GML fragments.
type Encoder struct {
	Version Version
	SRID    uint64
	sb      strings.Builder
}

func (e *Encoder) coord(c [2]float64) {
	x, y := c[0], c[1]
	if e.Version.axisSwap(e.SRID) {
		x, y = y, x
	}
	e.sb.WriteString(strconv.FormatFloat(x, 'g', -1, 64))
	e.sb.WriteByte(' ')
	e.sb.WriteString(strconv.FormatFloat(y, 'g', -1, 64))
}

func (e *Encoder) posList(coords [][2]float64) {
	e.sb.WriteString("<gml:posList>")
	for i, c := range coords {
		if i > 0 {
			e.sb.WriteByte(' ')
		}
		e.coord(c)
	}
	e.sb.WriteString("</gml:posList>")
}

func (e *Encoder) pos(c [2]float64) {
	e.sb.WriteString("<gml:pos>")
	e.coord(c)
	e.sb.WriteString("</gml:pos>")
}

// EncodeGeometry writes the GML geometry element for g.
func (e *Encoder) EncodeGeometry(g geom.Geometry) error {
	srs := fmt.Sprintf(` srsName="%s"`, xmlEscape(e.Version.srsName(e.SRID)))
	switch t := g.(type) {
	case geom.Point:
		e.sb.WriteString("<gml:Point" + srs + ">")
		e.pos(t)
		e.sb.WriteString("</gml:Point>")
	case geom.LineString:
		e.sb.WriteString("<gml:LineString" + srs + ">")
		e.posList(t)
		e.sb.WriteString("</gml:LineString>")
	case geom.Polygon:
		e.sb.WriteString("<gml:Polygon" + srs + ">")
		for i, ring := range t {
			if i == 0 {
				e.sb.WriteString("<gml:exterior><gml:LinearRing>")
			} else {
				e.sb.WriteString("<gml:interior><gml:LinearRing>")
			}
			e.posList(ring)
			e.sb.WriteString("</gml:LinearRing>")
			if i == 0 {
				e.sb.WriteString("</gml:exterior>")
			} else {
				e.sb.WriteString("</gml:interior>")
			}
		}
		e.sb.WriteString("</gml:Polygon>")
	case geom.MultiPoint:
		e.sb.WriteString("<gml:MultiPoint" + srs + ">")
		for _, c := range t {
			e.sb.WriteString("<gml:pointMember><gml:Point" + srs + ">")
			e.pos(c)
			e.sb.WriteString("</gml:Point></gml:pointMember>")
		}
		e.sb.WriteString("</gml:MultiPoint>")
	case geom.MultiLineString:
		e.sb.WriteString("<gml:MultiCurve" + srs + ">")
		for _, l := range t {
			e.sb.WriteString("<gml:curveMember><gml:LineString" + srs + ">")
			e.posList(l)
			e.sb.WriteString("</gml:LineString></gml:curveMember>")
		}
		e.sb.WriteString("</gml:MultiCurve>")
	case geom.MultiPolygon:
		e.sb.WriteString("<gml:MultiSurface" + srs + ">")
		for _, p := range t {
			e.sb.WriteString("<gml:surfaceMember><gml:Polygon" + srs + ">")
			for i, ring := range p {
				if i == 0 {
					e.sb.WriteString("<gml:exterior><gml:LinearRing>")
				} else {
					e.sb.WriteString("<gml:interior><gml:LinearRing>")
				}
				e.posList(ring)
				e.sb.WriteString("</gml:LinearRing>")
				if i == 0 {
					e.sb.WriteString("</gml:exterior>")
				} else {
					e.sb.WriteString("</gml:interior>")
				}
			}
			e.sb.WriteString("</gml:Polygon></gml:surfaceMember>")
		}
		e.sb.WriteString("</gml:MultiSurface>")
	default:
		return fmt.Errorf("unsupported geometry type %T", g)
	}
	return nil
}

// Feature is one feature to encode.
type Feature struct {
	// ID is the gml:id (NCName-safe WFS FID).
	ID string
	// TypeName is the qualified feature type name (prefix:local).
	TypeName string
	// GeometryName is the geometry property element name.
	GeometryName string
	Geometry     geom.Geometry
	// Properties maps property names to typed values.
	// A30: interface{} preserves types; encoder formats via formatGMLValue.
	Properties map[string]interface{}
	// SortKeys carries per-criterion sort values for post-query sorting;
	// not serialized, cleared before encoding.
	SortKeys []string
}

// EncodeFeature writes one GML feature member.
func (e *Encoder) EncodeFeature(f Feature) error {
	if !ValidNCName(f.TypeName) || !ValidNCName(f.GeometryName) {
		return fmt.Errorf("unsupported WFS XML feature or geometry name")
	}
	for name := range f.Properties {
		if !ValidNCName(name) || name == f.GeometryName {
			return fmt.Errorf("unsupported WFS XML property name %q", name)
		}
	}
	member := "gml:featureMember"
	if e.Version == V321 {
		member = "wfs:member"
	}
	e.sb.WriteString("<" + member + ">")
	e.sb.WriteString("<" + f.TypeName + ` xmlns="` + xmlEscape("http://example.com/tegola/"+f.TypeName) + `" gml:id="` + xmlEscape(f.ID) + `">`)
	if f.Geometry != nil {
		e.sb.WriteString("<" + f.GeometryName + ">")
		if err := e.EncodeGeometry(f.Geometry); err != nil {
			return err
		}
		e.sb.WriteString("</" + f.GeometryName + ">")
	}
	for _, name := range sortedKeys(f.Properties) {
		if f.Properties[name] == nil {
			e.sb.WriteString("<" + name + ` xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"/>`)
			continue
		}
		e.sb.WriteString("<" + name + ">")
		e.sb.WriteString(xmlEscape(formatGMLValue(f.Properties[name])))
		e.sb.WriteString("</" + name + ">")
	}
	e.sb.WriteString("</" + f.TypeName + ">")
	e.sb.WriteString("</" + member + ">")
	return nil
}

// ValidNCName admits XML names used by the supported simple-feature profile.
func ValidNCName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == '_' || unicode.IsLetter(r) {
			continue
		}
		if i > 0 && (unicode.IsDigit(r) || r == '-' || r == '.' || unicode.IsMark(r)) {
			continue
		}
		return false
	}
	return true
}

func (e *Encoder) String() string { return e.sb.String() }

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func xmlEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// formatGMLValue formats a typed property value for GML output.
// A30: preserves types instead of fmt.Sprintf("%v").
func formatGMLValue(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(t), 'g', -1, 32)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case []byte:
		return base64.StdEncoding.EncodeToString(t)
	default:
		return fmt.Sprintf("%v", v)
	}
}
