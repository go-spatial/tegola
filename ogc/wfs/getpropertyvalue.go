package wfs

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/ogc/wfs/gml"
)

// GetPropertyValueRequest selects one simple property using GetFeature query semantics.
type GetPropertyValueRequest struct {
	Version        Version
	TypeName       string
	ValueReference string
	MaxFeatures    uint
	BBox           *[4]float64
	BBoxCRS        string
	FeatureIDs     []uint64
	ResultType     string
	StartIndex     uint
	SortBy         []SortCriterion
	Filter         *FESFilter
}

func ParseGetPropertyValueKVP(v Version, q map[string]string) (*GetPropertyValueRequest, []Exception) {
	if v != V200 && v != V202 {
		return nil, []Exception{{Code: ExceptionOperationNotSupported, Text: "GetPropertyValue requires WFS 2.0"}}
	}
	vr := strings.TrimSpace(q["valuereference"])
	if vr == "" {
		return nil, []Exception{{Code: ExceptionMissingParameterValue, Locator: "valueReference", Text: "valueReference is required"}}
	}
	if q["propertyname"] != "" {
		return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "propertyName", Text: "use valueReference for property selection"}}
	}
	query, ex := ParseGetFeatureKVP(v, q)
	if len(ex) > 0 {
		return nil, ex
	}
	bindings, err := propertyBindingsKVP(query.TypeName, q)
	if err != nil {
		return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "namespaces", Text: err.Error()}}
	}
	vr, err = resolvePropertyQName(vr, query.TypeName, bindings)
	if err != nil {
		return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "valueReference", Text: err.Error()}}
	}
	return &GetPropertyValueRequest{Version: v, TypeName: query.TypeName, ValueReference: vr, MaxFeatures: query.MaxFeatures, BBox: query.BBox, BBoxCRS: query.BBoxCRS, FeatureIDs: query.FeatureIDs, ResultType: query.ResultType, StartIndex: query.StartIndex, SortBy: query.SortBy, Filter: query.Filter}, nil
}

// ExecuteGetPropertyValue projects the same validated, sorted and paged query as
// GetFeature. Extracting its encoded property preserves scalar and GML geometry
// representations, including xsi:nil, without a second query implementation.
func ExecuteGetPropertyValue(ctx context.Context, service *features.Service, req *GetPropertyValueRequest) (string, []Exception) {
	if req.Version != V200 && req.Version != V202 {
		return "", []Exception{{Code: ExceptionOperationNotSupported, Text: "GetPropertyValue requires WFS 2.0"}}
	}
	if !gml.ValidNCName(req.ValueReference) {
		return "", []Exception{{Code: ExceptionInvalidParameterValue, Locator: "valueReference", Text: "invalid property name"}}
	}
	query := &GetFeatureRequest{Version: req.Version, TypeName: req.TypeName, MaxFeatures: req.MaxFeatures, BBox: req.BBox, BBoxCRS: req.BBoxCRS, FeatureIDs: req.FeatureIDs, ResultType: req.ResultType, StartIndex: req.StartIndex, SortBy: req.SortBy, Filter: req.Filter, PropertyNames: []string{req.ValueReference}}
	out, ex := ExecuteGetFeature(ctx, service, query)
	if len(ex) > 0 {
		for i := range ex {
			if ex[i].Locator == "propertyName" {
				ex[i].Locator = "valueReference"
			}
		}
		return "", ex
	}
	var collection struct {
		Matched  string `xml:"numberMatched,attr"`
		Returned string `xml:"numberReturned,attr"`
		Members  []struct {
			Feature struct {
				Inner string `xml:",innerxml"`
			} `xml:",any"`
		} `xml:"member"`
	}
	if err := xml.Unmarshal([]byte(out), &collection); err != nil {
		return "", []Exception{{Code: ExceptionNoApplicableCode, Text: "cannot encode property collection"}}
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, `<wfs:ValueCollection xmlns:wfs="http://www.opengis.net/wfs/2.0" xmlns:gml="http://www.opengis.net/gml/3.2" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" numberMatched="%s" numberReturned="%s" timeStamp="%s">`, xmlEscape(collection.Matched), xmlEscape(collection.Returned), xmlTime())
	for _, m := range collection.Members {
		fmt.Fprintf(&sb, `<wfs:member xmlns="%s">`, xmlEscape("http://example.com/tegola/"+req.TypeName))
		if strings.TrimSpace(m.Feature.Inner) == "" {
			// A null geometry or absent selected nullable value has no encoded element.
			fmt.Fprintf(&sb, `<%s xsi:nil="true"/>`, req.ValueReference)
		} else {
			sb.WriteString(m.Feature.Inner)
		}
		sb.WriteString(`</wfs:member>`)
	}
	sb.WriteString(`</wfs:ValueCollection>`)
	return sb.String(), nil
}

func xmlTime() string { return time.Now().UTC().Format(time.RFC3339) }
