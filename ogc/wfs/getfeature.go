package wfs

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"mime"
	"sort"
	"strconv"
	"strings"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/feature"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/ogc/wfs/gml"
	"github.com/alexeydott/tegola/provider"
)

// GetFeatureRequest is the parsed GetFeature input (KVP subset).
type GetFeatureRequest struct {
	Version     Version
	TypeName    string
	MaxFeatures uint
	// BBox uses CRS84 lon/lat unless BBoxCRS names a collection-supported CRS.
	BBox       *[4]float64
	BBoxCRS    string
	FeatureIDs []uint64
	// OutputFormat: only "application/gml+xml" variants are supported.
	OutputFormat string
	// ResultType is "results" (default) or "hits" (count only).
	ResultType string
	// PropertyNames restricts returned properties (projection). Empty
	// means all properties.
	PropertyNames []string
	// SortBy is a list of "property [ASC|DESC]" sort criteria.
	SortBy []SortCriterion
	// StartIndex is the 0-based offset for paging (A28).
	StartIndex uint
	// Filter is an optional FES filter expression (A26).
	Filter *FESFilter
}

// SortCriterion is one sortBy term.
type SortCriterion struct {
	Property   string
	Descending bool
	numeric    bool
}

// ParseGetFeatureKVP parses the KVP subset for GetFeature.
func ParseGetFeatureKVP(v Version, q map[string]string) (*GetFeatureRequest, []Exception) {
	outputFormat := strings.TrimSpace(q["outputformat"])
	if !supportsGMLOutputFormat(v, outputFormat) {
		return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "outputFormat", Text: "only this WFS version's GML output is supported"}}
	}
	// Stored query dispatch (WFS 2.0).
	if sqID := q["storedquery_id"]; sqID != "" {
		if v == V110 {
			return nil, []Exception{{Code: ExceptionOperationNotSupported, Text: "stored queries require WFS 2.0"}}
		}
		sq, ok := GetStoredQuery(sqID)
		if !ok {
			return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "storedQueryId", Text: fmt.Sprintf("unknown stored query %q", sqID)}}
		}
		parsed, ex := sq.ToGetFeature(q)
		if parsed != nil {
			parsed.Version = v
			parsed.OutputFormat = outputFormat
		}
		return parsed, ex
	}
	req := &GetFeatureRequest{Version: v, MaxFeatures: 1000, OutputFormat: outputFormat}
	if q["srsname"] != "" {
		return nil, []Exception{{Code: ExceptionOperationNotSupported, Locator: "srsName", Text: "explicit output CRS selection is not supported"}}
	}
	selectors := 0
	for _, key := range []string{"filter", "bbox", "featureid", "resourceid"} {
		if q[key] != "" {
			selectors++
		}
	}
	if selectors > 1 {
		return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "filter", Text: "filter, bbox and feature IDs are mutually exclusive"}}
	}
	typeName := q["typename"]
	if typeName == "" {
		typeName = q["typenames"]
	}
	if typeName == "" {
		return nil, []Exception{{Code: ExceptionMissingParameterValue, Locator: "typeName", Text: "typeName is required"}}
	}
	// Only single type names in this profile.
	if strings.Contains(typeName, ",") {
		return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "typeName", Text: "multiple type names not supported"}}
	}
	resolved, err := ResolveTypeNameKVP(typeName, q)
	if err != nil {
		return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "typeName", Text: err.Error()}}
	}
	req.TypeName = resolved
	bindings, err := propertyBindingsKVP(resolved, q)
	if err != nil {
		return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "namespaces", Text: err.Error()}}
	}
	if f := q["filter"]; f != "" {
		// XML QName values use explicit declarations; the convenient advertised
		// app prefix fallback applies only to KVP QName values.
		filterBindings, err := kvpNamespaces(q)
		if err != nil {
			return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "namespaces", Text: err.Error()}}
		}
		flt, err := parseFESFilterContext([]byte(f), resolved, filterBindings)
		if err != nil {
			return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "filter", Text: err.Error()}}
		}
		req.Filter = &flt
	}
	if mf := q["maxfeatures"]; mf == "" {
		mf = q["count"]
		if mf != "" {
			n, err := strconv.ParseUint(mf, 10, 32)
			if err != nil {
				return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "count", Text: "invalid count"}}
			}
			req.MaxFeatures = uint(n)
		}
	} else {
		n, err := strconv.ParseUint(mf, 10, 32)
		if err != nil {
			return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "maxFeatures", Text: "invalid maxFeatures"}}
		}
		req.MaxFeatures = uint(n)
	}
	// A28: parse startIndex for paging.
	if si := q["startindex"]; si != "" {
		n, err := strconv.ParseUint(si, 10, 32)
		if err != nil {
			return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "startIndex", Text: "invalid startIndex"}}
		}
		req.StartIndex = uint(n)
	}
	if bbox := q["bbox"]; bbox != "" {
		parts := strings.Split(bbox, ",")
		if len(parts) != 4 && len(parts) != 5 {
			return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "bbox", Text: "bbox needs 4 ordinates and an optional CRS URI"}}
		}
		var b [4]float64
		if len(parts) == 5 {
			req.BBoxCRS = strings.TrimSpace(parts[4])
			if req.BBoxCRS == "" || len(req.BBoxCRS) > features.MaxCRSURIBytes {
				return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "bbox", Text: "invalid bbox CRS URI"}}
			}
		}
		for i, p := range parts[:4] {
			f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
			if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
				return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "bbox", Text: "invalid bbox ordinate"}}
			}
			b[i] = f
		}
		if b[0] > b[2] || b[1] > b[3] {
			return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "bbox", Text: "bbox minimum exceeds maximum"}}
		}
		req.BBox = &b
	}
	fids := q["featureid"]
	if fids == "" {
		fids = q["resourceid"]
	}
	if fids != "" {
		for _, fid := range strings.Split(fids, ",") {
			id, err := resolveFeatureID(strings.TrimSpace(fid), req.TypeName, bindings)
			if err != nil {
				return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "featureId", Text: fmt.Sprintf("invalid feature ID %q", fid)}}
			}
			req.FeatureIDs = append(req.FeatureIDs, id)
		}
	}
	if rt := strings.ToLower(q["resulttype"]); rt != "" {
		if rt != "results" && rt != "hits" {
			return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "resultType", Text: fmt.Sprintf("unsupported resultType %q", q["resulttype"])}}
		}
		req.ResultType = rt
	} else {
		req.ResultType = "results"
	}
	if pn := q["propertyname"]; pn != "" {
		for _, p := range strings.Split(pn, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			p, err = resolvePropertyQName(p, req.TypeName, bindings)
			if err != nil {
				return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "propertyName", Text: err.Error()}}
			}
			req.PropertyNames = append(req.PropertyNames, p)
		}
	}
	if sb := q["sortby"]; sb != "" {
		for _, term := range strings.Split(sb, ",") {
			term = strings.TrimSpace(term)
			if term == "" {
				continue
			}
			crit := SortCriterion{}
			parts := strings.Fields(term)
			if len(parts) > 2 {
				return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "sortBy", Text: "invalid sort criterion"}}
			}
			prop, err := resolvePropertyQName(parts[0], req.TypeName, bindings)
			if err != nil {
				return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "sortBy", Text: err.Error()}}
			}
			crit.Property = prop
			if len(parts) > 1 {
				switch strings.ToUpper(parts[1]) {
				case "DESC", "D":
					crit.Descending = true
				case "ASC", "A":
					// default
				default:
					return nil, []Exception{{Code: ExceptionInvalidParameterValue, Locator: "sortBy", Text: fmt.Sprintf("invalid sort direction %q", parts[1])}}
				}
			}
			req.SortBy = append(req.SortBy, crit)
		}
	}
	return req, nil
}

