package features

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// OpenAPIOptions describes the installed HTTP publication, not provider settings.
type OpenAPIOptions struct {
	Title, Description, Version, ServerURL string
	DefaultLimit, MaxLimit                 uint
	HTML                                   bool
	MaxResponseBytes                       int64
	QueryTimeout                           time.Duration
	Write                                  OpenAPIWriteConfig
}

// OpenAPIWriteConfig describes enabled mutation operations per collection
// for the OpenAPI document. Operations use Part 4 vocabulary.
type OpenAPIWriteConfig struct {
	Enabled     bool
	Collections map[string][]string // collection ID -> operations
}

// OpenAPI builds a detached OpenAPI 3.0.3 document from frozen service metadata.
// It performs no provider I/O and retains no caller-owned values.
func (s *Service) OpenAPI(options OpenAPIOptions) (map[string]any, error) {
	if s == nil || options.Version == "" || options.DefaultLimit == 0 ||
		options.MaxLimit < options.DefaultLimit || options.MaxResponseBytes < 1024 || options.QueryTimeout <= 0 {
		return nil, fmt.Errorf("features: invalid OpenAPI publication options")
	}
	serverURL, err := url.Parse(options.ServerURL)
	if err != nil || len(options.ServerURL) > 8192 || options.ServerURL == "" ||
		serverURL.User != nil || serverURL.RawQuery != "" || serverURL.Fragment != "" ||
		strings.ContainsAny(options.ServerURL, "{}\r\n") ||
		(serverURL.Scheme != "http" && serverURL.Scheme != "https" && serverURL.Scheme != "") ||
		(serverURL.Scheme != "" && serverURL.Host == "") ||
		(serverURL.Scheme == "" && (serverURL.Host != "" || !strings.HasPrefix(serverURL.Path, "/"))) {
		return nil, fmt.Errorf("features: invalid OpenAPI server URL")
	}
	paths := map[string]any{}
	add := func(path, id, summary, media, schema string, parameters []any, spatial bool, unsupported []string) {
		paths[path] = openAPIPath(id, summary, media, schema, parameters, spatial, unsupported, options)
	}
	add("/", "Landing", "Feature API landing page", "application/json", "LandingPage", nil, false, nil)
	add("/api", "APIDefinition", "API definition", "application/vnd.oai.openapi+json;version=3.0", "APIDefinition", nil, false, nil)
	add("/conformance", "Conformance", "Implemented conformance declarations", "application/json", "Conformance", nil, false, nil)
	add("/collections", "Collections", "Published collections", "application/json", "Collections", nil, false, nil)
	for _, metadata := range s.Collections() {
		if !openAPICollectionID(metadata.ID) {
			return nil, fmt.Errorf("features: invalid OpenAPI collection path")
		}
		collection := s.collections[metadata.ID]
		base := "/collections/" + metadata.ID
		id := hex.EncodeToString([]byte(metadata.ID))
		add(base, "Collection_"+id, "Collection metadata", "application/json", "Collection", nil, false, nil)
		parameters := []any{
			openAPIParameter("limit", "Page size. Values above the maximum are rejected with 400.", map[string]any{
				"type": "integer", "minimum": 1, "maximum": options.MaxLimit, "default": options.DefaultLimit,
			}),
			openAPIParameter("offset", "Tegola extension: stable feature-ID paging offset, decimal uint64 (maximum 18446744073709551615).", map[string]any{"type": "string", "pattern": "^[0-9]+$", "default": "0"}),
			openAPIParameter("bbox", "Four or six finite coordinates. With bbox-crs omitted, CRS84 longitude/latitude or CRS84h longitude/latitude/height applies respectively; antimeridian wrapping is supported.", map[string]any{
				"type": "array", "items": map[string]any{"type": "number"}, "minItems": 4, "maxItems": 6,
				"oneOf": []any{map[string]any{"minItems": 4, "maxItems": 4}, map[string]any{"minItems": 6, "maxItems": 6}},
			}),
		}
		var unsupported []string
		parameters = append(parameters, openAPIParameter("datetime", "RFC3339 instant or closed interval with at most one open endpoint. Exact fractions, offsets and known positive leap seconds are supported. Features without temporal geometry match every valid datetime; collections without a temporal mapping therefore retain all other query conditions.", map[string]any{"type": "string"}))
		if collection.queryablesAvailable {
			add(base+"/queryables", "Queryables_"+id, "Queryable property schema", "application/schema+json", "Queryables", nil, false, nil)
			parameters = append(parameters,
				openAPIParameter("filter", "CQL2 text over the collection Queryables; combines with bbox and datetime. Maximum encoded expression size is 65536 bytes, including UTF-8 bytes.", map[string]any{"type": "string", "minLength": 1, "maxLength": 65536}),
				openAPIParameter("filter-lang", "Requires filter; omitted language defaults to cql2-text. Other encodings and filter-crs are unsupported.", map[string]any{"type": "string", "enum": []string{"cql2-text"}, "default": "cql2-text"}),
			)
		} else {
			unsupported = append(unsupported, "filter", "filter-lang")
		}
		var output []any
		var itemUnsupported []string
		if collection.crsAvailable {
			output = []any{openAPIParameter("crs", "Output CRS: exact collection URI; coordinate axes follow its definition and height is preserved.", map[string]any{"type": "string", "format": "uri", "enum": collection.crs.URIs(), "default": collection.crs.DefaultURI()})}
			parameters = append(parameters, openAPIParameter("bbox-crs", "Requires bbox; exact collection URI. Bbox coordinate order and dimensionality follow this CRS.", map[string]any{"type": "string", "format": "uri", "enum": collection.crs.URIs()}))
		} else {
			unsupported = append(unsupported, "bbox-crs", "crs")
			itemUnsupported = []string{"crs"}
		}
		parameters = append(parameters, output...)
		add(base+"/items", "Items_"+id, "Features in the collection", "application/geo+json", "FeatureCollection", parameters, true, unsupported)
		itemParameters := []any{map[string]any{"name": "feature", "in": "path", "required": true, "description": "Decimal uint64 feature ID, maximum 18446744073709551615.", "schema": map[string]any{"type": "string", "pattern": "^[0-9]+$"}}}
		itemParameters = append(itemParameters, output...)
		add(base+"/items/{feature}", "Item_"+id, "One feature", "application/geo+json", "Feature", itemParameters, true, itemUnsupported)
		// Part 4 (Features: Create-Replace-Update-Delete) write operations.
		if ops := options.Write.Collections[metadata.ID]; options.Write.Enabled && len(ops) > 0 {
			addWritePaths(paths, base, id, ops)
		}
	}
	return map[string]any{
		"openapi": "3.0.3",
		"info":    map[string]any{"title": options.Title, "description": options.Description, "version": options.Version},
		"servers": []any{map[string]any{"url": options.ServerURL}}, "paths": paths,
		"components":                  map[string]any{"schemas": openAPISchemas()},
		"x-tegola-max-response-bytes": options.MaxResponseBytes,
		"x-tegola-query-timeout-ms":   options.QueryTimeout.Milliseconds(),
	}, nil
}

