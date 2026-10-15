[Documentation index](../../README.md) · [Feature service](../feature-service.md)

# Architecture decisions

The read and write work originally used overlapping numeric identifiers. Their descriptive filenames are stable; use the full decision title or the qualified series name below when linking. Decisions describe software contracts. Historical acceptance dates are not current deployment or certification claims.

| Decision | Scope |
|---|---|
| [ADR-0001: Optional feature queries and one OGC service](ADR-0001-feature-service-boundaries.md) | Feature queries and transport |
| [ADR-0002: Absent spatial geometry in feature queries](ADR-0002-absent-feature-geometry.md) | Feature queries and transport |
| [ADR-0003: Immutable temporal metadata and resolved collection inputs](ADR-0003-temporal-metadata-and-resolved-collections.md) | Feature queries and transport |
| [ADR-0004: Explicit feature publication and immutable HTTP runtime](ADR-0004-feature-publication-and-runtime.md) | Feature queries and transport |
| [ADR-0005: Dimensional raw feature queries](ADR-0005-dimensional-feature-query.md) | Feature queries and transport |
| [ADR-0006: Exact RFC 3339 query boundaries](ADR-0006-exact-datetime-bounds.md) | Feature queries and transport |
| [ADR-0007: Safe provider feature SQL and parity profiles](ADR-0007-safe-provider-feature-sql.md) | Feature queries and transport |
| [ADR-0008: HANA feature reads with a transaction-owned source lock](ADR-0008-hana-feature-source-lock.md) | Feature queries and transport |
| [Read ADR-0009: Typed feature filtering and Queryables](ADR-0009-typed-feature-filtering.md) | Feature queries and transport |
| [Write ADR-0009: Shared WFS and OGC API Features mutation architecture](ADR-0009-wfs-part4-program-baseline.md) | Shared mutation architecture |
| [Read ADR-0010: Public CRS identity and immutable Part 2 projection](ADR-0010-public-crs.md) | Feature queries and transport |
| [Write ADR-0010: One feature core, two transport adapters (WFS and OGC API Features)](ADR-0010-shared-feature-core.md) | Shared mutation architecture |
| [Read ADR-0011: Feature representations, API definition and protocol limits](ADR-0011-feature-representations-and-protocol.md) | Feature queries and transport |
| [Write ADR-0011: Schema, identity and revision mapping](ADR-0011-schema-identity-revision.md) | Shared mutation architecture |
| [Read ADR-0012: Feature conformance admission and panic containment](ADR-0012-feature-conformance-and-panic-containment.md) | Feature queries and transport |
| [Write ADR-0012: Transaction domains and unknown commit outcome](ADR-0012-transaction-domains.md) | Shared mutation architecture |
| [Read ADR-0013: Feature observability and measured performance budgets](ADR-0013-feature-observability.md) | Feature queries and transport |
| [Write ADR-0013: Write admission and source shapes](ADR-0013-write-admission.md) | Shared mutation architecture |
| [Write ADR-0014: GeoPackage as the initial reference writer](ADR-0014-gpkg-reference-writer.md) | Shared mutation architecture |
| [Read ADR-0014: Lambda transport over the existing router](ADR-0014-lambda-router-adapter.md) | Feature queries and transport |
