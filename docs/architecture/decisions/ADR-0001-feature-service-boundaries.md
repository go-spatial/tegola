[Feature-service architecture](../feature-service.md)

# ADR-0001: Optional feature queries and one OGC service

Status: accepted on 2026-10-01; implemented and extended by the shared mutation core.

## Context and decision

Tile queries carry a tile and tile parameters. Feature publication requires independent bounded queries, stable identity and standards-based coordinate semantics. Adapting the legacy Jivan `EmptyTile` approach would couple feature access to clipping and full materialization.

Introduce optional `provider.FeatureQuerier` alongside `Tiler`. Keep neutral query/filter contracts in `provider/`, read service and representations in `ogc/features/`, and HTTP handlers in `server/`. Default response coordinates are CRS84. Unknown result count remains distinct from exact zero. Provider paging is independent of HTTP links. Explicit publication excludes MVT-only sources without a raw-feature capability. Lambda wraps the same router.

The later shared `feature/` core owns schema, identity, policy and mutations for both WFS and REST. This extends the original read decision without introducing separate protocol-specific mutation engines.

## Consequences

Providers can support both tile and raw queries. Decode seams are shared only when their semantics agree. Collection publication is independent of map styling. Services preserve cancellation and transform detached response geometry. HTTP strings and SQL fragments do not belong in provider contracts.

## Core datetime

Core datetime is part of the selected OGC Part 1 profile, including temporal intersection and absent temporal geometry semantics. Queryables/CQL2 compose with it. Authority: OGC Part 1 section 7.15.4 requirements 25/26. Mapping, storage precision and exact bounds are defined by the temporal decisions.

## Compatibility and verification

Existing `Tiler` callers retain their interfaces. Jivan `time` migrates to `datetime`; arbitrary property filters migrate to the documented Queryables/CQL2 subset. Provider contracts, HTTP behavior, CRS, filtering, MVT compatibility and conformance evidence must be checked against the candidate revision. Architectural acceptance alone is not a runtime or certification result.
