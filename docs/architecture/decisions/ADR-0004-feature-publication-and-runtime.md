# ADR-0004: Explicit feature publication and immutable HTTP runtime

Status: Accepted on 2026-10-01; implemented in the current publication runtime.

## Decision

Extend the existing FeatureService catalog and Tegola router. Configuration remains separate from provider resolution; executable wiring remains in cmd. No mutable global feature service is introduced.

### Configuration and binding

`config.FeaturesConfig` contains Enabled bool, BasePath env.String, DefaultLimit and MaxLimit *env.Int, Title and Description env.String, and Collections []FeatureCollectionConfig. Collection entries contain ID, ProviderLayer, Title and Description env.String. TOML names are enabled, basepath, default_limit, max_limit, title, description, collections, id, provider_layer.

Defaults are disabled, /features, default limit 100 and maximum 10000. Explicit nonpositive limits and default greater than maximum fail validation. Base paths consist of one or more unreserved ASCII segments, start with /, and have no trailing slash. Segments . and .. are forbidden. Root paths and first segments maps, capabilities and metrics are reserved. Reject encoded paths, empty segments, router markers, query strings and fragments. Collection IDs use unreserved ASCII characters and cannot be empty, `.` or `..`. Do not normalize IDs.

Provider bindings reuse the existing exactly-one-dot provider.layer grammar. Disabled configuration still undergoes syntactic validation but does not resolve providers or install routes. Enabled configuration requires at least one explicitly mapped collection. Duplicate public IDs, missing sources, MVT sources, unknown feature eligibility and invalid metadata fail startup before listening.

Freeze the service publication interface: `CollectionMetadata { ID, Title, Description string }`, `Collections() []CollectionMetadata` sorted by ascending public ID, and `Collection(id string) (CollectionMetadata, error)` returning CollectionNotFoundError for missing IDs. Accessors return detached value snapshots.

`cmd/internal/register.Features(cfg config.FeaturesConfig, providers map[string]provider.TilerUnion) (*features.Service, error)` resolves exact source metadata and delegates to NewService. Disabled returns nil, nil. Titles/descriptions are copied into CollectionSource and immutable catalog entries; public catalog access returns detached value snapshots. Unknown extents are omitted; map presentation bounds are not source extents.

### HTTP runtime and compatibility

Introduce `server.FeatureAPIConfig` containing BasePath string, DefaultLimit and MaxLimit uint, Title and Description string. `server.NewFeatureAPI(service *features.Service, cfg FeatureAPIConfig) (*FeatureAPI, error)` validates and copies settings into private fields. Nil service is invalid here. `server.RouterOptions` contains Features *FeatureAPI; nil means disabled.

Introduce named constructors `NewRouterWithOptions(a *atlas.Atlas, options RouterOptions) (*Router, error)` and `StartWithOptions(a *atlas.Atlas, port string, options RouterOptions) (*http.Server, error)`. Preserve existing NewRouter and Start signatures as wrappers with empty options. Validate feature routes before router registration and before starting a listener. Existing wrapper defaults must always be valid.

Use an internal infallible router assembly helper after validation; legacy wrappers invoke it with empty options, rather than swallowing errors. NewFeatureAPI validates catalog IDs even for callers bypassing TOML; option-aware constructors reject uninitialized FeatureAPI values. Router validation rejects feature basepaths whose first segment collides with actual embedded viewer file or directory prefixes. Implement that check through build-specific viewer helpers; noViewer has no viewer prefixes. This protects index.html, favicon.ico, assets and other actual resources without coupling config to UI. Apply no-store after custom/standard headers.

Resolve the publication service while the provider map is available at CLI initialization. Carry the immutable FeatureAPI through a private typed Cobra context key. Preserve initialization compatibility through a wrapper if needed. Lambda uses the same binder and explicit router options; enabled publication must never be silently omitted.

Compatibility amendment: this repository's vendored Cobra has no Context/SetContext API. Instead carry FeatureAPI in closures scoped to the root/server command instance, installed during command assembly. The pre-run initializes the runtime before the server closure reads it; reset the carrier before each initialization, including failures, to prevent stale reuse. Request handlers receive only the constructed immutable runtime. Do not add a package-global runtime or modify vendored Cobra to simulate context. Preserve initConfig compatibility with an internal initializer returning the runtime.

Register under the existing URI-prefix group before the viewer fallback. Compose URLRoot, URI prefix and feature basepath once. Reuse instrumentation and standard headers/CORS. Feature responses use Cache-Control: no-store, including errors; tile caching and maintenance authorization do not govern feature routes. Explicitly published reads have the existing server's read-access policy, including external proxy protection when configured.

### Discovery boundary

Negotiation uses one canonical representation per resource: application/json for discovery, application/vnd.oai.openapi+json;version=3.0 for /api, and application/geo+json for items/item. Absent Accept and matching wildcard ranges are accepted. Unsupported or malformed Accept receives406 before provider I/O, with generic JSON error, HEAD suppression and no-store. Highest matching range specificity controls quality, so explicit q=0 overrides a broader wildcard; equal-specificity duplicates use maximum quality. Honour media parameters and quoted delimiters; matching parameter count participates in specificity. Join all Accept header values and tolerate empty HTTP list members as RFC 9110 requires. Quality follows the HTTP qvalue grammar (0–1, at most three fractional digits). The initial API supplies no application/json alias for feature payloads; later representations require corresponding implementations and tests. Existing tile/viewer negotiation remains unchanged.

HTTP publication implements landing, /api, /conformance, /collections and collection resources. Supply a valid minimal OpenAPI definition matching implemented routes. Conformance advertises only verified implemented classes; do not announce Core merely because discovery is installed. HEAD preserves GET status/headers and suppresses bodies. The accepted user decision requires correct four- and six-number bbox support for item queries; dimensional semantics are specified in a separate reviewed ADR before dependent implementation.

## Verification

Configuration parsing/defaults/invalid paths and IDs, explicit binding and startup failure, disabled behavior, immutable metadata, legacy router callers, URI prefix/proxy links, HEAD and no-store require focused tests. Independent reviews precede checkpoint acceptance. CLI and Lambda wiring must compile in applicable build modes.

## Consequences

Source-error classification amendment: `provider.FeatureDataError { Err error }` unwraps encountered source-row/decode/integrity errors. HTTP recognizes it before InvalidFeatureQueryError and returns generic500. Request syntax and structural query errors remain400; unsupported requested capabilities remain501. Context cancellation/deadlines retain their independent mapping and must not be reclassified as corrupt data. Independent review confirmed the previous source-row InvalidFeatureQueryError chain otherwise incorrectly produced400 for valid requests. Preserve underlying error chains without exposing source details in responses.

Feature publication is explicit and defaults off. Existing tile and viewer behavior is preserved. The exactly-one-dot binding restricts source names as existing map bindings do. The current source also implements OpenAPI generation and a shared-router Lambda adapter; deployment acceptance remains installation-specific.

Router compatibility amendment: the option-aware constructor returns *server.Router embedding *httptreemux.TreeMux. Its ServeHTTP captures the immutable mounted feature base and sets no-store before delegation for original or TreeMux-equivalent cleaned feature paths, including automatic redirects. Legacy NewRouter still returns *httptreemux.TreeMux. StartWithOptions and Lambda must use Router.ServeHTTP through http.Handler; bypassing through the embedded TreeMux is not the serving interface. Validate enabled-feature URI prefixes as static paths (root and one trailing slash allowed), without changing legacy disabled behavior. Verify trailing/clean redirects, custom headers and nonfeature redirect parity. The vendored TreeMux has no redirect hook; no dependency patch is introduced.
