[Feature-service architecture](../feature-service.md)

# ADR-0002: Absent spatial geometry in feature queries

Status: accepted after independent normative/domain review on 2026-10-01.
Date: 2026-10-01
Sources: [OGC Part 1 1.0.1, section 7.15.3, requirement 24C](https://docs.ogc.org/is/17-069r4/17-069r4.html), [current GPKG row path](../../../provider/gpkg/gpkg.go), [shared query contract](../../../provider/query.go).

## Context

The legacy tile pipeline omits null/empty geometry because it cannot encode those features as tile geometry. The Feature API must preserve features without spatial geometry and apply the selected normative bbox behavior. The shared suite must not encode tile exclusion as feature-query parity.

## Constraints / normative requirements

Part 1 requirement /req/core/fc-bbox-response C requires bbox to also match features without spatial geometry. IDs, temporal predicates and other independent dimensions still constrain those features. Invalid geometry is distinct from absent geometry.

## Considered options

Rejected skipping absent geometry in the common query suite because that would prevent compliant HTTP results. Rejected returning malformed geometry as absence because that hides corrupt data. Accepted retaining representable null/empty features in the raw-feature path, with explicit tile policy outside shared decoding.

## Decision

A row with a valid feature ID and no spatial geometry remains a queryable feature. A decoded empty geometry represented as nil is normalized to absent spatial geometry. Such a feature matches any valid horizontal bounds constraint; ID, temporal, limit and paging semantics still apply. GeoJSON represents it with geometry null. Deduplicate and apply paging/count after this semantic selection.

Malformed explicitly selected geometry formats return a decode error. Metadata rows and unrepresentable null-ID rows are not invented features. The GPKG shared decoder reports row representability separately from Geometry nil. TileFeatures continues rejecting nil geometry and keeps its existing exact filter. A legacy tolerance for auto-detected malformed MOS must remain identifiable as a tile policy; it does not authorize silent loss in the new raw-feature contract.

## Consequences

SQL spatial pushdown must include null/no-spatial-geometry rows, including indexed query plans whose spatial index excludes them. The provider contract suite includes null/empty geometry without bbox and with a nonintersecting bbox, plus ID/time/paging combinations. Tests distinguish null geometry, empty geometry normalized to null, malformed geometry, null IDs and metadata rows.

## Compatibility and migration

Existing MVT behavior is unchanged. The new optional raw-feature capability has its own normative selection policy; no fake TileFeatures fallback is introduced. Collections with temporal geometry continue using the approved temporal intersection/absence semantics.

## Verification required

Shared decoder and provider contract tests must distinguish null, empty, malformed geometry and null IDs. HTTP tests verify `geometry: null` and bbox behavior. The same scenarios apply to each admitted provider. Temporal mapping and storage precision are separate decisions.