func openAPICollectionID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	for _, c := range []byte(id) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-._~", rune(c))) {
			return false
		}
	}
	return true
}

func openAPIParameter(name, description string, schema map[string]any) map[string]any {
	return map[string]any{"name": name, "in": "query", "required": false, "description": description, "style": "form", "explode": false, "schema": schema}
}

func openAPIRef(name string) map[string]any {
	return map[string]any{"$ref": "#/components/schemas/" + name}
}

func openAPIHeaders(spatial bool) map[string]any {
	headers := map[string]any{
		"Content-Type":   map[string]any{"description": "Selected canonical JSON media type or text/html; charset=utf-8 for success; application/json for errors.", "schema": map[string]any{"type": "string"}},
		"Cache-Control":  map[string]any{"schema": map[string]any{"type": "string", "enum": []string{"no-store"}}},
		"Vary":           map[string]any{"description": "Includes Accept and preserves configured values.", "schema": map[string]any{"type": "string"}},
		"Content-Length": map[string]any{"schema": map[string]any{"type": "string", "pattern": "^[0-9]+$"}},
	}
	if spatial {
		headers["Content-Crs"] = map[string]any{"description": "Output CRS URI enclosed in angle brackets, including defaults and empty pages.", "schema": map[string]any{"type": "string"}}
	}
	return headers
}

