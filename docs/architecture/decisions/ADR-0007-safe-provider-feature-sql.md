# ADR-0007: Safe provider feature SQL and parity profiles

Status: Accepted after independent architecture and provider feasibility review on 2026-10-01. Backend implementation and runtime acceptance remain separate.

## Context

Feature publication requires MySQL/MariaDB, PostGIS and HANA raw feature queries, including safe custom SQL. The user explicitly selected support now. [ADR-0003](ADR-0003-temporal-metadata-and-resolved-collections.md) initially admits only proven table-backed sources; this amendment adds a constrained custom selection with the same proof obligations. Existing tile SQL contains tile-dependent macros and may not define a stable raw feature source. Token-free SQL, sample uniqueness and configured assurances cannot establish identity lineage or snapshot semantics.

The dimensional and temporal contracts in [ADR-0005](ADR-0005-dimensional-feature-query.md) and [ADR-0006](ADR-0006-exact-datetime-bounds.md) remain mandatory. SQL paging must not precede their exact predicates.

## Decision

### Separate feature selection

Add optional layer configuration `feature_sql`, independent of the existing tile `sql`. For ordinary table layers, its absence uses the proven table profile and all eligible source properties. For custom tile-SQL layers, its absence leaves feature publication unsupported. Presence defines the raw feature domain using the constrained grammar below. It does not rewrite tile SQL, execute dummy tiles or substitute tile macros.

```toml
feature_sql = """
SELECT f.id AS id, f.geom AS geom, f.name, f.observed_at
FROM public.features AS f
WHERE f.enabled = 1
"""
```

This selection has its own documented domain; it need not reproduce zoom-dependent tile filtering. Existing `id_fieldname`, `geometry_fieldname` and temporal field keys refer to projected output names, resolved to physical source columns during registration. Identity, geometry and mapped temporal fields must be projected. Output property names follow the projection. Private bounds and source metadata fields remain suppressed. The feature geometry format and MOS/CRS settings use the existing resolved configuration, validated against the proven source independently of mutable tile-time inference.

### Constrained SQL grammar

One `SELECT` over one physical base table, optionally schema-qualified and aliased. Require explicit direct-column projections with optional aliases; no wildcard or expression projections. Resolve every relation, qualifier, source column and output alias against database catalog metadata using the backend's identifier rules. Unique aliases and unambiguous resolution are required.

The optional `WHERE` is a bounded deterministic AST: direct-column versus scalar-literal comparisons (`=`, `<>`, `!=`, `<`, `<=`, `>`, `>=`), `IS [NOT] NULL`, bounded literal `IN`, Boolean `AND`/`OR`/`NOT` and parentheses. Bind literals after validation against source column types; preserve integer precision and distinguish NULL from false/zero. Unsupported literal/type combinations fail eligibility. No arbitrary expressions, functions, collations or implicit session-dependent conversions are admitted.

Reject views, joins, CTEs, subqueries, set operations, DISTINCT, aggregates, grouping, windows, computed/cast projections, ordering, LIMIT/OFFSET, locking/output clauses, SQL parameters, Tegola macros and statement separators. Reject comments (including MySQL executable comments) and backslash escapes. Strings use single quotes with doubled-quote escaping. MySQL identifiers use backticks; PostgreSQL/HANA identifiers use double quotes. Unquoted identifiers follow the appropriate backend folding rules; MySQL double-quoted syntax is rejected to avoid SQL-mode dependence. Consume the complete input and fail closed on unknown syntax.

Bound input to 64 KiB, 8192 tokens, expression depth 32, 1024 projections and 256 values per IN list. Do not execute the original SQL as an opaque nested string. Render the accepted AST using dialect quoting and bound literal arguments; native geometry serialization is inserted only by the trusted backend executor, outside configured expressions. Shared parser code lives under `provider/internal/featuresql`; its AST exposes detached metadata and no mutable retained caller storage. Dialect SQL builders remain provider-owned.

### Schema-proven identity and metadata

Require one ordinary integral physical identity column with a single-column primary or unique key. A nullable unique key is accepted by explicitly excluding NULL identities, consistent with the existing GPKG contract. Composite-only, partial/prefix/expression or unproven uniqueness is unsupported. Sampling is not proof. Reject computed/generated expression or virtual identity/geometry/temporal columns and source policies whose determinism or visibility cannot be established in this profile. Ordinary stored integer IDENTITY, auto-increment and sequence-default columns remain admissible when the same catalog key proof holds; automatic assignment at insertion is not a computed result identity.

Retain immutable resolved relation identity, source/output lineage, catalog types, identity width/signedness, geometry storage, effective source CRS and provenance, spatial metadata, temporal mapping, MOS settings and private field lists. A source schema/result mismatch during a query is `FeatureDataError`, not an invented feature or capability fallback. Never silently adopt a row SRID or tile-time configuration mutation in the feature profile.

