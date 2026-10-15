# Write ADR-0013: Write admission and source shapes

Status: accepted on 2026-10-04.
Date: 2026-10-04

## Context

Read admission must not imply write admission. A layer readable from an
arbitrary SQL query or a tile-optimized source is not automatically safe
to write.

## Decision

1. Write eligibility is proven per (backend, DB version, source shape,
   geometry storage, schema, transaction guarantees). `writable=true`
   alone never grants it.
2. Admission checks: physical base relation, stable PK/identity,
   generated/default columns, nullable/precision, geometry column/type/
   dimension/CRS, privileges, transactional engine. A read-compatible SQL
   or table is not assumed write-safe.
3. Non-transactional engines (e.g. MyISAM), arbitrary `feature_sql`
   (first release), tile SQL and MVT-encoded sources are read-only by
   default with no automatic reverse mapping.
4. Schema drift after startup fails closed until re-admission. No
   automatic migration of user schema on `serve`.
5. Revision strategy is an explicit column (+ trigger) or another proven
   backend mechanism. If external SQL writers do not maintain the
   revision, the profile does not claim lost-update detection for them.

## Consequences

Each backend documents its admitted profiles separately; anything else
stays read-only even when the same backend writes another shape.

## Verification and acceptance record

The per-backend `mutation_contract_test.go` suites. Negative controls must actually fail when a protection is
disabled.