func supportsGMLOutputFormat(v Version, format string) bool {
	if format == "" {
		return true
	}
	gmlVersion := "3.2"
	if v == V110 {
		gmlVersion = "3.1.1"
	}
	// WFS 1.1 commonly uses this historical unquoted subtype spelling,
	// although the slash requires quotes in an ordinary MIME parameter.
	if strings.EqualFold(strings.ReplaceAll(format, " ", ""), "text/xml;subtype=gml/"+gmlVersion) {
		return true
	}
	mediaType, parameters, err := mime.ParseMediaType(format)
	if err != nil || (mediaType != "application/gml+xml" && mediaType != "application/xml" && mediaType != "text/xml") {
		return false
	}
	for name, value := range parameters {
		switch name {
		case "charset":
			if !strings.EqualFold(value, "utf-8") {
				return false
			}
		case "version":
			if mediaType != "application/gml+xml" || value != gmlVersion {
				return false
			}
		case "subtype":
			if mediaType != "text/xml" || !strings.EqualFold(value, "gml/"+gmlVersion) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// ExecuteGetFeature runs the request and renders a GML FeatureCollection.
func ExecuteGetFeature(ctx context.Context, service *features.Service, req *GetFeatureRequest) (string, []Exception) {
	// Keep schema-derived sort metadata local to this execution.
	request := *req
	request.SortBy = append([]SortCriterion(nil), req.SortBy...)
	req = &request
	// Collection must be published.
	if _, err := service.Collection(req.TypeName); err != nil {
		return "", []Exception{{Code: ExceptionInvalidParameterValue, Locator: "typeName", Text: fmt.Sprintf("unknown type %q", req.TypeName)}}
	}
	const maxScan = 100000
	if req.MaxFeatures > maxScan {
		return "", []Exception{{Code: ExceptionInvalidParameterValue, Locator: "count", Text: "count exceeds 100000"}}
	}
	fq := provider.FeatureQuery{Limit: req.MaxFeatures, Offset: uint64(req.StartIndex)}
	if fq.Limit == 0 {
		fq.Limit = 1
	}
	// Providers currently expose ID ordering only. Custom sorting scans a bounded
	// complete match set, then sorts before applying the requested page.
	if len(req.SortBy) > 0 {
		fq.Limit = maxScan + 1
		fq.Offset = 0
	}
	if req.BBox != nil {
		b := req.BBox
		fq.Bounds = []geom.Extent{*geom.NewExtent([2]float64{b[0], b[1]}, [2]float64{b[2], b[3]})}
		fq.BoundsSRID = 4326
		if req.BBoxCRS != "" {
			catalog, err := service.CollectionCRS(req.TypeName)
			if err != nil {
				return "", []Exception{{Code: ExceptionInvalidParameterValue, Locator: "bbox", Text: "collection CRS unavailable"}}
			}
			descriptor, err := catalog.ValidateBounds(req.BBoxCRS, 4)
			if err != nil {
				return "", []Exception{{Code: ExceptionInvalidParameterValue, Locator: "bbox", Text: "unsupported bbox CRS for collection"}}
			}
			minimum, err := descriptor.ToInternalPosition(b[:2])
			if err != nil {
				return "", []Exception{{Code: ExceptionInvalidParameterValue, Locator: "bbox", Text: "invalid bbox position"}}
			}
			maximum, err := descriptor.ToInternalPosition(b[2:])
			if err != nil {
				return "", []Exception{{Code: ExceptionInvalidParameterValue, Locator: "bbox", Text: "invalid bbox position"}}
			}
			if descriptor.Geographic() && (minimum[0] < -180 || maximum[0] > 180 || minimum[1] < -90 || maximum[1] > 90) {
				return "", []Exception{{Code: ExceptionInvalidParameterValue, Locator: "bbox", Text: "bbox outside geographic domain"}}
			}
			fq.Bounds = []geom.Extent{{minimum[0], minimum[1], maximum[0], maximum[1]}}
			fq.BoundsSRID = descriptor.InternalSRID()
			fq.BoundsCRSDefinition = descriptor.Definition().Definition
		}
	}
	if len(req.FeatureIDs) > 0 {
		fq.IDs = req.FeatureIDs
	}
	// A26: apply the FES filter if present.
	if req.Filter != nil {
		catalog, err := service.Queryables(req.TypeName)
		if err != nil {
			return "", []Exception{{Code: ExceptionOperationNotSupported, Locator: "filter", Text: "queryables unavailable"}}
		}
		filter, err := req.Filter.Resolve(catalog)
		if err != nil {
			return "", []Exception{{Code: ExceptionInvalidParameterValue, Locator: "filter", Text: err.Error()}}
		}
		fq.Filter = &filter
	}
	schema, err := service.SchemaDescriptorFor(ctx, req.TypeName)
	if err != nil {
		return "", []Exception{{Code: ExceptionNoApplicableCode, Text: "schema unavailable"}}
	}
	view := &FeatureSchemaView{GeometryName: schema.Geometry.Name}
	for _, p := range schema.Properties {
		view.Properties = append(view.Properties, SchemaPropView{Name: p.Name})
	}
	if err := ValidateSchemaView(req.TypeName, view); err != nil {
		return "", []Exception{{Code: ExceptionOperationNotSupported, Locator: "typeName", Text: "Collection schema contains unsupported WFS XML names"}}
	}
	var gv gml.Version
	if req.Version == V110 {
		gv = gml.V311
	} else {
		gv = gml.V321
	}
	enc := &gml.Encoder{Version: gv, SRID: 4326}
	// Validate property names and sort criteria against the schema.
	if len(req.PropertyNames) > 0 {
		for _, pn := range req.PropertyNames {
			if _, ok := schema.Property(pn); !ok && pn != schema.Geometry.Name {
				return "", []Exception{{Code: ExceptionInvalidParameterValue, Locator: "propertyName", Text: fmt.Sprintf("unknown property %q", pn)}}
			}
		}
	}
	for i, sc := range req.SortBy {
		desc, ok := schema.Property(sc.Property)
		if ok {
			req.SortBy[i].numeric = desc.Type == feature.TypeInteger || desc.Type == feature.TypeDecimal
		}
		if !ok {
			return "", []Exception{{Code: ExceptionInvalidParameterValue, Locator: "sortBy", Text: fmt.Sprintf("unknown sort property %q", sc.Property)}}
		}
	}
	// resultType=hits: count only, no feature encoding.
	// W27: numberMatched must be the global matching count, not the page
	// count.
	// A29: bounded hits. Full scan is capped at maxHitsScan to avoid
	// runaway queries; native COUNT is planned (requires provider
	// interface change). If the cap is hit, return 400 instead of a
	// misleading partial count.
	if req.ResultType == "hits" {
		const maxHitsScan = 100000
		hitsQ := fq
		hitsQ.Limit = maxHitsScan + 1
		hitsQ.Offset = 0
		count := 0
		_, err = service.QueryCollection(ctx, req.TypeName, hitsQ, func(f features.Feature) error {
			count++
			return nil
		})
		if err != nil {
			return "", []Exception{{Code: ExceptionNoApplicableCode, Text: fmt.Sprintf("query failed: %v", err)}}
		}
		if count > maxHitsScan {
			return "", []Exception{{Code: ExceptionOperationNotSupported, Text: fmt.Sprintf("hits count exceeds bounded limit %d; native COUNT not yet implemented", maxHitsScan)}}
		}
		returned := 0
		if req.Version == V110 {
			returned = count
		}
		return featureCollectionEnvelope(req.Version, "", strconv.Itoa(count), returned), nil
	}
	// Buffer for sorting; streams directly when no sortBy.
	var buffered []gml.Feature
	encodeOne := func(f features.Feature) error {
		fid, err := feature.EncodeWFSFID(req.TypeName, f.ID)
		if err != nil {
			return err
		}
		gf := gml.Feature{
			ID:           fid,
			TypeName:     req.TypeName,
			GeometryName: schema.Geometry.Name,
			Properties:   map[string]interface{}{},
		}
		// A30: use typed geometry directly when available, bypassing
		// the GeoJSON intermediate parse.
		if f.GeometryTyped != nil {
			gf.Geometry = f.GeometryTyped
		} else if len(f.Geometry) > 0 && string(f.Geometry) != "null" {
			g, err := parseServiceGeometry(f.Geometry)
			if err != nil {
				return err
			}
			gf.Geometry = g
		}
		if len(req.PropertyNames) > 0 {
			includeGeometry := false
			for _, pn := range req.PropertyNames {
				if pn == schema.Geometry.Name {
					includeGeometry = true
				}
			}
			if !includeGeometry {
				gf.Geometry = nil
			}
		}
		// Projection: only requested properties.
		props := f.Properties
		if len(req.PropertyNames) > 0 {
			props = map[string]interface{}{}
			for _, pn := range req.PropertyNames {
				if v, ok := f.Properties[pn]; ok {
					props[pn] = v
				}
			}
		}
		for k, v := range props {
			// Keep typed values and SQL NULL for the GML encoder.
			gf.Properties[k] = v
		}
		// Keep sort keys alongside for post-query sorting.
		if len(req.SortBy) > 0 {
			gf.SortKeys = make([]string, len(req.SortBy))
			for i, sc := range req.SortBy {
				if v, ok := f.Properties[sc.Property]; ok {
					gf.SortKeys[i] = fmt.Sprintf("%v", v)
				}
			}
			buffered = append(buffered, gf)
		} else {
			if err := enc.EncodeFeature(gf); err != nil {
				return err
			}
		}
		return nil
	}
	count := 0
	result, err := service.QueryCollection(ctx, req.TypeName, fq, func(f features.Feature) error {
		if req.MaxFeatures == 0 {
			return nil
		}
		count++
		return encodeOne(f)
	})
	if err != nil {
		return "", []Exception{{Code: ExceptionNoApplicableCode, Text: fmt.Sprintf("query failed: %v", err)}}
	}
	if len(req.SortBy) > 0 {
		if len(buffered) > maxScan {
			return "", []Exception{{Code: ExceptionOperationNotSupported, Text: "sort exceeds bounded scan limit"}}
		}
		sortFeatures(buffered, req.SortBy)
		start := min(uint(len(buffered)), req.StartIndex)
		end := min(uint(len(buffered)), start+req.MaxFeatures)
		count = int(end - start)
		for _, gf := range buffered[start:end] {
			gf.SortKeys = nil
			if err := enc.EncodeFeature(gf); err != nil {
				return "", []Exception{{Code: ExceptionNoApplicableCode, Text: fmt.Sprintf("encode failed: %v", err)}}
			}
		}
	}
	matched := "unknown"
	if result.NumberMatched != nil {
		matched = strconv.FormatUint(*result.NumberMatched, 10)
	}
	if len(req.SortBy) > 0 && req.MaxFeatures > 0 {
		matched = strconv.Itoa(len(buffered))
	}
	return featureCollectionEnvelope(req.Version, enc.String(), matched, count), nil
}

// sortFeatures orders buffered GML features by the sort criteria.
// Numeric strings compare numerically; otherwise lexicographically.
func sortFeatures(fs []gml.Feature, criteria []SortCriterion) {
	compare := func(a, b string, numeric bool) int {
		if numeric {
			av, aok := new(big.Rat).SetString(a)
			bv, bok := new(big.Rat).SetString(b)
			if aok && bok {
				return av.Cmp(bv)
			}
		}
		return strings.Compare(a, b)
	}

	sort.SliceStable(fs, func(i, j int) bool {
		for k, sc := range criteria {
			a, b := fs[i].SortKeys[k], fs[j].SortKeys[k]
			cmp := compare(a, b, sc.numeric)
			if cmp == 0 {
				continue
			}
			if sc.Descending {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})
}

func featureCollectionEnvelope(v Version, members string, matched string, count int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	if v == V110 {

		sb.WriteString(`<wfs:FeatureCollection xmlns:wfs="http://www.opengis.net/wfs" xmlns:gml="http://www.opengis.net/gml" xmlns:ogc="http://www.opengis.net/ogc" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.opengis.net/wfs http://schemas.opengis.net/wfs/1.1.0/wfs.xsd" numberOfFeatures="` + strconv.Itoa(count) + `">` + "\n")
	} else {
		sb.WriteString(`<wfs:FeatureCollection xmlns:wfs="http://www.opengis.net/wfs/2.0" xmlns:gml="http://www.opengis.net/gml/3.2" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.opengis.net/wfs/2.0 http://schemas.opengis.net/wfs/2.0/wfs.xsd" numberMatched="` + matched + `" numberReturned="` + strconv.Itoa(count) + `">` + "\n")
	}
	sb.WriteString(members)
	sb.WriteString("</wfs:FeatureCollection>\n")
	return sb.String()
}