Signed negative identities are corrupt source data; signed nonnegative values retain exact uint64 identity. MySQL unsigned integer identities include the full uint64 range: decode and bind without int64 or float64 narrowing. Out-of-range requested IDs against signed-only storage match no row; they do not wrap into negative values.

Explicit malformed feature configuration fails registration with typed invalid-query information. Unsupported storage/SQL/schema capabilities leave legacy tile registration usable and reject feature publication through the capability interfaces. Wrong configuration types, blank SQL, lexical errors (including comments, forbidden escapes or separators), malformed supported grammar and unknown syntax are registration errors. Recognized SQL constructs outside the admitted grammar produce a bounded unsupported-profile error without execution; this does not claim validation of the rest of arbitrary dialect SQL. The parser is an admission parser, not a general SQL validator. Admission/probe failures must be recorded honestly; an unavailable database cannot become an accepted unknown profile.

Freeze the qualified physical relation at registration, including its schema/database and catalog identity; later queries must not resolve an unqualified name through a changed search path. Bind literals only after checking physical column type and collation compatibility. Suppress private bounds/metadata properties by physical lineage even when custom projections rename them. Column label matching is backend-specific; it must not substitute universal case-insensitive comparison for catalog resolution.

### Query snapshot and paging

Each QueryFeatures invocation uses one read snapshot for all candidate chunks, source integrity checks and optional totals. MySQL/MariaDB require a proven transactional InnoDB base table and read-only repeatable-read transaction. PostGIS uses read-only repeatable-read. HANA uses the single reserved read-write repeatable-read transaction and transaction-owned EXCLUSIVE source lock accepted in [ADR-0008](ADR-0008-hana-feature-source-lock.md). Its executor performs no source DML. No session transaction state may leak to pooled callers; HANA connections are physically discarded. Unsupported snapshot or protection setup fails explicitly.

Acquire source relation protection through the transaction's source read before emitting any callback; verify retained catalog/result identity under that protection. Fail on DDL/schema drift. Do not assume catalog snapshots and physical query plans remain compatible without backend-specific checks. Live adversarial mutation and DDL tests are required before accepting each backend's runtime profile.

Generate ordered keyset chunks with bounded candidate row fetches. Bind IDs, cursors, request values and configured filter literals. SQL may push only predicates proven conservative for the actual storage/CRS, including absent/unclassifiable geometry handling. Nonlinear inverse-envelope transformation is not a safe pruning proof. Use a documented scan fallback when conservative bounds cannot be established.

Decode and fully validate each encountered source row, evaluate exact query-frame spatial and temporal predicates, deduplicate, then apply logical offset/limit. Obtain HasMore from one further exact match. SQL LIMIT bounds candidate chunks only; it never implements final logical paging before exact matching. Report numberMatched only when exactly known. Preserve callback serialization, cancellation/error chains, detached properties/geometry and cleanup; no retry after callbacks begin. Do not call TileFeatures.

### Backend spatial and temporal profiles

| Backend | Initial admitted native/raw profile |
| --- | --- |
| MySQL/MariaDB | Schema-proven native XY export with separate SRID and correct flavor-specific axis handling; strict WKB/WKT XY/XYZ; XY MOS and proven MapplGIS tables. No unproven native XYZ claim. |
| PostGIS | Schema-proven native geometry exported as standard dimensional WKB; strict WKB/WKT/MOS. Geography, M/ZM, curved/surface and arbitrary vertical profiles remain unsupported. |
| HANA | Schema-proven native XY or an explicitly configured native XY/XYZ/mixed profile over permissive dimensional storage under ADR-0008; strict WKB/WKT through canonical height profiles; XY MOS. Each native profile requires independent version-specific lossless export and runtime evidence before admission. |

Raw WKB/WKT default XY, with explicit XYZ/mixed and CRS84h metadata under ADR-0005. Native dimensional admission uses authoritative schema evidence and validates encountered bodies; sampling does not certify all rows. HANA dimension-4 storage is a permissive envelope, not a schema guarantee of XY or XYZ: its configured contract and strict body validation must satisfy ADR-0008. XYZ/mixed projection uses the existing immutable canonical adapter. Preserve original source coordinates; HANA planar-equivalent labels may normalize only when their horizontal CRS equivalence is proven, without silently changing coordinates or relabeling synthetic definitions as canonical.

Use the same declared integer POSIX seconds/milliseconds/microseconds/nanoseconds temporal storage and exact ADR-0006 bound quantization. Native SQL date/time storage is a separately reviewed extension; it is not silently interpreted as integer time. Absent temporal geometry matches. Source temporal corruption is FeatureDataError, and request unsupported capability remains distinct.

