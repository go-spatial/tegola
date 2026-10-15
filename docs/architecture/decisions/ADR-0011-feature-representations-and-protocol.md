# Read ADR-0011: Feature representations, API definition and protocol limits

Status: Accepted after independent architecture and security review on 2026-10-02.
Limit and response-size error behavior amended with PR #1 on 2026-10-03.

## Decision

Extend the existing FeatureService and feature routes. Provider query contracts,
tile routes and the embedded tile viewer retain their existing ownership.

### API definition

`Service.OpenAPI(OpenAPIOptions)` builds a detached OpenAPI 3.0.3 document from
the immutable publication catalog without provider I/O. Transport options carry
the public server URL, publication limits and available representations. Literal
collection paths express collection-specific capabilities. Operation identifiers
are deterministic and unique, including separate GET and HEAD operations.

Use local reference schemas for discovery, features, errors and Queryables.
Describe uint64 IDs without coercing them into signed int64. Preserve absent and
null geometry and the supported coordinate dimensions. The Queryables resource
uses JSON Schema; its Draft 2020-12 document is not itself an OpenAPI 3.0 schema.

Advertise Core datetime for every collection. Validate its syntax before provider
I/O; a collection without temporal geometry matches every feature for a valid
datetime constraint, as required by Core requirements 25 and 26. Collections with
an admitted temporal mapping retain exact provider filtering. Advertise filtering,
Queryables and referenced CRS support only where the collection supports it.
Document existing error behavior for unsupported optional parameters separately.
The configured `limit.maximum`
remains in the parameter schema; requests above it receive 400 `InvalidParameter`
before provider I/O. Include actual defaults, media types, headers,
errors and enabled operations. Validate the generated document with an independent
OpenAPI validator, including negative controls.

### Representations and links

Every successful feature resource offers its canonical JSON representation and
`text/html; charset=utf-8`, including the API definition and Queryables resources.
HTML link metadata uses the media type `text/html`; the response Content-Type
retains its UTF-8 charset.
Absent Accept defaults to the canonical JSON media type. For Accept negotiation,
use the existing RFC 9110 range matching, specificity and qvalue rules. Compare
available representations by quality, then range specificity and parameter count;
an exact tie chooses canonical JSON. An explicit exclusion overrides a broader
wildcard for that representation.

An optional, single `f=html|json` selects HTML or the resource's canonical JSON
profile and overrides Accept. Empty, repeated or unknown format values receive
400 before provider I/O. Middleware validates the original query, removes only
`f` from a cloned business request, and retains the original query and format in
request-local context. All other unknown or malformed parameters retain their
existing rejection behavior.

Self, paging and alternate links preserve validated data parameters and the
mounted proxy/URI prefix exactly once. Browser links to representations use an
explicit `f` selector. Alternate links identify their media types. Collection
navigation advertises optional resources only when available.

HTML uses embedded `html/template` resources with no external CDN or JavaScript
dependency. It displays the complete response model, including escaped JSON
details, and renders all genuine protocol links as anchors. Property values and
metadata are escaped; a user property named `links` is not a protocol link source.
Queries, geometry transformations and capability decisions remain outside templates.

### HTTP policy

Feature responses retain `Cache-Control: no-store`. Remove ETag and Last-Modified
at response commitment, including redirects, OPTIONS and errors; conditional
requests do not produce 304. Merge `Vary: Accept` with configured headers.
Feature payloads use identity encoding; remove configured Content-Encoding at
commitment rather than labeling uncompressed bytes as compressed content.
Successful HEAD responses use the same selected representation, status and
Content-Length as GET and emit no body. Error responses retain the generic JSON
error schema, including when HTML was requested. Preserve Content-Crs for
successful spatial representations; clear it on errors.

Feature-only CORS advertises GET, HEAD and OPTIONS, exposes Content-Crs, and
preserves configured origin, credentials and other headers. OPTIONS returns 200
with deterministic `Allow: GET, HEAD, OPTIONS`; unsupported methods receive 405
with the same Allow value. Existing URLRoot/HostName and URI-prefix behavior
remains authoritative; do not introduce trust in arbitrary forwarded prefixes.

Publication defaults are a 30-second cooperative query deadline and a 16 MiB
encoded response cap. Configure them through `query_timeout_ms` and
`max_response_bytes`. Runtime zero values resolve to defaults. Explicit timeouts
must be positive; explicit response caps must be at least 1024 bytes so generic
error responses fit. Validate configuration before routing and carry the settings
through CLI and Lambda wiring.

Reject a raw query above 64 KiB with 414 and aggregate Accept headers above 16 KiB
with 431 before parsing or provider I/O. These fixed guards also apply to HEAD.
Measure the complete selected response before committing headers. Oversized
responses produce generic 400 `ResponseTooLarge`, without a partial success body;
HEAD reports the corresponding error headers and suppresses its body.

Retain the existing 408 mapping for cancellation and deadline errors, preserve
error chains and source-data error precedence, and check cancellation during
encoding and before commitment. The deadline relies on cooperative providers;
do not start an abandoned worker goroutine. The response cap bounds emitted bytes
and retained output, not all intermediate service or encoder allocations.

## Compatibility and verification

This amends ADR-0004's initial single-representation discovery boundary. Existing
JSON media types and feature query semantics remain compatible. Public conformance
declarations require matching conformance evidence; implementing HTML or OpenAPI
does not independently declare an OGC conformance class.

Independent review covers generated OpenAPI, JSON/HTML parity, escaped metadata
and properties, all protocol links, negotiation, CORS, GET/HEAD, redirects,
prefixes, cancellation, timeouts, size boundaries and feature-only cache policy.
Real HTTP/GPKG and browser checks supplement unit tests. Final integrated and
external consumer checks precede Group H acceptance.

## Sources

- [OGC API Features Core corrigendum](https://docs.ogc.org/is/17-069r4/17-069r4.html)
- [OpenAPI 3.0.3](https://spec.openapis.org/oas/v3.0.3.html)
- [ADR-0004](ADR-0004-feature-publication-and-runtime.md)
