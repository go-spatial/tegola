package server

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/ogc/cql2"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
	"github.com/dimfeld/httptreemux"
)

func (api *FeatureAPI) serveItems(w http.ResponseWriter, r *http.Request) {
	query, parameters, err := api.parseItemsQuery(r.URL.RawQuery)
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "InvalidParameter", "Invalid query parameter")
		return
	}
	id := httptreemux.ContextParams(r.Context())["collection"]
	outputURI, err := api.resolveFeatureCRS(id, parameters, &query)
	if err != nil {
		api.writeQueryError(w, r, err)
		return
	}
	page, err := api.service.QueryCollectionPageWithOptions(r.Context(), id, query, features.QueryOptions{OutputCRS: parameters.Get("crs")})
	if err != nil {
		api.writeQueryError(w, r, err)
		return
	}
	if page.HasMore && page.NumberReturned == 0 {
		api.writeError(w, r, http.StatusInternalServerError, "InternalError", "Invalid provider paging result")
		return
	}
	parameters.Set("limit", strconv.FormatUint(uint64(query.Limit), 10))
	parameters.Set("offset", strconv.FormatUint(query.Offset, 10))
	suffix := "/collections/" + id + "/items"
	links := api.representationLinks(r, suffix, "application/geo+json", parameters)
	if page.HasMore {
		links = append(links, api.pageLink(r, suffix, "next", parameters, query.Offset+page.NumberReturned))
	}
	if query.Offset != 0 {
		previous := uint64(0)
		if query.Offset > uint64(query.Limit) {
			previous = query.Offset - uint64(query.Limit)
		}
		links = append(links, api.pageLink(r, suffix, "prev", parameters, previous))
	}
	for _, feature := range page.Features {
		links = append(links, api.formatLink(r, suffix+"/"+strconv.FormatUint(feature.ID, 10), "item", "application/geo+json", itemRepresentationParameters(parameters), featureSelectedFormat(r)))
	}
	response := struct {
		features.FeatureCollection
		Links []featureLink `json:"links"`
	}{FeatureCollection: page, Links: links}
	w.Header().Set("Content-Crs", "<"+outputURI+">")
	api.writeRepresentation(w, r, http.StatusOK, "application/geo+json", response, links)
}

func (api *FeatureAPI) serveItem(w http.ResponseWriter, r *http.Request) {
	parameters := httptreemux.ContextParams(r.Context())
	id := parameters["collection"]
	rawID := parameters["feature"]
	if !decimalDigits(rawID) {
		api.writeError(w, r, http.StatusBadRequest, "InvalidParameter", "Invalid feature ID")
		return
	}
	featureID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "InvalidParameter", "Invalid feature ID")
		return
	}
	options, outputURI, err := api.selectedItemQuery(r, id)
	if err != nil {
		api.writeQueryError(w, r, err)
		return
	}
	feature, revision, err := api.queryFeatureRevision(r.Context(), id, featureID, options)
	if err != nil {
		api.writeQueryError(w, r, err)
		return
	}
	w.Header().Set("Content-Crs", "<"+outputURI+">")
	// Strong ETag bound to the exact served bytes (including links), for
	// use with If-Match on Part 4 mutations. Set after
	// featureProtocolHeaders, which clears validators.
	raw, media, err := api.renderItemRepresentation(r, id, featureID, feature)
	if err != nil {
		if errors.Is(err, errFeatureResponseTooLarge) {
			api.writeError(w, r, http.StatusBadRequest, "ResponseTooLarge", "Response exceeds publication limit")
		} else {
			api.writeQueryError(w, r, err)
		}
		return
	}
	featureProtocolHeaders(w.Header())
	// R01: prefer the revision-based ETag for CAS consistency with
	// If-Match on mutations; fall back to the representation hash when
	// no revision is available (e.g. read-only providers).
	etag := revisionETag(raw, revision)
	w.Header().Set("ETag", etag)
	mergeFeatureHeader(w.Header(), "Access-Control-Expose-Headers", "ETag")
	w.Header().Set("Content-Type", media)
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		if _, err := w.Write(raw); err != nil {
			log.Error("feature response write failed", "error", err)
		}
	}
}

func (api *FeatureAPI) pageLink(r *http.Request, suffix, relation string, parameters url.Values, offset uint64) featureLink {
	values := make(url.Values, len(parameters))
	for key, value := range parameters {
		values[key] = append([]string(nil), value...)
	}
	values.Set("offset", strconv.FormatUint(offset, 10))
	return api.formatLink(r, suffix, relation, "application/geo+json", values, featureSelectedFormat(r))
}