### Source CRS metadata without changing tile getters

Add optional `provider.FeatureSourceLayerInfo` with `FeatureSourceSRID() uint64`. It reports the frozen horizontal source-coordinate CRS used by QueryFeatures. Catalog construction uses this accessor when present and otherwise retains the existing LayerInfo.SRID fallback; it copies the value once and applies the same CRS/height-profile validation. Zero or unsupported source CRS rejects publication. The accessor is not a provenance assertion: provider eligibility must separately establish definition/vertical/alias equivalence under ADR-0005.

Keep legacy LayerInfo.SRID and tile token behavior unchanged. HANA may retain its tile planar-equivalent label while returning the separately proven source-coordinate CRS for raw publication. Query output coordinates, row SRID validation, exact predicate transformations and FeatureService serialization must agree with that frozen source CRS. Do not silently remap coordinates or treat an arbitrary synthetic definition as canonical. Tests cover distinct tile/source labels, unknown zero source, copied metadata after mutation, XYZ canonical admission and legacy fallback.

### Execution clarifications (2026-10-02)

Ordinary table `fields` limits the public property subset; an empty subset configuration uses all eligible properties. A custom `feature_sql` explicit projection defines public output labels independently of tile fields. Identity, geometry and private bounds/metadata are suppressed by physical lineage, including aliases. Physical `min_zoom` and `max_zoom` are always private under ASCII case folding. Required temporal columns are read privately when omitted from public selection; explicitly selected temporal columns remain public. Request field selection may only narrow the frozen public set.

A selected public property whose source value is SQL NULL remains present with a nil value and serializes as JSON null. Private and unselected properties remain absent. Zero, false and the empty string retain their values. This raw-feature rule does not change tile property decoding.

Static floating-point comparisons are initially unsupported where exact literal representation has not been proved; finite floating-point properties remain supported. Trusted backend parameter-expression wrappers may cast a bound value using validated immutable catalog type metadata. They retain exactly one existing placeholder and its argument ordinal; no literal or configured SQL text supplies a cast target. Numeric parsing must bound precision, scale and exponent before allocating arbitrary-precision values.

HANA repeatable-read alone does not protect a source from DDL. HANA read-only transaction mode persists beyond commit/rollback, and the current driver does not restore it when returning a session to the pool. A provisional feature executor must reserve its own connection and physically discard it after rows and transaction cleanup, including setup failure. The user subsequently approved the single read-write transaction-owned protection protocol in ADR-0008. Production HANA feature admission stays unsupported until that implementation passes its required source and actual-server gates; architectural approval alone does not establish runtime acceptance. Native dimensional admission and exact decimal character transport also require actual server evidence. Decimal transport uses a canonical string and trusted nested NVARCHAR/DECIMAL casts, with catalog precision and scale validated before binding.

### Verification and ownership

Independent parser admission/lineage/catalog tests precede dependent execution. Tests cover alias/domain preservation, nullable and unsigned identities, malformed/source schema drift, parameterization, raw/Z/empty/null geometry, exact bbox/time before paging, callback ownership/cancellation, snapshot mutations and DDL. Backend contract factories run ordinary and admitted custom selections against independently defined logical fixtures. SQL mocks are unit evidence; real MySQL, MariaDB, PostGIS and HANA parity needs actual services and version-specific native export evidence.

Unavailable services remain an explicit verification limitation. Existing tile tests, CGO variants, race checks and lint retain their separate verification roles. Extract only demonstrably identical provider semantics into shared helpers. Verify external consumers without filesystem replacements before making portability claims.

## Sources

- [MySQL InnoDB repeatable-read behavior](https://dev.mysql.com/doc/refman/8.4/en/innodb-transaction-isolation-levels.html)
- [PostgreSQL transaction isolation](https://www.postgresql.org/docs/current/transaction-iso.html)
- [SAP HANA SET TRANSACTION and transaction-level snapshots](https://help.sap.com/docs/SAP_HANA_PLATFORM/4fe29514fd584807ac9f2a04f6754767/20fdf9cb75191014b85aaa9dec841291.html)
- [PostGIS ST_AsBinary](https://postgis.net/docs/ST_AsBinary.html)
- [MySQL geometry format conversion](https://dev.mysql.com/doc/refman/8.4/en/gis-format-conversion-functions.html)

## Consequences

Safe custom selection is useful but deliberately narrower than arbitrary configured SQL. Tile behavior and macros retain their existing contract. Source eligibility is proved from structure and catalog constraints, not inferred from sampled success. Provider performance claims distinguish conservative scan fallbacks from verified index paths. Public feature query interfaces remain unchanged; additive configuration and capability support are reviewed before use.
