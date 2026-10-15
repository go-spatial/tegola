package server

import (
	"net/http"

	"github.com/alexeydott/tegola/provider"
	"github.com/dimfeld/httptreemux"
)

func (api *FeatureAPI) serveQueryables(w http.ResponseWriter, r *http.Request) {
	if !api.discoveryQueryValid(w, r) {
		return
	}
	id := httptreemux.ContextParams(r.Context())["collection"]
	catalog, err := api.service.Queryables(id)
	if err != nil {
		api.writeQueryError(w, r, err)
		return
	}
	metadata, err := api.service.Collection(id)
	if err != nil {
		api.writeQueryError(w, r, err)
		return
	}
	properties := make(map[string]any)
	for _, field := range catalog.Fields() {
		properties[field.Name] = featureQueryableSchema(field)
	}
	links := api.representationLinks(r, "/collections/"+id+"/queryables", "application/schema+json", nil)
	document := map[string]any{"links": links, "$schema": "https://json-schema.org/draft/2020-12/schema", "$id": api.link(r, "/collections/"+id+"/queryables", "", "application/schema+json").Href, "type": "object", "additionalProperties": false, "properties": properties}
	if metadata.Title != "" {
		document["title"] = metadata.Title
	}
	api.writeRepresentation(w, r, http.StatusOK, "application/schema+json", document, links)
}

func featureQueryableSchema(field provider.FeatureQueryable) map[string]any {
	schema := map[string]any{}
	kind := "string"
	switch field.Type {
	case provider.QueryableInteger:
		kind = "integer"
	case provider.QueryableNumber:
		kind = "number"
	case provider.QueryableBoolean:
		kind = "boolean"
	case provider.QueryableDate:
		schema["format"] = "date"
	case provider.QueryableTimestamp:
		schema["format"] = "date-time"
	}
	if field.Nullable {
		schema["type"] = []string{kind, "null"}
	} else {
		schema["type"] = kind
	}
	return schema
}