func (api *FeatureAPI) writeQueryError(w http.ResponseWriter, r *http.Request, err error) {
	var collection features.CollectionNotFoundError
	var feature features.FeatureNotFoundError
	var invalid provider.InvalidFeatureQueryError
	var data provider.FeatureDataError
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		api.writeError(w, r, http.StatusRequestTimeout, "RequestTimeout", "Request did not complete")
	case errors.As(err, &data):
		log.Error("feature source data failed", "error", err)
		api.writeError(w, r, http.StatusInternalServerError, "InternalError", "Feature query failed")
	case errors.As(err, &collection), errors.As(err, &feature):
		api.writeError(w, r, http.StatusNotFound, "NotFound", "Resource not found")
	case errors.As(err, &invalid):
		api.writeError(w, r, http.StatusBadRequest, "InvalidParameter", "Invalid query parameter")
	case errors.Is(err, provider.ErrUnsupported):
		api.writeError(w, r, http.StatusNotImplemented, "UnsupportedQuery", "Query operation is unsupported")
	default:
		log.Error("feature query failed", "error", err)
		api.writeError(w, r, http.StatusInternalServerError, "InternalError", "Feature query failed")
	}
}

func (api *FeatureAPI) parseItemsQuery(raw string) (provider.FeatureQuery, url.Values, error) {
	parameters, err := url.ParseQuery(raw)
	if err != nil {
		return provider.FeatureQuery{}, nil, fmt.Errorf("parse query: %w", err)
	}
	for key, values := range parameters {
		switch key {
		case "limit", "offset", "bbox", "datetime", "filter", "filter-lang", "crs", "bbox-crs":
		default:
			return provider.FeatureQuery{}, nil, fmt.Errorf("unknown query parameter")
		}
		if len(values) != 1 {
			return provider.FeatureQuery{}, nil, fmt.Errorf("repeated query parameter")
		}
	}
	for _, key := range []string{"crs", "bbox-crs"} {
		if value, present := parameters[key]; present && value[0] == "" {
			return provider.FeatureQuery{}, nil, fmt.Errorf("blank CRS")
		}
	}
	if _, present := parameters["bbox-crs"]; present {
		if _, bounded := parameters["bbox"]; !bounded {
			return provider.FeatureQuery{}, nil, fmt.Errorf("bbox-crs requires bbox")
		}
	}
	query := provider.FeatureQuery{Limit: api.cfg.DefaultLimit}
	if value, ok := parameters["limit"]; ok {
		query.Limit, err = parsePageLimit(value[0], api.cfg.MaxLimit)
		if err != nil {
			return provider.FeatureQuery{}, nil, err
		}
	}
	if value, ok := parameters["offset"]; ok {
		if !decimalDigits(value[0]) {
			return provider.FeatureQuery{}, nil, fmt.Errorf("invalid offset")
		}
		query.Offset, err = strconv.ParseUint(value[0], 10, 64)
		if err != nil {
			return provider.FeatureQuery{}, nil, fmt.Errorf("invalid offset: %w", err)
		}
	}
	if value, ok := parameters["bbox"]; ok && parameters.Get("bbox-crs") == "" {
		if err := parseFeatureBounds(value[0], &query); err != nil {
			return provider.FeatureQuery{}, nil, err
		}
	}
	if value, ok := parameters["datetime"]; ok {
		query.Temporal, err = parseFeatureDatetime(value[0])
		if err != nil {
			return provider.FeatureQuery{}, nil, err
		}
	}
	if language, present := parameters["filter-lang"]; present {
		if _, filtered := parameters["filter"]; !filtered || language[0] != "cql2-text" {
			return provider.FeatureQuery{}, nil, fmt.Errorf("invalid filter language")
		}
	}
	if value, present := parameters["filter"]; present {
		if strings.TrimSpace(value[0]) == "" {
			return provider.FeatureQuery{}, nil, fmt.Errorf("blank filter")
		}
		filter, err := cql2.Parse(value[0])
		if err != nil {
			return provider.FeatureQuery{}, nil, err
		}
		query.Filter = &filter
	}
	if err := query.Validate(); err != nil {
		return provider.FeatureQuery{}, nil, err
	}
	return query, parameters, nil
}

func decimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func parsePageLimit(raw string, maximum uint) (uint, error) {
	if !decimalDigits(raw) {
		return 0, fmt.Errorf("invalid limit")
	}
	value := strings.TrimLeft(raw, "0")
	if value == "" {
		return 0, fmt.Errorf("limit must be positive")
	}
	bound := strconv.FormatUint(uint64(maximum), 10)
	if len(value) > len(bound) || (len(value) == len(bound) && value > bound) {
		// No silent clamp: an over-maximum limit is a client error, and
		// clamping it could still blow the response budget with a 500.
		return 0, fmt.Errorf("limit %s exceeds maximum %d", raw, maximum)
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid limit: %w", err)
	}
	return uint(parsed), nil
}

func parseFeatureBounds(raw string, query *provider.FeatureQuery) error {
	parts := strings.Split(raw, ",")
	if len(parts) != 4 && len(parts) != 6 {
		return fmt.Errorf("bbox requires four or six coordinates")
	}
	values := make([]float64, len(parts))
	for i, part := range parts {
		value, err := strconv.ParseFloat(part, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("bbox coordinates must be finite")
		}
		values[i] = value
	}
	maximum := len(parts) / 2
	west, south, east, north := values[0], values[1], values[maximum], values[maximum+1]
	if west < -180 || west > 180 || east < -180 || east > 180 || south < -90 || south > 90 || north < -90 || north > 90 || south > north {
		return fmt.Errorf("bbox outside geographic domain")
	}
	query.BoundsSRID = 4326
	if len(parts) == 4 {
		if west <= east {
			query.Bounds = []geom.Extent{{west, south, east, north}}
		} else {
			query.Bounds = []geom.Extent{{west, south, 180, north}, {-180, south, east, north}}
		}
	} else {
		bottom, top := values[2], values[5]
		if bottom > top {
			return fmt.Errorf("bbox minimum height exceeds maximum")
		}
		query.BoundsVerticalCRS = provider.CRS84h
		if west <= east {
			query.Bounds3D = []provider.Extent3D{{west, south, bottom, east, north, top}}
		} else {
			query.Bounds3D = []provider.Extent3D{{west, south, bottom, 180, north, top}, {-180, south, bottom, east, north, top}}
		}
	}
	return nil
}

func parseFeatureDatetime(raw string) (*provider.TemporalConstraint, error) {
	parts := strings.Split(raw, "/")
	if len(parts) > 2 {
		return nil, fmt.Errorf("invalid datetime interval")
	}
	constraint := &provider.TemporalConstraint{}
	if len(parts) == 1 {
		timestamp, tail, leap, err := parseFeatureTimestamp(parts[0])
		if err != nil {
			return nil, err
		}
		end := *timestamp
		constraint.Start, constraint.End = timestamp, &end
		constraint.StartSubNanosecond, constraint.EndSubNanosecond = tail, tail
		constraint.StartLeapSecond, constraint.EndLeapSecond = leap, leap
	} else {
		for i, part := range parts {
			if part == "" || part == ".." {
				continue
			}
			timestamp, tail, leap, err := parseFeatureTimestamp(part)
			if err != nil {
				return nil, err
			}
			if i == 0 {
				constraint.Start = timestamp
				constraint.StartSubNanosecond = tail
				constraint.StartLeapSecond = leap
			} else {
				constraint.End = timestamp
				constraint.EndSubNanosecond = tail
				constraint.EndLeapSecond = leap
			}
		}
	}
	if err := constraint.Validate(); err != nil {
		return nil, err
	}
	return constraint, nil
}

