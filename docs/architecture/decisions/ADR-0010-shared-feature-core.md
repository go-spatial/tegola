# Write ADR-0010: One feature core, two transport adapters (WFS and OGC API Features)

Status: accepted on 2026-10-04.
Date: 2026-10-04

## Context

WFS (KVP/XML, GML, Filter Encoding) and OGC API Features Part 4
(JSON, GeoJSON, merge-patch) need the same business semantics: one object
has one identity, one schema, one set of rights, one storage CRS and one
revision across protocols. Two independent write stacks would diverge.

## Decision

1. `feature/` holds neutral services: schema descriptors, logical/physical
   identity, authorization policy, query coordinator, mutation coordinator.
   It owns `FeatureRecord` (logical identity, owned geometry, typed values,
   source CRS descriptor, revision, schema version).
2. `provider/mutation.go` defines `MutationProvider` / `FeatureTx` /
   `Mutation` / `MutationOutcome` / `CommitReceipt`. `Apply` executes
   transaction-bound selection, locking, revision/authorization checks and
   post-image validation itself; it never calls the read-snapshot
   `FeatureQuerier.QueryFeatures` for state-dependent checks.
3. `ogc/wfs/` and the Part 4 REST handlers are thin adapters: they parse
   their wire format, build neutral commands, invoke the core, and render
   version-specific responses/errors. WFS never HTTP-calls the REST
   endpoint; REST never builds XML Transactions.
4. Existing `FeatureQuerier` stays callable and unchanged. The new
   query/mutation contract gets an adapter; tilers are not forced to
   implement writes.

## Consequences

Protocol bugs are fixed in adapters; semantic bugs in the core. Adding a
third protocol later means a new adapter, not a new mutation engine.

## Verification and acceptance record

Contract tests execute the same logical commands against every admitted
writer and assert identical outcomes. Cross-protocol CRUD tests
prove one identity across WFS-T and Part 4.
