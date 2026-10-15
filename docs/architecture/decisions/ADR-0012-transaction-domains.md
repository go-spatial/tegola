# Write ADR-0012: Transaction domains and unknown commit outcome

Status: accepted on 2026-10-04.
Date: 2026-10-04

## Context

WFS Transactions can touch several layers; Part 4 does single-resource
operations. All-or-nothing must be real, not best-effort.

## Decision

1. A transaction domain is the set of layers one native writer executes on
   one connection in one native transaction. Provider names alone do not
   define atomicity boundaries.
2. One WFS Transaction inside one domain is all-or-nothing. A request
   spanning two independent databases is rejected before any change.
   Compensating operations are never presented as atomicity.
3. Structural validation of all actions happens before Begin;
   state-dependent predicates, rights, locks, preconditions and post-image
   checks run inside the transaction. Document order is preserved and
   later actions see earlier changes.
4. `CommitReceipt` distinguishes committed / not-committed / outcome-
   unknown. A lost acknowledgement after COMMIT is reported as unknown,
   never silently retried for Insert. Audit/outbox records are written in
   the same native transaction as the data.
5. Error taxonomy: malformed input, schema violation, denied, not found,
   precondition failed, lock conflict, unsupported capability, quota
   exceeded, transaction-domain mismatch, commit unknown. Each protocol
   adapter maps them to its own response; they are not all 409.

## Consequences

Multi-database atomicity is out of scope for the first release. The UI
treats outcome-unknown as "reconcile, do not blindly resend".

## Verification and acceptance record

Transaction tests cover one native transaction, mid-transaction rollback, document order,
cross-domain rejection, unknown-commit handling, outbox atomicity.