func openAPIPath(id, summary, media, schema string, parameters []any, spatial bool, unsupported []string, options OpenAPIOptions) map[string]any {
	path := map[string]any{}
	for _, method := range []string{"get", "head"} {
		params := append([]any{}, parameters...)
		formats := []string{"json"}
		if options.HTML {
			formats = append(formats, "html")
		}
		params = append(params, openAPIParameter("f", "Selects the canonical JSON profile or HTML, overriding Accept. Empty, repeated or unknown values are rejected.", map[string]any{"type": "string", "enum": formats}))
		success := map[string]any{"description": "Successful response", "headers": openAPIHeaders(spatial)}
		if method == "get" {
			content := map[string]any{media: map[string]any{"schema": openAPIRef(schema)}}
			if options.HTML {
				content["text/html"] = map[string]any{"schema": map[string]any{"type": "string"}}
			}
			success["content"] = content
		}
		responses := map[string]any{"200": success}
		redirect := map[string]any{
			"description": "Canonical path redirect for trailing slash or cleaned path; no validators are emitted.",
			"headers": map[string]any{
				"Location":      map[string]any{"schema": map[string]any{"type": "string"}},
				"Cache-Control": map[string]any{"schema": map[string]any{"type": "string", "enum": []string{"no-store"}}},
			},
		}
		if method == "get" {
			redirect["content"] = map[string]any{"text/html": map[string]any{"schema": map[string]any{"type": "string"}}}
		}
		responses["301"] = redirect
		for status, description := range map[string]string{
			"400": "Invalid, repeated, blank or unknown query parameter; or the response exceeds the publication limit (reduce limit)",
			"404": "Resource not found", "405": "Method not supported; Allow: GET, HEAD, OPTIONS",
			"406": "Requested representation is unavailable", "408": "Request cancelled or deadline exceeded",
			"414": "Raw query exceeds 65536 bytes", "431": "Aggregate Accept headers exceed 16384 bytes",
			"500": "Resource failed, including source integrity or encoding",
			"501": "Query operation is unsupported",
		} {
			response := map[string]any{"description": description, "headers": openAPIHeaders(false)}
			if method == "get" {
				response["content"] = map[string]any{"application/json": map[string]any{"schema": openAPIRef("Error")}}
			}
			responses[status] = response
		}
		operation := map[string]any{"operationId": method + id, "summary": summary, "parameters": params, "responses": responses}
		if len(unsupported) > 0 {
			operation["x-tegola-unsupported-parameters"] = append([]string{}, unsupported...)
			operation["description"] = "The listed optional parameters are accepted only for error compatibility on this collection; they cannot produce a successful query and return 400 or 501. They are not successful capabilities."
		}
		path[method] = operation
	}
	optionsOperation := map[string]any{"operationId": "options" + id, "summary": "Supported methods and CORS policy", "responses": map[string]any{"200": map[string]any{"description": "OPTIONS response without a body", "headers": map[string]any{"Allow": map[string]any{"schema": map[string]any{"type": "string", "enum": []string{"GET, HEAD, OPTIONS"}}}}}}}
	var pathParameters []any
	for _, value := range parameters {
		parameter := value.(map[string]any)
		if parameter["in"] == "path" {
			pathParameters = append(pathParameters, parameter)
		}
	}
	if len(pathParameters) != 0 {
		optionsOperation["parameters"] = pathParameters
	}
	path["options"] = optionsOperation
	return path
}

