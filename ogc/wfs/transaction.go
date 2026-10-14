package wfs

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/alexeydott/tegola/feature"
	"github.com/alexeydott/tegola/provider"
)

// TransactionAction is one parsed Transaction action.
type TransactionAction struct {
	Op         provider.MutationOp
	TypeName   string
	Handle     string
	Properties map[string]string
	// FeatureXML is the raw inner XML of the feature element
	// (Insert/Replace), parsed by the executor.
	FeatureXML string
	// FilterIDs are target IDs from ResourceId/FeatureId filters.
	FilterIDs []uint64
}

// wfsTransaction is the wire struct for decoding.
type wfsTransaction struct {
	XMLName       xml.Name         `xml:"Transaction"`
	Version       string           `xml:"version,attr"`
	LockID        string           `xml:"lockId,attr"`
	ReleaseAction string           `xml:"releaseAction,attr"`
	Actions       []wfsActionInner `xml:",any"`
}

type wfsActionInner struct {
	XMLName   xml.Name
	Handle    string `xml:"handle,attr"`
	TypeName  string `xml:"typeName,attr"`  // WFS 1.1 singular
	TypeNames string `xml:"typeNames,attr"` // WFS 2.0 plural
	Inner     string `xml:",innerxml"`
}

// actionTypeName resolves the action's target layer: WFS 1.1 uses the
// singular typeName attribute, WFS 2.0 the plural typeNames. Without the
// fallback, Update/Delete with typeNames parsed to an empty TypeName and
// the R11 fid-belongs-to-type check failed every plural-form request.
func actionTypeName(a wfsActionInner) string {
	if a.TypeName != "" {
		return stripPrefix(a.TypeName)
	}
	return stripPrefix(a.TypeNames)
}

// ParseTransaction parses a WFS Transaction document (1.1 or 2.0).
// Structural XML limits are enforced by the HTTP layer before calling.
// ParseTransaction parses a WFS Transaction. Returns actions, lockId,
// releaseAction ("ALL" default), and error.
func ParseTransaction(v Version, body []byte) ([]TransactionAction, string, string, error) {
	var doc wfsTransaction
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, "", "ALL", fmt.Errorf("invalid Transaction XML: %w", err)
	}
	var actions []TransactionAction
	for _, a := range doc.Actions {
		local := a.XMLName.Local
		act := TransactionAction{Handle: a.Handle}
		switch local {
		case "Insert":
			act.Op = provider.MutationInsert
		case "Update":
			act.Op = provider.MutationUpdate
			act.TypeName = actionTypeName(a)
		case "Delete":
			act.Op = provider.MutationDelete
			act.TypeName = actionTypeName(a)
		case "Replace":
			if v == V110 {
				return nil, "", "ALL", fmt.Errorf("Replace is not a WFS 1.1 action")
			}
			act.Op = provider.MutationReplace
		default:
			// Filter elements etc. at top level are ignored here.
			continue
		}
		if act.Op == provider.MutationUpdate || act.Op == provider.MutationDelete {
			props, ids, err := parseActionFilter(a.Inner, act.TypeName)
			if err != nil {
				return nil, "", "ALL", err
			}
			act.Properties = props
			act.FilterIDs = ids
		} else if act.Op == provider.MutationInsert {
			// A27: Insert may contain multiple feature elements.
			features, err := parseFeatureElements(a.Inner)
			if err != nil {
				return nil, "", "ALL", fmt.Errorf("%s: %w", local, err)
			}
			if len(features) == 0 {
				return nil, "", "ALL", fmt.Errorf("%s: no features in Insert", local)
			}
			// First feature goes to act; rest become additional actions.
			act.TypeName = features[0].TypeName
			act.Properties = features[0].Properties
			act.FeatureXML = features[0].GeomXML
			for _, f := range features[1:] {
				extra := TransactionAction{
					Op:         provider.MutationInsert,
					TypeName:   f.TypeName,
					Properties: f.Properties,
					FeatureXML: f.GeomXML,
				}
				actions = append(actions, extra)
			}
		} else {
			// Replace: extract feature element AND filter IDs.
			// Structure: <Replace><Feature>...</Feature><Filter>...</Filter></Replace>
			typeName, props, geomXML, err := parseFeatureElement(a.Inner)
			if err != nil {
				return nil, "", "ALL", fmt.Errorf("%s: %w", local, err)
			}
			act.TypeName = typeName
			act.Properties = props
			act.FeatureXML = geomXML
			// Extract FilterIDs from the Filter element
			_, ids, ferr := parseActionFilter(a.Inner, act.TypeName)
			if ferr != nil {
				return nil, "", "ALL", ferr
			}
			act.FilterIDs = ids
		}
		actions = append(actions, act)
	}
	if len(actions) == 0 {
		return nil, "", "ALL", fmt.Errorf("no transaction actions found")
	}
	releaseAction := doc.ReleaseAction
	if releaseAction == "" {
		releaseAction = "ALL"
	}
	return actions, doc.LockID, releaseAction, nil
}

