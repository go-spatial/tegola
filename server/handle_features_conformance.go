package server

import (
	"net/http"

	"github.com/alexeydott/tegola/ogc/features"
)

func (api *FeatureAPI) serveConformance(w http.ResponseWriter, r *http.Request) {
	if !api.discoveryQueryValid(w, r) {
		return
	}
	links := api.representationLinks(r, "/conformance", "application/json", nil)
	classes := api.service.ConformanceClasses()
	// A39: declare Part 4 (CRUD) iff writes are enabled for at least one
	// collection. The declaration matches the actual dispatch capability.
	if api.cfg.Write.Enabled {
		classes = append(classes, features.ConformancePart4)
	}
	api.writeRepresentation(w, r, http.StatusOK, "application/json", struct {
		ConformsTo []string      `json:"conformsTo"`
		Links      []featureLink `json:"links"`
	}{ConformsTo: classes, Links: links}, links)
}