func openAPISchemas() map[string]any {
	stringSchema := func() map[string]any { return map[string]any{"type": "string"} }
	array := func(items map[string]any) map[string]any { return map[string]any{"type": "array", "items": items} }
	object := func(required []string, properties map[string]any) map[string]any {
		return map[string]any{"type": "object", "required": required, "properties": properties}
	}
	uint64Schema := func() map[string]any {
		return map[string]any{"type": "integer", "minimum": 0, "maximum": json.Number("18446744073709551615")}
	}
	links := func() map[string]any { return array(openAPIRef("Link")) }
	schemas := map[string]any{
		"Link":          object([]string{"href", "rel"}, map[string]any{"href": stringSchema(), "rel": stringSchema(), "type": stringSchema(), "title": stringSchema()}),
		"LandingPage":   object([]string{"links"}, map[string]any{"title": stringSchema(), "description": stringSchema(), "links": links()}),
		"Conformance":   object([]string{"conformsTo"}, map[string]any{"conformsTo": array(stringSchema()), "links": links()}),
		"Collection":    object([]string{"id", "itemType", "links"}, map[string]any{"id": stringSchema(), "title": stringSchema(), "description": stringSchema(), "itemType": map[string]any{"type": "string", "enum": []string{"feature"}}, "links": links(), "crs": array(stringSchema()), "storageCrs": stringSchema()}),
		"Collections":   object([]string{"collections", "links"}, map[string]any{"collections": array(openAPIRef("Collection")), "links": links()}),
		"Error":         object([]string{"code", "description"}, map[string]any{"code": stringSchema(), "description": stringSchema()}),
		"Queryables":    object([]string{"$schema", "$id", "type", "additionalProperties", "properties"}, map[string]any{"$schema": stringSchema(), "$id": stringSchema(), "type": map[string]any{"type": "string", "enum": []string{"object"}}, "title": stringSchema(), "additionalProperties": map[string]any{"type": "boolean", "enum": []bool{false}}, "properties": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "object"}}}),
		"APIDefinition": object([]string{"openapi", "info", "paths"}, map[string]any{"openapi": map[string]any{"type": "string", "enum": []string{"3.0.3"}}, "info": map[string]any{"type": "object"}, "paths": map[string]any{"type": "object"}}),
		"Position":      map[string]any{"type": "array", "items": map[string]any{"type": "number"}, "minItems": 2, "maxItems": 3},
	}
	coordinates := func(depth int) map[string]any {
		result := openAPIRef("Position")
		for i := 0; i < depth; i++ {
			result = array(result)
		}
		return result
	}
	for name, depth := range map[string]int{"Point": 0, "MultiPoint": 1, "LineString": 1, "MultiLineString": 2, "Polygon": 2, "MultiPolygon": 3} {
		schemas[name] = object([]string{"type", "coordinates"}, map[string]any{"type": map[string]any{"type": "string", "enum": []string{name}}, "coordinates": coordinates(depth)})
	}
	// Keep branch order fixed so document bytes do not depend on Go map iteration.
	geometries := []any{openAPIRef("Point"), openAPIRef("MultiPoint"), openAPIRef("LineString"), openAPIRef("MultiLineString"), openAPIRef("Polygon"), openAPIRef("MultiPolygon"), openAPIRef("GeometryCollection")}
	schemas["GeometryCollection"] = object([]string{"type", "geometries"}, map[string]any{"type": map[string]any{"type": "string", "enum": []string{"GeometryCollection"}}, "geometries": array(openAPIRef("Geometry"))})
	schemas["Geometry"] = map[string]any{"oneOf": geometries}
	schemas["NullableGeometry"] = map[string]any{"oneOf": []any{openAPIRef("Geometry"), map[string]any{"type": "object", "nullable": true, "enum": []any{nil}}}}
	schemas["Feature"] = object([]string{"type", "id", "geometry", "properties"}, map[string]any{"type": map[string]any{"type": "string", "enum": []string{"Feature"}}, "id": uint64Schema(), "geometry": openAPIRef("NullableGeometry"), "properties": map[string]any{"type": "object", "additionalProperties": true}, "links": links()})
	schemas["FeatureInput"] = object([]string{"type", "geometry", "properties"}, map[string]any{"type": map[string]any{"type": "string", "enum": []string{"Feature"}}, "geometry": openAPIRef("NullableGeometry"), "properties": map[string]any{"type": "object", "additionalProperties": true}})
	schemas["FeatureCollection"] = object([]string{"type", "features", "numberReturned", "links"}, map[string]any{"type": map[string]any{"type": "string", "enum": []string{"FeatureCollection"}}, "features": array(openAPIRef("Feature")), "numberReturned": uint64Schema(), "numberMatched": uint64Schema(), "links": links()})
	return schemas
}

