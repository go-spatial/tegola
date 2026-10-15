package server

import (
	"net/http"

	"github.com/alexeydott/tegola/ogc/features"
)

func (api *FeatureAPI) serveAPI(w http.ResponseWriter, r *http.Request) {
	if !api.discoveryQueryValid(w, r) {
		return
	}
	writeCfg := features.OpenAPIWriteConfig{Enabled: api.cfg.Write.Enabled}
	if writeCfg.Enabled {
		writeCfg.Collections = make(map[string][]string, len(api.cfg.Write.Collections))
		for _, c := range api.cfg.Write.Collections {
			writeCfg.Collections[string(c.ID)] = append([]string(nil), c.Operations...)
		}
	}
	document, err := api.service.OpenAPI(features.OpenAPIOptions{
		Title: api.cfg.Title, Description: api.cfg.Description, Version: Version,
		ServerURL:    api.link(r, "", "", "").Href,
		DefaultLimit: api.cfg.DefaultLimit, MaxLimit: api.cfg.MaxLimit,
		HTML: true, MaxResponseBytes: api.cfg.MaxResponseBytes, QueryTimeout: api.cfg.QueryTimeout,
		Write: writeCfg,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "InternalError", "API definition failed")
		return
	}
	const mediaType = "application/vnd.oai.openapi+json;version=3.0"
	api.writeRepresentation(w, r, http.StatusOK, mediaType, document, api.representationLinks(r, "/api", mediaType, nil))
}