// parseActionFilter extracts Property assignments and ResourceId/FeatureId
// filters from Update/Delete inner XML.
// parseActionFilter extracts Property assignments and ResourceId/FeatureId
// filters from Update/Delete inner XML.
//
// A26 (fail-closed): only pure ID filters are accepted. The filter must
// consist solely of fes:ResourceId / ogc:FeatureId elements (optionally
// wrapped in a single fes:Filter). Any other predicate (PropertyIsEqualTo,
// And/Or/Not, BBOX, etc.) is rejected explicitly instead of being silently
// ignored, which would widen the affected set.
// fidMatchesType checks if a FID collection matches the action typeName (R11).
func fidMatchesType(fidColl, typeName string) bool {
	strip := func(s string) string {
		if i := strings.LastIndex(s, ":"); i >= 0 {
			return s[i+1:]
		}
		return s
	}
	return strip(fidColl) == strip(typeName)
}

func parseActionFilter(inner, typeName string) (map[string]string, []uint64, error) {
	props := map[string]string{}
	var ids []uint64
	dec := xml.NewDecoder(strings.NewReader(inner))
	var curProp, curValue string
	var curValueXML strings.Builder
	inValue, inRef := false, false
	valueDepth := 0
	depth := 0
	inFilter := false
	filterDepth := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			local := t.Name.Local
			if inValue && valueDepth == 0 && local != "Value" {
				valueDepth = 1
				writeStartElement(&curValueXML, t)
				continue
			}
			if valueDepth > 0 {
				valueDepth++
				writeStartElement(&curValueXML, t)
				continue
			}
			if local == "Filter" && !inFilter {
				inFilter = true
				filterDepth = depth
				continue
			}
			if inFilter {
				// A26: inside Filter, only ResourceId/FeatureId allowed.
				switch local {
				case "ResourceId", "FeatureId":
					for _, at := range t.Attr {
						if at.Name.Local == "fid" || at.Name.Local == "rid" {
							// R11: verify FID belongs to the action's typeName.
							// A ResourceId like "other.1" must not target "sites.1".
							fidColl, id, err := feature.DecodeWFSFID(at.Value)
							if err != nil {
								return nil, nil, fmt.Errorf("invalid feature ID %q", at.Value)
							}
							if fidColl != "" && !fidMatchesType(fidColl, typeName) {
								return nil, nil, fmt.Errorf("feature ID %q does not belong to typeName %q", at.Value, typeName)
							}
							ids = append(ids, id)
						}
					}
				default:
					return nil, nil, fmt.Errorf("unsupported filter predicate <%s>: only ResourceId/FeatureId filters are supported", local)
				}
				continue
			}
			switch local {
			case "Property":
			curProp, curValue = "", ""
			case "Name", "ValueReference":
			inRef = true
			case "Value":
			inValue = true
			}
		case xml.EndElement:
			if valueDepth > 0 {
				valueDepth--
				curValueXML.WriteString("</" + t.Name.Local + ">")
				if valueDepth == 0 {
					curValue = curValueXML.String()
					curValueXML.Reset()
				}
				continue
			}
			if inFilter && depth == filterDepth && t.Name.Local == "Filter" {
				inFilter = false
			}
			depth--
			switch t.Name.Local {
			case "Property":
				if curProp != "" {
					props[curProp] = curValue
				}
			case "Name", "ValueReference":
				inRef = false
			case "Value":
				inValue = false
			}
		case xml.CharData:
			if valueDepth > 0 {
				curValueXML.WriteString(xmlEscape(string(t)))
				continue
			}
			text := strings.TrimSpace(string(t))
			if text == "" {
				continue
			}
			if inRef {
				curProp = stripPrefix(text)
			} else if inValue {
				curValue = text
			}
		}
	}
	return props, ids, nil
}

