package server

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

type featureRequestKey struct{}
type featureRequestState struct {
	original string
	format   string
}

func featureSelectedFormat(r *http.Request) string {
	s, _ := r.Context().Value(featureRequestKey{}).(featureRequestState)
	return s.format
}
func featureOriginalQuery(r *http.Request) string {
	s, ok := r.Context().Value(featureRequestKey{}).(featureRequestState)
	if ok {
		return s.original
	}
	return r.URL.RawQuery
}

func mergeFeatureHeader(h http.Header, key, token string) {
	for _, v := range h.Values(key) {
		for _, part := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return
			}
		}
	}
	h.Add(key, token)
}
func featureProtocolHeaders(h http.Header) {
	h.Set("Cache-Control", "no-store")
	// ETag is deleted here; Part 4 handlers set a strong validator after
	// this call when the response carries one.
	h.Del("ETag")
	h.Del("Last-Modified")
	h.Del("Content-Encoding")
	mergeFeatureHeader(h, "Vary", "Accept")
	h.Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	mergeFeatureHeader(h, "Access-Control-Expose-Headers", "Content-Crs")
}
func (api *FeatureAPI) requestBudgetValid(w http.ResponseWriter, r *http.Request) bool {
	if len(r.URL.RawQuery) > 65536 {
		api.writeError(w, r, 414, "RequestURITooLong", "Query exceeds publication limit")
		return false
	}
	size := 0
	for _, value := range r.Header.Values("Accept") {
		if len(value) > 16384-size {
			api.writeError(w, r, 431, "RequestHeaderFieldsTooLarge", "Accept exceeds publication limit")
			return false
		}
		size += len(value)
	}
	return true
}
func (api *FeatureAPI) protocolHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !api.requestBudgetValid(w, r) {
			return
		}
		values, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			api.writeError(w, r, 400, "InvalidParameter", "Invalid query parameter")
			return
		}
		format := ""
		if f, present := values["f"]; present {
			if len(f) != 1 || (f[0] != "html" && f[0] != "json") {
				api.writeError(w, r, 400, "InvalidParameter", "Invalid format parameter")
				return
			}
			format = f[0]
			values.Del("f")
		}
		ctx, cancel := context.WithTimeout(r.Context(), api.cfg.QueryTimeout)
		defer cancel()
		ctx = context.WithValue(ctx, featureRequestKey{}, featureRequestState{original: r.URL.RawQuery, format: format})
		business := r.Clone(ctx)
		u := *r.URL
		business.URL = &u
		business.URL.RawQuery = values.Encode()
		next.ServeHTTP(w, business)
	})
}
func (api *FeatureAPI) formatLink(r *http.Request, suffix, relation, mediaType string, parameters url.Values, format string) featureLink {
	values := make(url.Values, len(parameters)+1)
	for k, v := range parameters {
		values[k] = append([]string(nil), v...)
	}
	if format == "" {
		format = "json"
	}
	if format == "html" {
		mediaType = "text/html"
	}
	values.Set("f", format)
	link := api.link(r, suffix, relation, mediaType)
	link.Href += "?" + values.Encode()
	return link
}
func (api *FeatureAPI) representationLinks(r *http.Request, suffix, canonicalMedia string, parameters url.Values) []featureLink {
	format := featureSelectedFormat(r)
	if format == "" {
		format = "json"
	}
	other, media := "html", canonicalMedia
	if format == "html" {
		other = "json"
		media = "text/html"
	}
	alternate := canonicalMedia
	if other == "html" {
		alternate = "text/html"
	}
	return []featureLink{api.formatLink(r, suffix, "self", media, parameters, format), api.formatLink(r, suffix, "alternate", alternate, parameters, other)}
}