// addWritePaths adds OGC API Features Part 4 mutation operations for one
// collection to the OpenAPI paths map.
func addWritePaths(paths map[string]any, base, id string, ops []string) {
	has := func(op string) bool {
		for _, o := range ops {
			if strings.EqualFold(o, op) {
				return true
			}
		}
		return false
	}
	featureIDParam := map[string]any{"name": "feature", "in": "path", "required": true,
		"description": "Decimal uint64 feature ID.", "schema": map[string]any{"type": "string", "pattern": "^[0-9]+$"}}
	geoJSONBody := func(desc string) map[string]any {
		return map[string]any{"description": desc, "required": true,
			"content": map[string]any{"application/geo+json": map[string]any{"schema": openAPIRef("FeatureInput")}}}
	}
	ifMatch := map[string]any{"name": "If-Match", "in": "header", "required": false,
		"description": "Strong ETag precondition. Required when require_if_match is enabled (428 if missing); stale validators are rejected with 412.",
		"schema":      map[string]any{"type": "string"}}
	// Build per-method entries explicitly to keep operation shapes clear.
	if has("create") {
		existing, _ := paths[base+"/items"].(map[string]any)
		if existing == nil {
			existing = map[string]any{}
		}
		existing["post"] = map[string]any{
			"operationId": "Create_" + id, "summary": "Create a feature",
			"requestBody": geoJSONBody("GeoJSON Feature to create"),
			"responses": map[string]any{
				"201": map[string]any{"description": "Created feature", "content": map[string]any{"application/geo+json": map[string]any{"schema": openAPIRef("Feature")}}},
				"400": map[string]any{"description": "Invalid input"},
			},
		}
		paths[base+"/items"] = existing
	}
	itemOps := map[string]any{}
	if has("replace") {
		itemOps["put"] = map[string]any{
			"operationId": "Replace_" + id, "summary": "Replace a feature",
			"parameters":  []any{featureIDParam, ifMatch},
			"requestBody": geoJSONBody("Replacement GeoJSON Feature"),
			"responses": map[string]any{
				"200": map[string]any{"description": "Replaced feature", "content": map[string]any{"application/geo+json": map[string]any{"schema": openAPIRef("Feature")}}},
				"400": map[string]any{"description": "Invalid input"},
				"412": map[string]any{"description": "Precondition failed"},
			},
		}
	}
	if has("update") {
		itemOps["patch"] = map[string]any{
			"operationId": "Update_" + id, "summary": "Partially update a feature",
			"parameters": []any{featureIDParam, ifMatch},
			"requestBody": map[string]any{"description": "Merge Patch or JSON Patch document", "required": true,
				"content": map[string]any{
					"application/merge-patch+json": map[string]any{"schema": map[string]any{"type": "object"}},
					"application/json-patch+json":  map[string]any{"schema": map[string]any{"type": "array", "items": map[string]any{"type": "object"}}},
				}},
			"responses": map[string]any{
				"200": map[string]any{"description": "Updated feature", "content": map[string]any{"application/geo+json": map[string]any{"schema": openAPIRef("Feature")}}},
				"400": map[string]any{"description": "Invalid input"},
				"412": map[string]any{"description": "Precondition failed"},
			},
		}
	}
	if has("delete") {
		itemOps["delete"] = map[string]any{
			"operationId": "Delete_" + id, "summary": "Delete a feature",
			"parameters": []any{featureIDParam, ifMatch},
			"responses": map[string]any{
				"204": map[string]any{"description": "Deleted"},
				"400": map[string]any{"description": "Invalid input"},
				"412": map[string]any{"description": "Precondition failed"},
			},
		}
	}
	if len(itemOps) > 0 {
		// Merge with the existing GET item path entry.
		existing, _ := paths[base+"/items/{feature}"].(map[string]any)
		if existing == nil {
			existing = map[string]any{}
		}
		for k, v := range itemOps {
			existing[k] = v
		}
		paths[base+"/items/{feature}"] = existing
	}
	if len(ops) > 0 {
		paths[base+"/schema"] = map[string]any{
			"get": map[string]any{
				"operationId": "Schema_" + id, "summary": "JSON Schema for feature mutation input",
				"responses": map[string]any{
					"200": map[string]any{"description": "JSON Schema", "content": map[string]any{"application/schema+json": map[string]any{"schema": map[string]any{"type": "object"}}}},
				},
			},
		}
	}
	for _, path := range []string{base + "/items", base + "/items/{feature}"} {
		entry, _ := paths[path].(map[string]any)
		allowed := []string{"GET", "HEAD", "OPTIONS"}
		for _, method := range []string{"post", "put", "patch", "delete"} {
			operation, exists := entry[method].(map[string]any)
			if !exists {
				continue
			}
			allowed = append(allowed, strings.ToUpper(method))
			responses := operation["responses"].(map[string]any)
			if method == "put" || method == "patch" {
				responses["204"] = map[string]any{"description": "Write committed, but source representation is unavailable. Do not repeat the mutation.", "headers": map[string]any{"Tegola-Commit-Status": map[string]any{"schema": map[string]any{"type": "string", "enum": []string{"committed"}}}}}
			}
			for status, description := range map[string]string{
				"401": "Authentication required", "403": "Write is forbidden", "404": "Feature or collection not found",
				"409": "Mutation conflicts with source constraints", "415": "Unsupported request media type",
				"423": "Feature is locked", "428": "Required precondition missing", "500": "Write or readback failed; verify source before retrying",
			} {
				responses[status] = map[string]any{"description": description}
			}
			for _, status := range []string{"200", "201"} {
				if success, ok := responses[status].(map[string]any); ok {
					headers := map[string]any{"ETag": map[string]any{"description": "Validator for persisted representation", "schema": map[string]any{"type": "string"}}}
					if status == "201" {
						headers["Location"] = map[string]any{"schema": map[string]any{"type": "string"}}
						headers["Tegola-Commit-Status"] = map[string]any{"description": "committed when creation succeeded but source readback failed; response body may be absent", "schema": map[string]any{"type": "string"}}
					}
					success["headers"] = headers
				}
			}
		}
		if options, ok := entry["options"].(map[string]any); ok {
			response := options["responses"].(map[string]any)["200"].(map[string]any)
			response["headers"] = map[string]any{"Allow": map[string]any{"schema": map[string]any{"type": "string", "enum": []string{strings.Join(allowed, ", ")}}}}
		}
	}
}