// writeStartElement appends a raw start tag (local name + attributes) to sb.
func writeStartElement(sb *strings.Builder, t xml.StartElement) {
	sb.WriteString("<" + t.Name.Local)
	for _, at := range t.Attr {
		sb.WriteString(" " + at.Name.Local + `="` + xmlEscape(at.Value) + `"`)
	}
	sb.WriteString(">")
}

// parsedFeature is one feature from an Insert.
type parsedFeature struct {
	TypeName   string
	Properties map[string]string
	GeomXML    string
}

// parseFeatureElements splits inner XML into top-level feature elements
// and parses each (A27: multi-feature Insert).
func parseFeatureElements(inner string) ([]parsedFeature, error) {
	dec := xml.NewDecoder(strings.NewReader(inner))
	var features []parsedFeature
	var buf strings.Builder
	depth := 0
	inFeature := false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 1 {
				inFeature = true
				buf.Reset()
			}
			if inFeature {
				writeStartElement(&buf, t)
			}
		case xml.EndElement:
			if inFeature {
				buf.WriteString("</" + t.Name.Local + ">")
			}
			if depth == 1 && inFeature {
				inFeature = false
				tn, props, geom, err := parseFeatureElement(buf.String())
				if err != nil {
					return nil, err
				}
				features = append(features, parsedFeature{TypeName: tn, Properties: props, GeomXML: geom})
			}
			depth--
		case xml.CharData:
			if inFeature {
				buf.WriteString(xmlEscape(string(t)))
			}
		}
	}
	return features, nil
}

// parseFeatureElement extracts the type name, scalar properties and the
// raw GML geometry element from a feature's inner XML.
func parseFeatureElement(inner string) (string, map[string]string, string, error) {
	dec := xml.NewDecoder(strings.NewReader(inner))
	props := map[string]string{}
	var typeName, geomXML, curElem string
	var buf strings.Builder
	inGeom := false
	geomDepth := 0
	depth := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 1 {
				// Skip Filter elements; typeName is the feature element.
				// BUG-1 fix: don't overwrite typeName with "Filter".
				local := stripPrefix(t.Name.Local)
				if local != "Filter" && typeName == "" {
					typeName = local
				}
				continue
			}
			if depth == 2 {
				curElem = stripPrefix(t.Name.Local)
				// Heuristic: an element containing GML namespace children
				// or a known GML geometry name is the geometry.
				if isGMLGeometryElement(t.Name.Local) {
					inGeom, geomDepth = true, depth
					buf.Reset()
					buf.WriteString("<" + t.Name.Local)
					for _, at := range t.Attr {
						buf.WriteString(fmt.Sprintf(` %s="%s"`, at.Name.Local, at.Value))
					}
					buf.WriteString(">")
					continue
				}
			}
			// OL style: <geometryProperty><gml:Point>… — GML wrapped in a
			// property element at depth 2, geometry element at depth 3.
			if depth == 3 && !inGeom && isGMLGeometryElement(t.Name.Local) {
				inGeom, geomDepth = true, depth
				buf.Reset()
				buf.WriteString("<" + t.Name.Local)
				for _, at := range t.Attr {
					buf.WriteString(fmt.Sprintf(` %s="%s"`, at.Name.Local, at.Value))
				}
				buf.WriteString(">")
				continue
			}
			if inGeom {
				buf.WriteString("<" + t.Name.Local + ">")
			}
		case xml.EndElement:
			if inGeom {
				buf.WriteString("</" + t.Name.Local + ">")
				if depth == geomDepth {
					inGeom = false
					geomXML = buf.String()
				}
			} else if depth == 2 && curElem != "" {
				curElem = ""
			}
			depth--
		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if inGeom {
				buf.WriteString(text)
			} else if depth == 2 && curElem != "" && text != "" {
				props[curElem] = text
			}
		}
	}
	if typeName == "" {
		return "", nil, "", fmt.Errorf("no feature element found")
	}
	return typeName, props, geomXML, nil
}