var featureTimestampSyntax = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}[Tt][0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?([Zz]|[+-][0-9]{2}:[0-9]{2})$`)

func parseFeatureTimestamp(raw string) (*time.Time, string, bool, error) {
	if !featureTimestampSyntax.MatchString(raw) {
		return nil, "", false, fmt.Errorf("invalid datetime syntax")
	}
	if raw[len(raw)-1] != 'Z' && raw[len(raw)-1] != 'z' {
		zone := raw[len(raw)-6:]
		if zone[1:3] > "23" || zone[4:6] > "59" {
			return nil, "", false, fmt.Errorf("invalid datetime offset")
		}
	}
	normalized := raw[:10] + "T" + raw[11:]
	if strings.HasSuffix(normalized, "z") {
		normalized = normalized[:len(normalized)-1] + "Z"
	}
	leap := normalized[17:19] == "60"
	if leap {
		normalized = normalized[:17] + "59" + normalized[19:]
	}
	tail := ""
	if normalized[19] == '.' {
		end := 20
		for end < len(normalized) && normalized[end] >= '0' && normalized[end] <= '9' {
			end++
		}
		if end-20 > 9 {
			tail = strings.TrimRight(normalized[29:end], "0")
			normalized = normalized[:29] + normalized[end:]
		}
	}
	parsed, err := time.Parse(time.RFC3339Nano, normalized)
	if err != nil {
		return nil, "", false, fmt.Errorf("invalid datetime: %w", err)
	}
	if leap && !provider.IsPositiveLeapSecondPredecessor(parsed) {
		return nil, "", false, fmt.Errorf("datetime is not an announced leap second")
	}
	return &parsed, tail, leap, nil
}

func (api *FeatureAPI) resolveFeatureCRS(id string, parameters url.Values, query *provider.FeatureQuery) (string, error) {
	output, err := api.service.DefaultCRSURI(id)
	if err != nil {
		return "", err
	}
	explicitOutput := parameters.Get("crs")
	explicitBounds := parameters.Get("bbox-crs")
	catalog, catalogErr := api.service.CollectionCRS(id)
	if explicitOutput != "" || explicitBounds != "" {
		if catalogErr != nil {
			return "", provider.InvalidFeatureQueryError{Field: "crs", Reason: "CRS publication unavailable"}
		}
	}
	if explicitOutput != "" {
		descriptor, err := catalog.ValidateOutput(explicitOutput)
		if err != nil {
			return "", err
		}
		output = descriptor.URI()
	}
	if query != nil && parameters.Get("bbox") != "" && (explicitBounds != "" || catalogErr == nil) {
		raw := parameters.Get("bbox")
		count := len(strings.Split(raw, ","))
		var descriptor features.CRS
		if explicitBounds != "" {
			descriptor, err = catalog.ValidateBounds(explicitBounds, count)
		} else {
			uri := "http://www.opengis.net/def/crs/OGC/1.3/CRS84"
			if count == 6 {
				uri = provider.CRS84h
			}
			descriptor, err = features.ResolveCRS(uri)
		}
		if err != nil {
			return "", err
		}
		if err := parseFeatureBoundsCRS(raw, descriptor, query); err != nil {
			return "", provider.InvalidFeatureQueryError{Field: "bbox", Reason: "Invalid bounding box"}
		}
	}
	return output, nil
}

func parseFeatureBoundsCRS(raw string, descriptor features.CRS, query *provider.FeatureQuery) error {
	parts := strings.Split(raw, ",")
	if len(parts) != descriptor.Dimension()*2 {
		return fmt.Errorf("bbox dimension mismatch")
	}
	values := make([]float64, len(parts))
	for i, part := range parts {
		value, err := strconv.ParseFloat(part, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("nonfinite bbox")
		}
		values[i] = value
	}
	dimension := descriptor.Dimension()
	minimum, err := descriptor.ToInternalPosition(values[:dimension])
	if err != nil {
		return err
	}
	maximum, err := descriptor.ToInternalPosition(values[dimension:])
	if err != nil {
		return err
	}
	west, south, east, north := minimum[0], minimum[1], maximum[0], maximum[1]
	if south > north || (!descriptor.Geographic() && west > east) {
		return fmt.Errorf("unordered bbox")
	}
	if descriptor.Geographic() && (west < -180 || west > 180 || east < -180 || east > 180 || south < -90 || south > 90 || north < -90 || north > 90) {
		return fmt.Errorf("geographic bbox domain")
	}
	query.Bounds, query.Bounds3D = nil, nil
	query.BoundsSRID = descriptor.InternalSRID()
	query.BoundsCRSDefinition = descriptor.Definition().Definition
	query.BoundsVerticalCRS = ""
	if dimension == 2 {
		if west <= east {
			query.Bounds = []geom.Extent{{west, south, east, north}}
		} else {
			query.Bounds = []geom.Extent{{west, south, 180, north}, {-180, south, east, north}}
		}
	} else {
		bottom, top := minimum[2], maximum[2]
		if bottom > top {
			return fmt.Errorf("unordered height")
		}
		query.BoundsVerticalCRS = provider.CRS84h
		if west <= east {
			query.Bounds3D = []provider.Extent3D{{west, south, bottom, east, north, top}}
		} else {
			query.Bounds3D = []provider.Extent3D{{west, south, bottom, 180, north, top}, {-180, south, bottom, east, north, top}}
		}
	}
	return nil
}

// A feature link preserves output CRS, while collection-only selection and paging
// parameters do not become unsupported parameters on an individual feature.
func itemRepresentationParameters(parameters url.Values) url.Values {
	values := url.Values{}
	if crs := parameters.Get("crs"); crs != "" {
		values.Set("crs", crs)
	}
	return values
}
