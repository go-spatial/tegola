# Read ADR-0012: Feature conformance admission and panic containment

Status: Accepted after independent architecture and security design review on 2026-10-02. Implementation independently reviewed; checkpoint acceptance requires the verification evidence below.

## Conformance admission

The feature service exposes a detached, deterministic conformance declaration.
Admission requires reviewed implementation evidence for the complete class and
eligibility across every published collection. Use an immutable implementation
registry and the intersection of frozen collection capabilities. Do not take
class URIs or prior report paths from runtime configuration or requests. A
capability in one collection cannot authorize a service-wide extension claim.

Core datetime applies to every collection, including collections without temporal
geometry. Core, GeoJSON, HTML and OpenAPI classes require their corresponding
official checks and independent supplements. Part 2 is admitted only when every
published source has the accepted coordinate-reference-system capability.
Empty or unsupported catalogs must not manufacture extension eligibility.

Part 3 and CQL2 endpoints remain available according to their existing collection
capabilities. Their global class declarations are withheld until an appropriate
approved validator is available. Normative ATS supplements provide explicitly
identified endpoint evidence, not a substitute official executable suite or a
certification claim.

The reproducible product runner uses committed fixtures and the normal CLI. It
records the actual declaration, exact revision and dirty source inventory,
executable/configuration/dataset hashes, pinned suite/image identity, raw reports,
classified skips and source drift. Final verification does not insert a wrapper
that changes the declaration. Diagnostic wrappers are explicitly separate and
cannot alone establish final acceptance. Failed required checks block admission;
skips are never credited as passes.

An independently established limitation in the pinned executable suite can
require separately reported normative ATS evidence. Such admission is restricted
to the exact affected methods, suite version and owned fixture, with internally
executed literal geometry, axis, height, paging and response-header oracles.
The suite's absent optional-extent default mapping and unsupported EPSG:4979
transformer are recorded as tool limitations. Preserve original missing methods,
skips and failures; supplements never increase official PASS counts or establish
a whole-suite PASS. An ordinary product failure or any uncovered required method
still blocks admission. The runner guide records this bounded policy.

Generated data and reports belong in a fresh caller-selected output directory.
Runner preflight is limited to an explicit local origin, rejects redirects and
credentials, bounds input sizes and rejects XML entity declarations. The runner
does not depend on local AI context or private test-server credentials.

## Feature-only panic containment

The feature-owned router branch executes synchronously with a detached header
map and bounded response capture. Commit headers and body only after successful
handler completion. The capture is bounded by the existing response cap; it is
not a promise to bound every intermediate allocation in providers or encoders.
Unrelated tile and viewer requests retain their existing serving path.

A recovered panic produces a fixed generic JSON 500, without partial successful
output. Never format or log the recovered value. Preserve configured CORS and
appropriate custom headers; clear Content-Crs, validators, content encoding,
redirect and trailer headers on the error response. Apply no-store and accurate
Content-Length; HEAD has the same selected error headers and no body.

Re-panic the exact standard-library `http.ErrAbortHandler` sentinel. A panic is
classified as 500 independently of context cancellation; ordinary returned
cancellation or deadline errors retain the existing 408 mapping. Recovery stays
in the serving goroutine, with no abandoned query worker. Network commitment
occurs outside the recover boundary; this guard does not promise recovery from
socket failures after commitment.

## Verification

Independent tests cover panic before output, after WriteHeader and after buffered
body output; JSON/HTML and GET/HEAD; header cleanup, fixed error data and healthy
subsequent requests; response-cap overflow and sentinel propagation. Actual
database-driver cancellation and same-pool recovery are separate required
reliability evidence and cannot be inferred from these HTTP tests.

## References

- [Part 1 Core](https://docs.ogc.org/is/17-069r4/17-069r4.html)
- [Part 2 CRS](https://docs.ogc.org/is/18-058r1/18-058r1.html)
- [Part 3 Filtering](https://docs.ogc.org/is/19-079r2/19-079r2.html)
- [CQL2](https://docs.ogc.org/is/21-065r2/21-065r2.html)
- [Feature protocol policy](ADR-0011-feature-representations-and-protocol.md)
