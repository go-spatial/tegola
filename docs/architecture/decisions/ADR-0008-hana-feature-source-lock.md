# ADR-0008: HANA feature reads with a transaction-owned source lock

Status: Accepted architectural decision by the user on 2026-10-02 after independent review of the single-connection proposal. Production admission remains contingent on the implementation and runtime gates below.

## Context

[ADR-0007](ADR-0007-safe-provider-feature-sql.md) requires a stable physical source throughout each feature query. Actual HANA 2.00.088 tests showed that a read-only repeatable-read transaction permits concurrent ALTER and observes the changed schema. EXCLUSIVE table locking in that transaction is rejected. A separate read-write guard connection can protect the table, but losing that guard can release protection independently of the reader.

The independently reviewed replacement uses one physical connection and one transaction for both protection and reads. Actual probes confirmed that its EXCLUSIVE lock blocks concurrent ALTER and UPDATE, and that disconnecting the transaction causes subsequent reads to fail without rebinding.

## Decision

HANA may use a single reserved **read-write repeatable-read transaction** for feature queries. The trusted executor issues source reads and metadata reads only, together with transaction setup, source locking and cleanup; it never executes configured SQL verbatim or performs source DML. The constrained selection and exact dimensional/temporal rules of ADR-0007 remain mandatory.

Acquire an EXCLUSIVE lock on the frozen qualified physical source before validating its retained table identity, columns, key, spatial metadata and CRS. All candidate chunks, integrity checks and callbacks belong to that same transaction. Do not create a separate guard transaction, retry a failed query, rebind a lost transaction or adopt a replacement source.

This protocol requires permissions sufficient for a read-write transaction and EXCLUSIVE locking. It blocks source writers and DDL for the protected query's duration. Concurrent readers and finite connection pools must be tested explicitly. No application-wide callback mutex or second connection reservation is required.

Use the earlier of the caller's deadline and a bounded feature-query deadline, initially 30 seconds. Lock acquisition must obey that deadline. Cancellation must stop further reads and callback dispatch and release transaction resources. A synchronous caller callback must cooperate with cancellation; the API does not claim to forcibly terminate caller code. Test cancellation while rows are open and while a callback is active, including timely server-side lock release.

Close rows, roll back the transaction and physically discard the reserved connection on every path, including setup failure, lock failure, cancellation and transaction loss. Do not return a session with changed transaction state to the shared pool. A failed transaction must return an error; it must not fall back to tiles, another source or an unprotected query.

## Native geometry admission

Use authoritative catalog coordinate-dimension metadata, not sampled rows or topological dimension. The actual `SYS.ST_GEOMETRY_COLUMNS.COORD_DIMENSION = 2` profile proves XY storage; unrestricted dimension 4 is not an XY-only guarantee. Native XYZ or mixed admission requires its own reviewed catalog, export and vertical-profile proof. Strict validation must precede empty-geometry normalization and callbacks.

An explicitly configured native XY, XYZ or mixed profile may use catalog dimension-4 storage only after lossless ISO-dimensional export and separate SRID evidence are independently verified for the supported families. Dimension 4 describes a permissive storage envelope; it does not certify the configured dimensions. Validate every encountered body, rejecting M/ZM, unsupported families and declared-dimension mismatches before filtering, paging or callbacks. Never truncate or coerce ordinates. XYZ/mixed requires the canonical horizontal and CRS84h vertical profiles of ADR-0005. Runtime admission remains contingent on actual family, dimensional and rejection tests.

Permanent column tables may report `SESSION_TYPE = SIMPLE`; row tables may report `NONE`. Admit these only with the corresponding table type and independently checked permanent status, absent masking/policies and absent temporal-history features. Reject HISTORY and unknown session profiles. Capture and revalidate session type with the remaining source fingerprint; SIMPLE alone does not prove permanence.

Native CRS admission uses an exact allowlist of independently verified full SRS tuples, including definition, transformation definition, round-earth flag and organization identity. Capture and compare the tuple during registration and locked source revalidation; labels or EPSG substrings alone are insufficient. Output interpretation uses the frozen canonical adapter and unchanged stored ordinates. Native reads may export geometry and its stored SRID, but must not perform database projection or depend on a mutable registry to interpret coordinates. The source-table lock does not claim to protect global SRS registry updates or prevent registry ABA changes. Observed tuple drift fails closed; a registry change cannot select a different runtime transformation.

The tested character-parameter transport through trusted NVARCHAR/DECIMAL casts preserves signed 38-digit values and scale-38 fractions. Keep bounded numeric normalization and validated catalog precision/scale; this transport result does not establish source locking or geometry eligibility.

## Admission gates

Before production capability is enabled, independently review the final source and execute actual-server tests for:

- source locking before catalog/key/CRS/dimension verification;
- concurrent ALTER, DROP, RENAME and drop/recreate replacement;
- stable data snapshots and blocked concurrent writers;
- cancellation during lock acquisition, row consumption and callbacks;
- connection/transaction loss without retry or source rebinding;
- setup failures, physical discard and lock cleanup;
- finite pools and concurrent queries/layers;
- ordinary/custom raw selections, exact 3D and temporal predicates, paging and ownership;
- each admitted native profile, malformed wire/storage enforcement and empty geometry.

Unsupported profiles remain explicitly unsupported. Architectural approval is permission to implement and test this protocol, not runtime acceptance of a provider profile.

## Sources

- [SAP HANA SET TRANSACTION](https://help.sap.com/docs/SAP_HANA_PLATFORM/4fe29514fd584807ac9f2a04f6754767/20fdf9cb75191014b85aaa9dec841291.html)
- [SAP HANA LOCK TABLE](https://help.sap.com/docs/SAP_HANA_PLATFORM/4fe29514fd584807ac9f2a04f6754767/20f88d8d75191014a51abbaa4e3d36cb.html)

## Consequences

HANA source protection differs from the read-only transactions used by other providers. Its stronger table lock has an explicit writer-blocking cost. Source and runtime evidence must distinguish the approved protocol from the earlier read-only and separate-guard experiments.
