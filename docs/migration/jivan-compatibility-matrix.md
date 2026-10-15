[← Migration guide](jivan-to-tegola.md) · [Documentation index](../README.md)

# Jivan compatibility and disposition

Baseline: Jivan `c9fba2bb5188ba43c27539740195085e92f0ad7d`. This table maps
source-observed legacy behavior to the implemented Tegola application profile.
Verification cells are operator acceptance actions, not claims that a particular
deployment or release has passed them. `B` below means the configured feature
base path plus external URI prefix (default `/features`).

| Category | Old endpoint/behavior | New endpoint/behavior | Intentional difference | Configuration/client migration | Verification |
|---|---|---|---|---|---|
| 1. Landing | GET/HEAD `/` | GET/HEAD `B` | Genuine navigation and explicit representation links; feature and tile roots coexist. | Set feature basepath and external prefix; follow links. | JSON/HTML links have correct origin/prefix; HEAD empty. |
| 2. API definition | GET/HEAD `/api`, WFS3-era artifact | GET/HEAD `B/api` | OpenAPI generated from immutable publication capabilities; HTML service documentation. | Replace hard-coded legacy definition URL. | Validate actual document, paths, parameters, media and HEAD headers. |
| 3. Conformance | GET/HEAD `/conformance`, legacy classes | GET/HEAD `B/conformance` | Fixed admitted Core/GeoJSON/HTML/OAS30; CRS only all-collection intersection. Part3/CQL2 global declarations withheld. | Inspect actual declaration; do not configure arbitrary class URIs. | Pinned official suite plus classified limitations/supplements; no certification inference. |
| 4. Collections | GET/HEAD `/collections` | GET/HEAD `B/collections` | Only explicit source mappings; no automatic map-layer exposure. | Declare `[[features.collections]]`. | Expected IDs only; published metadata and nested links correct. |
| 5. Collection | GET/HEAD `/collections/:name` | GET/HEAD `B/collections/{id}` | Literal public ID; optional Queryables/CRS metadata reflects frozen capability. | Map public name to `provider.layer`; check identifier grammar. | Known/unknown collection, available links and CRS list; 404 for unknown. |
| 6. Items | GET/HEAD `/collections/:name/items`, EmptyTile/materialized adapter | GET/HEAD `B/collections/{id}/items` | Raw FeatureQuerier, exact predicates before paging; bounded page; optional exact count. | Select eligible raw source, limits and timeout. | Static IDs/properties/geometry, nullable count, cancellation and errors. |
| 7. Item | Same handler at `.../items/:feature_id` | GET/HEAD `B/collections/{id}/items/{uint64}` | Strict unsigned ID; source selection, not full collection materialization; only `crs` and `f` options. | Preserve proven source IDs; update URL/parameter whitelist. | Known/missing/invalid ID; geometry/CRS and HEAD parity. |
| 8. Bbox | Four comma-separated floats | `bbox` four/six Core coordinates; optional advertised `bbox-crs` | Finite/range checks, antimeridian, exact query-frame geometry/height semantics. | Reconcile axes and dimensional metadata; do not use tile envelope semantics. | Boundary, holes, correlated Z, null geometry, axis swap and invalid inputs. |
| 9. Time | `time`, inferred `timestamp`/`start_time`/`stop_time` tags | `datetime`, explicit instant or interval source mapping | RFC3339/open bounds/exact precision and leap policy; unmapped match-all. | Rename parameter and deliberately convert/map storage to admitted integer unit. | Instant/interval/open/reversed, NULL ends, precise ticks and unmapped valid/invalid requests. |
| 10. Property filters | Arbitrary nonreserved query keys | `.../queryables`; `filter` with `cql2-text` | Closed typed public catalog; six comparisons/NULL/Boolean subset, bound SQL. | Replace `?name=Main` with encoded typed expression; inspect eligible aliases. | Unknown/private/type mismatch 400, optional capability 501, NULL three-valued logic before paging. |
| 11. Paging | `page`, `limit`; reconstructed legacy links | Returned `next`/`prev`; bounded `limit`, vendor `offset` | `page` invalid; links retain validated predicates/format; counts can be unknown. | Follow returned links; avoid carrying page arithmetic. | No duplicate/omitted IDs on stable data; filter/CRS/format preserved; count not fabricated. |
| 12. JSON/HTML | MIME-valued `f`; incomplete Accept parsing | `f=json|html`, validated Accept; embedded escaped HTML | Canonical per-resource media; 406 unsupported; explicit format override. | Replace MIME-valued `f`; remove CDN/runtime expectations. | All successful resources in both formats; genuine links, inert property markup, no remote assets. |
| 13. HEAD/ETag | Registered HEAD; FNV collection/name/ID validator | HEAD representation parity; no-store, no validators/304 | No freshness inference from IDs; bodyless HEAD includes cap/errors. | Remove feature conditional caching; preserve separate tile policy. | GET/HEAD status/header parity, ETag absent, conditional request not304, CORS/OPTIONS. |
| 14. Provider config | Legacy provider adapter and implicit collection domain | Explicit provider.layer publication and documented raw profiles | Unique integer identity, strict dimensions/CRS, private fields, selected SQL NULL; no MVT-only fallback. | Backend guide admission; constrained `feature_sql` where supported; explicit temporal mappings. | Ordinary/custom source selection, malformed data, NULL/public fields, source protection and tile regression. |
| 15. Deployment/Lambda | Jivan Lambda `StartServer` uses legacy routes | Normal Tegola CLI/shared router; Lambda adapter parity separately verified | No duplicate query/protocol stack; cooperative timeout/cap, safe panic containment. | Migrate executable/config, origin/stage prefix and access controls; retain rollback. | Actual event-format landing/collections/items/HEAD/errors and prefix links; separate deployed smoke and approval. |

