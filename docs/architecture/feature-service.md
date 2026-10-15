[Documentation index](../README.md)

# Feature service architecture

The current source implements raw feature queries, OGC API Features reads and mutations, and WFS reads and transactions alongside MVT serving. Publication and writes are independently opt-in. This architecture describes source behavior; it is not an OGC certification or a deployment acceptance record.

## Package boundaries

| Package | Responsibility |
|---|---|
| `provider/` | Optional `FeatureQuerier` and mutation contracts, detached metadata, typed query/filter values and errors |
| `provider/<backend>/` | Source admission, SQL compilation, protected reads, decoding and native transactions |
| `feature/` | Shared collection catalog, schema, identity, policy and mutation coordinator |
| `ogc/features/` | Read service, public CRS transforms and feature representations |
| `ogc/wfs/` | WFS/GML/filter parsing and protocol serialization |
| `ogc/cql2/` | Selected CQL2 text grammar mapped to provider-neutral expressions |
| `server/` | HTTP input validation, routing, representation negotiation, bounded responses and cache middleware |
| `cmd/` and `config/` | Startup validation and assembly of immutable runtime instances |

Feature queries never fabricate a tile or invoke `TileFeatures` as a fallback. MVT-only providers have no implicit raw-feature capability. Explicit collections map public IDs to provider layers independently of map styling.

## Query and mutation boundaries

Providers own dialect-specific SQL, bound parameters, row scanning and transactional guarantees. Services and handlers do not execute SQL. Reuse decoding only where semantics match: raw queries preserve absent geometry, while tiles omit geometry they cannot encode. Raw feature responses do not apply tile clipping or simplification.

Default feature coordinates use CRS84 longitude/latitude or an admitted height-preserving CRS. Public CRS identifiers describe effective coordinate semantics; internal synthetic SRIDs must not become fabricated EPSG codes. Transform detached response values without changing provider-owned geometry.

Paging is bounded and ordered by stable identity. Unknown totals remain distinct from exact zero. Provider selection, including exact spatial and temporal matching, precedes paging. Separate requests across a changing dataset do not promise one snapshot.

Both mutation adapters resolve the same schema, identity, authorization policy and transaction domain through `feature/`. Read eligibility does not grant write eligibility. Native writers enforce admitted source shapes, revision checks and atomic outcomes; protocol adapters map their neutral results.

## Tile cache after mutations

Writable routers retain server-side tile caching with a router-specific startup namespace and generation snapshots. Successful or uncertain commits invalidate the generation; confirmed rollback does not. Requests already rendering retain their old namespace, including background regeneration and deduplication keys. `X-Tegola-Editor-Active: true` or `1` bypasses ordinary tile cache use for that request. Writable tile responses remain HTTP `no-store`. This process-local mechanism does not observe external SQL changes or coordinate independent servers; see the [API reference](../api.md).

## Dependency direction

`cmd` assembles configuration, providers and services. `server` depends on protocol adapters and feature services; the shared core depends on provider contracts and geometry/CRS foundations. Provider contracts import neither HTTP handlers nor protocol packages. Lambda adapts the assembled router without duplicating feature semantics.

## Decisions and verification

The [decision index](decisions/README.md) distinguishes historical read and write decisions. Current provider guarantees and restrictions are in the [provider contract](../provider-contract.md), [WFS scope](../wfs-scope-limitations.md) and [release guide](../release/feature-api.md). Run the documented suites against the exact candidate revision; source implementation, local database evidence, official validator output and deployed acceptance are separate results.

## Shared decoder ownership

The optional raw-query contract is declared in [provider/query.go](../../provider/query.go) and consumed by the implemented FeatureService and HTTP API. [GPKG row decoding](../../provider/gpkg/row_decode.go) constructs features for both tile and raw-query callers. Scanning, SQL, exact tile filtering and callback delivery stay outside that helper. Its strict default returns malformed-geometry errors; the tile caller explicitly selects its existing auto-MOS tolerance and sticky null/empty exclusion.

PostGIS raw queries use `consumeFeatureChunk` and
`featureProfile.decodeFeatureGeometry`; tile decoding retains `decipherFields`
and `decodeGeometryValue`. MySQL raw queries use `featureProfile.decodeFeature`,
while tile queries retain their legacy `decodeGeometry` flow. HANA raw queries
use `featureScanRow` and `decodeFeature`; native geometry is exported as WKB
before strict decoding. Its tile path retains `readRowValues` and
`decodeGeometryValue`. The shared geometry codec owns strict dimensional raw
format decoding, exact predicates and pure transforms. Raw-query policy and
legacy tile interpretation remain distinct; provider-specific scanning and
native envelopes stay with their providers.

Raw null/normalized-empty geometry follows [ADR-0002](decisions/ADR-0002-absent-feature-geometry.md), including bbox matching. The [contract harness](../../provider/internal/querytest/querytest.go) supplies explicit fixture expectations; its reference adapter demonstrates harness behavior, without proving real provider or database parity.
## Temporal metadata and collection construction

[ADR-0003](decisions/ADR-0003-temporal-metadata-and-resolved-collections.md) defines immutable source temporal mappings and resolved collection inputs. Feature publication requires explicit provider eligibility and temporal metadata. Original query bounds remain with the provider for exact matching before pagination; FeatureService transforms response geometry to the default CRS84/CRS84h or an explicitly selected admitted output CRS.

The first GPKG feature profile admits table-backed layers with unique integer IDs. Custom SQL remains available to tiles and is unsupported for feature queries in this profile. Cross-CRS selection may need a bounded-memory source scan when a conservative indexed envelope cannot be established. These are capability and performance limits to report during acceptance, rather than completed runtime claims.

## Typed filtering integration

[ADR-0009](decisions/ADR-0009-typed-feature-filtering.md) defines the additive
Queryables and neutral filter boundary. `provider/query_filter.go` owns detached
catalogs, validated expressions and exact scalar literals. `ogc/cql2` parses
the selected text grammar; providers resolve properties against their own frozen
public-to-physical mappings and compile bound predicates within the protected
source snapshot. Optional catalog failure must preserve unfiltered Core and tile
capabilities. Public epoch aliases remain integers; unsupported native profiles
are omitted, rather than guessed from rows.

The [filtering guide](../filtering.md) describes the application profile, request
limits and three-valued comparison semantics. Implemented application behavior
does not establish an official conformance declaration.

## Representations and protocol boundary

[ADR-0011](decisions/ADR-0011-feature-representations-and-protocol.md) defines
capability-driven OpenAPI, JSON/HTML representations and feature-only HTTP limits.
The API generator reads immutable service metadata without provider I/O.
HTTP representation selection, format links, CORS and response commitment stay
in `server/`; embedded HTML templates receive complete response values and
explicit protocol links. Provider queries and geometry transformations remain
outside presentation code. Configuration and executable wiring carry the
publication deadline and encoded-response cap through CLI and Lambda.

## Conformance, reliability and observability

[ADR-0012](decisions/ADR-0012-feature-conformance-and-panic-containment.md)
defines immutable class admission across all published collections and the
feature-only response transaction. Generic panic errors discard buffered output;
tile and viewer routing retain their existing serving paths. The normal CLI and
committed fixtures drive the [conformance runner](../testing/ogc-conformance.md).

[ADR-0013](decisions/ADR-0013-feature-observability.md) defines optional,
instance-bound request and provider-query observation with fixed labels. Query
latency includes the provider/callback pipeline. Provider execution metadata is
snapshotted once; unavailable metadata remains unknown without disabling Core.
Performance budgets require measured, independently accepted references and do
not imply a remote database or production HTTP latency guarantee.