func isGMLGeometryElement(local string) bool {
	switch local {
	case "Point", "LineString", "Polygon", "MultiPoint", "MultiLineString",
		"MultiPolygon", "MultiCurve", "MultiSurface", "GeometryCollection",
		"Curve", "Surface", "LineStringSegment":
		return true
	}
	return false
}

func stripPrefix(qname string) string {
	if i := strings.Index(qname, ":"); i >= 0 {
		return qname[i+1:]
	}
	return qname
}

// TransactionResponse renders the versioned TransactionResponse.
func TransactionResponse(v Version, results []TransactionResult) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	if v == V110 {
		sb.WriteString(`<wfs:TransactionResponse xmlns:wfs="http://www.opengis.net/wfs" xmlns:ogc="http://www.opengis.net/ogc" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.opengis.net/wfs http://schemas.opengis.net/wfs/1.1.0/wfs.xsd">` + "\n")
	} else {
		sb.WriteString(`<wfs:TransactionResponse xmlns:wfs="http://www.opengis.net/wfs/2.0" xmlns:fes="http://www.opengis.net/fes/2.0" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.opengis.net/wfs/2.0 http://schemas.opengis.net/wfs/2.0/wfs.xsd" version="2.0.2">` + "\n")
	}
	var ins, upd, del, rep int
	for _, r := range results {
		switch r.Op {
		case provider.MutationInsert:
			ins += r.Affected
		case provider.MutationUpdate:
			upd += r.Affected
		case provider.MutationDelete:
			del += r.Affected
		case provider.MutationReplace:
			rep += r.Affected
		}
	}
	sb.WriteString(`  <wfs:TransactionSummary>`)
	sb.WriteString(fmt.Sprintf(`<wfs:totalInserted>%d</wfs:totalInserted>`, ins))
	sb.WriteString(fmt.Sprintf(`<wfs:totalUpdated>%d</wfs:totalUpdated>`, upd))
	sb.WriteString(fmt.Sprintf(`<wfs:totalDeleted>%d</wfs:totalDeleted>`, del))
	if v != V110 {
		sb.WriteString(fmt.Sprintf(`<wfs:totalReplaced>%d</wfs:totalReplaced>`, rep))
	}
	sb.WriteString(`</wfs:TransactionSummary>` + "\n")
	// A24: version-specific IDs. 1.1 uses ogc:FeatureId; 2.0 uses
	// fes:ResourceId only (ogc namespace not declared in 2.0 root).
	for _, r := range results {
		if r.Op == provider.MutationInsert && r.FeatureID != 0 {
			fid, _ := feature.EncodeWFSFID(r.TypeName, r.FeatureID)
			sb.WriteString(`  <wfs:InsertResults><wfs:Feature>`)
			if v == V110 {
				sb.WriteString(`<ogc:FeatureId fid="` + xmlEscape(fid) + `"/>`)
			} else {
				sb.WriteString(`<fes:ResourceId rid="` + xmlEscape(fid) + `"/>`)
			}
			sb.WriteString(`</wfs:Feature></wfs:InsertResults>` + "\n")
		}
	}
	sb.WriteString(`</wfs:TransactionResponse>` + "\n")
	return sb.String()
}

// TransactionResult is one executed action result.
type TransactionResult struct {
	Op        provider.MutationOp
	TypeName  string
	FeatureID uint64
	Affected  int
}