## Scope and retirement boundary

The [pinned inventory](jivan-feature-matrix.md) covers all 14 external route
registrations and the 27 requirement/disposition rows. Its unregistered
`filteredFeatures` and temporary-collection creation have no migrated public
route; promoting them is separate work. No legacy behavior permits bypassing
source eligibility, using a fabricated Tile or silently dropping dimensional
ordinates.

The current conformance policy separates official results from exact scoped
tool limitations and literal ATS supplements. Missing checks, skips and failures
remain visible. Part3/CQL2 implementation is not a global conformance claim.
Consult the [conformance guide](../testing/ogc-conformance.md) and current
[CRS](../crs.md)/[filter](../filtering.md) profiles before accepting a client.

This matrix describes full replacement of the legacy feature-serving role with
explicit supported capabilities and dispositions. It does not record release
or deployed parity as complete, or authorize changes to the unmanaged upstream
Jivan README/repository. Keep the original deployment recoverable until representative migration
smokes, Lambda disposition when relevant, provenance and release approval are
recorded for the exact candidate.

## Pinned sources

- [Routes](https://github.com/go-spatial/jivan/blob/c9fba2bb5188ba43c27539740195085e92f0ad7d/server/routes.go)
- [Handlers](https://github.com/go-spatial/jivan/blob/c9fba2bb5188ba43c27539740195085e92f0ad7d/server/handlers.go)
- [Provider adapter](https://github.com/go-spatial/jivan/blob/c9fba2bb5188ba43c27539740195085e92f0ad7d/data_provider/provider.go)
- [Feature output/validator](https://github.com/go-spatial/jivan/blob/c9fba2bb5188ba43c27539740195085e92f0ad7d/wfs3/features.go)
- [Lambda startup](https://github.com/go-spatial/jivan/blob/c9fba2bb5188ba43c27539740195085e92f0ad7d/server/server_awslambda.go)

## See Also

- [Operator migration guide](jivan-to-tegola.md)
- [HTTP API](../api.md)
- [Provider contract](../provider-contract.md)
