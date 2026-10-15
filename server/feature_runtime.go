package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/feature"
	"github.com/alexeydott/tegola/observability"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/dimfeld/httptreemux"
)

// FeatureAPIConfig supplies immutable HTTP publication settings.
type FeatureAPIConfig struct {
	MaxResponseBytes int64
	QueryTimeout     time.Duration
	BasePath         string
	DefaultLimit     uint
	MaxLimit         uint
	Title            string
	Description      string
	// Write gates Part 4 mutations. Zero value disables all writes.
	Write config.FeaturesWriteConfig
	// Authenticator resolves the request principal for mutation policy
	// checks. Nil means anonymous-only: every mutation runs as
	// feature.Principal{Anonymous: true}. Deployments that expose writes
	// beyond a trusted network MUST plug in an authenticator; the
	// reference policy grants collection operations to any principal
	// the config allows.
	Authenticator Authenticator
}

// Authenticator resolves the mutation principal for a request.
// Implementations must be safe for concurrent use.
type Authenticator interface {
	// Principal returns the actor for policy checks. Returning an
	// anonymous principal is allowed; the policy decides what it may do.
	Principal(r *http.Request) feature.Principal
}

// principal resolves the mutation principal for a request.
func (api *FeatureAPI) principal(r *http.Request) feature.Principal {
	if api.cfg.Authenticator != nil {
		return api.cfg.Authenticator.Principal(r)
	}
	return feature.Principal{Anonymous: true}
}

// FeatureAPI wraps a resolved service. Build it with NewFeatureAPI before routing.
type FeatureAPI struct {
	requestObserver observability.FeatureRequestObserver
	service         *features.Service
	cfg             FeatureAPIConfig
	uriPrefix       string
	// A36: called after successful mutation with mutated collections.
	// Set by router to bump Atlas cache epochs (no global state).
	OnMutate func(collections []string)
	// tileCacheInvalidate is bound to this router, including unknown outcomes.
	tileCacheInvalidate func()
}

// RouterOptions enables explicit feature publication; nil Features preserves legacy behavior.
type RouterOptions struct {
	Features *FeatureAPI
	WFS      *WFSHandler
}

// NewFeatureAPI validates settings and public IDs independently of TOML callers.
func NewFeatureAPI(service *features.Service, cfg FeatureAPIConfig) (*FeatureAPI, error) {
	if cfg.MaxResponseBytes == 0 {
		cfg.MaxResponseBytes = 16 << 20
	}
	if cfg.QueryTimeout == 0 {
		cfg.QueryTimeout = 30 * time.Second
	}
	if cfg.MaxResponseBytes < 1024 || cfg.QueryTimeout <= 0 {
		return nil, fmt.Errorf("features: invalid protocol limits")
	}
	if service == nil {
		return nil, fmt.Errorf("features: nil publication service")
	}
	if err := config.ValidateFeatureBasePath(cfg.BasePath); err != nil {
		return nil, err
	}
	if cfg.DefaultLimit == 0 || cfg.MaxLimit == 0 || cfg.DefaultLimit > cfg.MaxLimit {
		return nil, fmt.Errorf("features: invalid publication limits")
	}
	collections := service.Collections()
	if len(collections) == 0 {
		return nil, fmt.Errorf("features: publication has no collections")
	}
	published := make(map[string]bool, len(collections))
	for _, collection := range collections {
		if err := config.ValidateFeatureCollectionID(collection.ID); err != nil {
			return nil, err
		}
		published[collection.ID] = true
	}
	if err := cfg.Write.Validate(published); err != nil {
		return nil, err
	}
	cfg.Write = cfg.Write.Resolved()
	if err := validateWriteAdmission(service, cfg.Write, cfg.QueryTimeout); err != nil {
		return nil, err
	}
	if cfg.Title == "" {
		cfg.Title = "Tegola Feature API"
	}
	return &FeatureAPI{service: service, cfg: cfg}, nil
}

