# ADR-0003: Immutable temporal metadata and resolved collection inputs

Status: Accepted on 2026-10-01. This decision does not certify runtime behavior.

## Context

[ADR-0001](ADR-0001-feature-service-boundaries.md) separates providers, FeatureService and HTTP. The accepted query contract carries neutral temporal bounds. Providers need immutable source mappings to evaluate them. FeatureService must exist before configuration constructs the publication catalog.

## Decision

### Provider metadata

Add `provider/temporal.go`, preserving `LayerInfo` and `Tiler`:

```go
type TemporalMapping struct {
    InstantField string
    StartField   string
    EndField     string
}

type TemporalLayerInfo interface {
    TemporalMapping() (TemporalMapping, error)
}

type FeatureQueryLayerInfo interface {
    FeatureQuerySupported() error
}
```

`TemporalMapping.Validate()` accepts zero mapping (declared absence), one instant field, or two interval field names. Reject mixed/partial mapping, blank names and identical interval fields with `InvalidFeatureQueryError`. Source field existence and storage interpretation remain constructor validation. No storage enum belongs to the neutral mapping. Providers return value snapshots; no setters. A layer lacking the optional capability is unknown, and cannot be published by FeatureService until it declares support or absence. Existing tile-only use remains compatible.

GPKG layer configuration uses `temporal_field` OR `temporal_start_field` + `temporal_end_field`, with `temporal_storage` equal to `unix_seconds`, `unix_milliseconds`, `unix_microseconds` or `unix_nanoseconds`. Storage is required with mapping and rejected without it. No keys declares absence. Resolve against actual source/result columns independently of projected tags. The initial feature profile supports table-backed layers with a proven unique integer ID column. Custom-SQL layers remain fully available to tiles but their FeatureQuerySupported and QueryFeatures return an error compatible with ErrUnsupported. This includes token-free SELECT statements: a later reviewed compiler/profile must prove alias uniqueness, source identity, deterministic ordering and safe token handling before admitting them. Never substitute tile tokens or fake tiles. Custom SQL without temporal keys keeps existing tile registration. Explicit otherwise valid temporal mapping on custom SQL fails registration with ErrUnsupported after shape/storage validation, because this initial profile cannot establish source temporal metadata without a reviewed compiler. Do not invent a new structural token processor. Schema declarations assist validation; every encountered non-NULL temporal value must be SQLite INTEGER. Do not silently coerce TEXT/REAL or infer epoch units.

Both NULL interval endpoints mean absent temporal geometry and match a valid temporal constraint. One NULL means an open endpoint. Non-NULL source endpoints must be ordered; invalid rows fail with contextual errors without SQL or credentials. Missing fields/mixed mappings fail registration. Source storage unsupported by this initial profile returns an error compatible with `ErrUnsupported`. Explicit temporal mapping errors fail registration; feature-profile limitations alone do not break existing tile registration. GPKG stores a private eligibility error and exposes it through FeatureQueryLayerInfo. Publication requires this optional capability and checks it before serving; absence is unknown, not supported. Other providers add the capability during parity tasks. SQL temporal predicates must retain invalid-type/reversed-interval candidates for row validation rather than silently discard them through coercion. Requested IDs above MaxInt64 are impossible SQLite matches, not wrapped parameters; mixed requests retain representable IDs. Invalid negative/noninteger source IDs fail. Unique IDs proven from table schema permit stable keyset candidate paging within a read transaction snapshot.

### Inclusive temporal comparisons

Convert query bounds to integer epoch units with checked seconds/nanoseconds arithmetic or exact integer arithmetic. Instants satisfy `value >= ceil(queryStart)` and `value <= floor(queryEnd)`. Intervals satisfy `start <= floor(queryEnd)` and `end >= ceil(queryStart)` with NULL/open endpoint clauses. A sub-unit query range can contain no stored instant while still intersecting a stored interval. Negative epochs require mathematical floor/ceil. Avoid unchecked `UnixNano`, floating SQLite datetime conversions and RFC3339 lexical ordering. Bounds outside int64 storage range simplify to always/never predicates instead of overflowing. All SQL values use bound parameters; field identifiers come from resolved metadata.

### Resolved collection ownership

The read service owns `ogc/features/catalog.go` with:

```go
type CollectionSource struct {
    ID      string
    Layer   provider.LayerInfo
    Querier provider.FeatureQuerier
}
```

