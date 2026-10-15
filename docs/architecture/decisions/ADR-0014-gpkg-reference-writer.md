# Write ADR-0014: GeoPackage as the initial reference writer

Status: accepted on 2026-10-04. The initial sequencing rationale is historical; current source also includes PostGIS and MySQL/MariaDB writers.

## Context and decision

File-backed SQLite/GeoPackage provided a reproducible native database for developing the shared mutation contract before remote-provider acceptance. It was selected as the initial reference for Insert, Replace, Update, Delete, revision checks and atomic transaction/audit behavior.

The shared `provider/mutation.go` and `feature/` contracts remain backend-independent. Each additional provider must prove its source admission and transaction guarantees independently; a successful GeoPackage test does not admit another backend.

## Consequences and verification

GeoPackage contract tests use real database files. PostGIS and MySQL/MariaDB implementations now use the same contracts with their own native admission rules and fixtures. HANA has no admitted write implementation. Actual database versions, geometry storage and transactional engines determine supported profiles; see [write limitations](../../wfs-scope-limitations.md) and the [provider contract](../../provider-contract.md).