func validateRouterOptions(options RouterOptions) error {
	if options.Features == nil && options.WFS == nil {
		return nil
	}
	if err := validateFeatureURIPrefix(URIPrefix); err != nil {
		return err
	}
	if options.Features != nil {
		if _, err := NewFeatureAPI(options.Features.service, options.Features.cfg); err != nil {
			return err
		}
		if err := validateFeatureViewerPath(options.Features.cfg.BasePath); err != nil {
			return err
		}
	}
	if options.WFS != nil {
		h := options.WFS
		if h.Service == nil {
			return fmt.Errorf("wfs: nil publication service")
		}
		cfg := h.Config.Resolved()
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := validateFeatureViewerPath(string(cfg.BasePath)); err != nil {
			return err
		}
		published := make(map[string]bool)
		for _, c := range h.Service.Collections() {
			published[c.ID] = true
		}
		if err := h.WriteConfig.Validate(published); err != nil {
			return err
		}
		if err := validateWriteAdmission(h.Service, h.WriteConfig, 30*time.Second); err != nil {
			return err
		}
		if options.Features != nil {
			wfsPath, apiPath := string(cfg.BasePath), options.Features.cfg.BasePath
			if wfsPath == apiPath || strings.HasPrefix(wfsPath, apiPath+"/") || strings.HasPrefix(apiPath, wfsPath+"/") {
				return fmt.Errorf("wfs: WFS and feature API paths must not overlap")
			}
		}
	}
	return nil
}

func validateWriteAdmission(service *features.Service, cfg config.FeaturesWriteConfig, timeout time.Duration) error {
	if !cfg.Enabled {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for _, c := range cfg.Collections {
		p, layer, err := service.MutationProviderFor(string(c.ID))
		if err != nil {
			return err
		}
		descriptor, err := p.DescribeWritable(ctx, layer)
		if err != nil {
			return fmt.Errorf("write admission for collection %q: %w", c.ID, err)
		}
		if cfg.AllowsOperation(string(c.ID), "create") && descriptor.CreateUnsupportedReason != "" {
			return fmt.Errorf("create admission for collection %q: %s", c.ID, descriptor.CreateUnsupportedReason)
		}
	}
	return nil
}

func (api *FeatureAPI) register(router *httptreemux.TreeMux, group *httptreemux.Group, observer observability.APIObserver) {
	// Each router snapshots its own prefix, including when the API is reused.
	bound := *api
	bound.uriPrefix = strings.TrimSuffix(URIPrefix, "/")
	routes := []struct {
		path    string
		handler http.Handler
	}{
		{path: "", handler: http.HandlerFunc(bound.serveLanding)},
		{path: "/api", handler: http.HandlerFunc(bound.serveAPI)},
		{path: "/conformance", handler: http.HandlerFunc(bound.serveConformance)},
		{path: "/collections", handler: http.HandlerFunc(bound.serveCollections)},
		{path: "/collections/:collection", handler: http.HandlerFunc(bound.serveCollection)},
		{path: "/collections/:collection/queryables", handler: http.HandlerFunc(bound.serveQueryables)},
		{path: "/collections/:collection/items", handler: http.HandlerFunc(bound.serveItems)},
		{path: "/collections/:collection/items/:feature", handler: http.HandlerFunc(bound.serveItem)},
		{path: "/*feature_path", handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bound.writeError(w, r, http.StatusNotFound, "NotFound", "Resource not found")
		})},
	}
	for _, route := range routes {
		path := bound.cfg.BasePath + route.path
		mediaType := "application/json"
		if route.path == "/api" {
			mediaType = "application/vnd.oai.openapi+json;version=3.0"
		} else if strings.HasSuffix(route.path, "/queryables") {
			mediaType = "application/schema+json"
		} else if strings.Contains(route.path, "/items") {
			mediaType = "application/geo+json"
		}
		handler := HeadersHandler(featureNoStoreHandler(bound.protocolHandler(bound.negotiate(route.handler, mediaType))))
		if bound.requestObserver != nil {
			// Dedicated outer metrics use fixed resource enums, including unknown
			// paths. Do not feed feature wildcards into legacy dynamic path labels.
			group.UsingContext().Handler(http.MethodGet, path, handler)
			group.UsingContext().Handler(http.MethodHead, path, handler)
		} else {
			group.UsingContext().Handler(observability.InstrumentAPIHandler(http.MethodGet, path, observer, handler))
			group.UsingContext().Handler(observability.InstrumentAPIHandler(http.MethodHead, path, observer, handler))
		}
	}
	// Part 4 mutation routes (POST/PUT/PATCH/DELETE/OPTIONS). Registered
	// only when [features.write] is enabled; per-collection and
	// per-operation checks happen per request.
	bound.registerMutations(group)
	oldOptions := router.OptionsHandler
	router.OptionsHandler = func(w http.ResponseWriter, r *http.Request, params map[string]string) {
		oldOptions(w, r, params)
		if bound.ownsPath(r.URL.Path) {
			w.Header().Set("Allow", "GET, HEAD, OPTIONS")
			featureProtocolHeaders(w.Header())
		}
	}
	oldMethod := router.MethodNotAllowedHandler
	router.MethodNotAllowedHandler = func(w http.ResponseWriter, r *http.Request, methods map[string]httptreemux.HandlerFunc) {
		if !bound.ownsPath(r.URL.Path) {
			oldMethod(w, r, methods)
			return
		}
		setHeaders(w)
		featureProtocolHeaders(w.Header())
		w.Header().Set("Allow", strings.Join(bound.allowForPath(r.URL.Path), ", "))
		bound.writeError(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed", "Method not supported")
	}
}

