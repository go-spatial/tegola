package server

import "net/http"

func (api *FeatureAPI) serveLanding(w http.ResponseWriter, r *http.Request) {
	if !api.discoveryQueryValid(w, r) {
		return
	}
	links := api.representationLinks(r, "", "application/json", nil)
	links = append(links,
		api.formatLink(r, "/api", "service-desc", "application/vnd.oai.openapi+json;version=3.0", nil, "json"),
		api.formatLink(r, "/api", "service-doc", "text/html", nil, "html"),
		api.formatLink(r, "/conformance", "conformance", "application/json", nil, featureSelectedFormat(r)),
		api.formatLink(r, "/collections", "data", "application/json", nil, featureSelectedFormat(r)))
	response := struct {
		Title       string        `json:"title,omitempty"`
		Description string        `json:"description,omitempty"`
		Links       []featureLink `json:"links"`
	}{Title: api.cfg.Title, Description: api.cfg.Description, Links: links}
	api.writeRepresentation(w, r, http.StatusOK, "application/json", response, links)
}