`NewService([]CollectionSource) (*Service, error)` resolves and snapshots ID, layer name, source SRID and temporal mapping, validating unique nonblank IDs, available queriers, supported transforms, feature-query eligibility and optional temporal capability. Do not retain mutable layer metadata or caller slices. Private resolved entries form the initial catalog. Startup assembly extends publication metadata and constructs these inputs from registered providers; it does not introduce a second catalog or mutate providers. Tests can construct sources without map definitions or TOML.

`QueryCollection` accepts the neutral provider query and synchronously emits independent GeoJSON features; it returns query result/count metadata. `QueryFeature` issues an ID-restricted query and reports a typed missing-feature error. The original Bounds and BoundsSRID reach the provider unchanged. Providers own conservative candidate transformation and exact matching in the original query CRS before paging/counting; the service owns response transformation only. Reject unknown/unsupported transforms with ErrUnsupported. Source metadata is finalized before construction and returned feature SRID must match the frozen source SRID. GeometryCollection structure, identifiers and properties survive. Deep-copy mutable geometry/properties before transformation; reject unsupported/nonfinite geometry instead of emitting invalid JSON. Null geometry remains null. CRS84 output uses longitude/latitude degrees per [RFC 7946](https://www.rfc-editor.org/rfc/rfc7946).

Freeze these service methods: QueryCollection(ctx context.Context, collectionID string, query provider.FeatureQuery, fn func(Feature) error) (provider.FeatureQueryResult, error); QueryFeature(ctx context.Context, collectionID string, featureID uint64) (Feature, error); QueryCollectionPage(ctx context.Context, collectionID string, query provider.FeatureQuery) (FeatureCollection, error); WriteGeoJSON(ctx context.Context, w io.Writer, page FeatureCollection) error. Missing collection and missing feature use distinct typed errors. QueryFeature uses limit 1/offset 0; callbacks preserve caller/context errors. The page helper rejects overdelivery and callback/count disagreement and returns no successful partial page.

Provide explicit bounded collection encoding helpers: buffer at most the requested page, preserving pre-response errors for HTTP integration. No unbounded collection materialization. Large page encoding must check cancellation/writer errors; HTTP publication limits bound request pages. HTTP routing and configuration remain separate from response encoding.

### GPKG spatial and paging correctness

Reuse strict shared row decoding. RTree/raw bounds produce conservative candidates; exact spatial union matching and distinct-ID selection precede logical offset/limit. Read stable ordered bounded candidate chunks and continue past false positives. Source-CRS indexed queries must use available RTree/raw bounds. For a different bounds CRS, use inverse envelopes only when conservativeness is established; corner sampling or heuristic densification alone is insufficient. The initial correctness fallback reads ordered bounded source chunks and transforms each geometry into the requested CRS for exact matching. This fallback can scan the source, is reported as a cross-CRS performance limitation, and cannot justify an indexed-pushdown claim. Same-CRS indexed paths remain mandatory in provider pushdown smoke tests. Absence-aware branches may inspect source headers. Binary aggregate containers (including native GPKG polygon/multi/collection and raw WKB/MOS families) whose absence cannot be safely classified by SQL must be conservatively included, even outside coarse bounds; this can decode many aggregate rows. Do not register a per-query SQLite decoding function to hide that scan inside SQL. Report the format-specific fallback and restrict pruning claims to measured eligible profiles. Exact membership still precedes logical paging in every profile. One extra exact match determines HasMore. Include null/decoded-empty geometry candidates even with missing index entries or misleading bounds. A format-specific conservative predicate or missing-index branch must prove absence coverage; `geom IS NULL` alone is insufficient. Exact counts remain unknown unless established, with exhausted empty queries exact zero. Never route through TileFeatures or fake tiles.

## Verification

Test precision around epoch and storage boundaries, int64 extremes, NULL/open intervals, invalid mappings and source values. Test exact spatial paging after coarse false positives and absent geometry with misleading bounds. Run the real GPKG contract harness, CRS equivalence matrix and existing tile regressions before provider acceptance.

## Consequences

Existing provider interfaces remain source compatible. Feature publication requires explicit temporal metadata; each provider supplies its own admitted mapping. Integer epoch profiles initially limit GPKG temporal storage, with additional profiles requiring separately reviewed normalization rules. Configuration owns publication choices and providers own source interpretation.