// allowForPath computes the Allow header for a feature API path,
// including write methods where configured.
func (api *FeatureAPI) allowForPath(path string) []string {
	rel := strings.TrimPrefix(path, api.cfg.BasePath)
	rel = strings.TrimPrefix(rel, "/")
	parts := strings.Split(rel, "/")
	if len(parts) >= 3 && parts[0] == "collections" && parts[2] == "items" {
		collection := parts[1]
		return api.allowedMethods(collection, len(parts) > 3)
	}
	return []string{http.MethodGet, http.MethodHead, http.MethodOptions}
}

func featureNoStoreHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		featureProtocolHeaders(w.Header())
		next.ServeHTTP(w, r)
	})
}

func (api *FeatureAPI) ownsPath(path string) bool {
	base := api.uriPrefix + api.cfg.BasePath
	return path == base || strings.HasPrefix(path, base+"/")
}

type featureLink struct {
	Href  string `json:"href"`
	Rel   string `json:"rel"`
	Type  string `json:"type,omitempty"`
	Title string `json:"title,omitempty"`
}

func (api *FeatureAPI) link(r *http.Request, suffix, relation, mediaType string) featureLink {
	root := URLRoot(r)
	rootPath := strings.Trim(root.Path, "/")
	if rootPath != "" {
		rootPath = "/" + rootPath
	}
	target := url.URL{Scheme: root.Scheme, Host: root.Host, Path: rootPath + api.uriPrefix + api.cfg.BasePath + suffix}
	return featureLink{Href: target.String(), Rel: relation, Type: mediaType}
}

func (api *FeatureAPI) discoveryQueryValid(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.RawQuery != "" {
		api.writeError(w, r, http.StatusBadRequest, "InvalidParameter", "This resource accepts no query parameters")
		return false
	}
	return true
}

// Existing tile-only callers retain their historical prefix behavior. Explicit
// feature publication requires static segments so routes and links agree.
func validateFeatureURIPrefix(prefix string) error {
	if prefix == "/" {
		return nil
	}
	if prefix == "" || !strings.HasPrefix(prefix, "/") || strings.HasSuffix(prefix, "//") {
		return fmt.Errorf("features: invalid static URI prefix")
	}
	path := strings.TrimSuffix(strings.TrimPrefix(prefix, "/"), "/")
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." || strings.ContainsAny(segment, "%:*?#[\\] \t\r\n") {
			return fmt.Errorf("features: URI prefix must contain static unencoded segments")
		}
	}
	return nil
}

// Router preserves TreeMux APIs while guarding feature responses outside route
// handlers, including canonical redirects generated internally by TreeMux.
type Router struct {
	*httptreemux.TreeMux
	featureBasePath string
	featureAPI      *FeatureAPI
}

// ServeHTTP applies feature cache policy to original or canonical feature paths.
// Requests outside the published namespace reach the legacy mux unchanged.
func (router *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if router.featureBasePath != "" {
		owns := func(path string) bool {
			return path == router.featureBasePath || strings.HasPrefix(path, router.featureBasePath+"/")
		}
		if owns(r.URL.Path) || owns(httptreemux.Clean(r.URL.Path)) {
			setHeaders(w)
			featureProtocolHeaders(w.Header())
			if router.featureAPI != nil {
				router.featureAPI.serveObservedFeature(w, r, router.featureBasePath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if router.featureAPI.requestBudgetValid(w, r) {
						router.TreeMux.ServeHTTP(w, r)
					}
				}))
				return
			}
			w = &featureCacheResponseWriter{ResponseWriter: w}
		}
	}
	router.TreeMux.ServeHTTP(w, r)
}

type featureCacheResponseWriter struct{ http.ResponseWriter }

func (w *featureCacheResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *featureCacheResponseWriter) WriteHeader(status int) {
	featureProtocolHeaders(w.Header())
	w.ResponseWriter.WriteHeader(status)
}
func (w *featureCacheResponseWriter) Write(data []byte) (int, error) {
	featureProtocolHeaders(w.Header())
	return w.ResponseWriter.Write(data)
}
