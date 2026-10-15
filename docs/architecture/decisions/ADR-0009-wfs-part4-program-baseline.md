# Write ADR-0009: Shared WFS and OGC API Features mutation architecture

Status: accepted on 2026-10-04; historical read-only baseline superseded by the implemented mutation runtime.

## Context

Tegola already served MVT and raw feature reads. Adding edits must preserve opt-in publication and existing clients while sharing identity, schema, authorization and atomic outcomes across protocols.

## Decision

Implement WFS 1.1.0/2.0.0/2.0.2 reads and Transaction plus the selected OGC API Features Part 4 draft.3 mutation profile in the existing server. WFS is a native adapter, not a proxy to REST.

Use one common `feature/` core and provider mutation contracts. Protocol adapters convert input documents into neutral commands; only native providers execute SQL. Writes default off. Read admission never implies write eligibility; unsupported source shapes fail closed.

Normative editions: WFS 1.1 OGC 04-094r1, WFS 2.0 OGC 09-025r2, FES OGC 09-026r2 and Part 4 20-002r2 draft.3 at `1c68987e1a5222e9a488a7b089f7246010f3e83c`. Draft support does not establish final-standard conformance or certification.

## Consequences and verification

`feature/` owns neutral services, `provider/mutation.go` owns native contracts, `ogc/wfs/` owns WFS/GML/filter adaptation, and `server/` owns HTTP handling. Backend admission and actual service evidence remain profile-specific. See the [current scope and limitations](../../wfs-scope-limitations.md).
