# Read ADR-0013: Feature observability and measured performance budgets

Status: Accepted after independent architecture and security design review on 2026-10-02. Implementation and the local performance matrix independently accepted within the measurement scope below.

## Decision

Feature traffic has dedicated request and provider-query metrics. Extend the
existing installed observability instance through optional interfaces; do not
expand mandatory provider or observer interfaces or register process-global
collectors. Export collectors through the same instance registry used by its
existing metrics handler and push lifecycle.

The service accepts an optional query observer through an immutable service copy.
Observe only actual provider calls, with timing immediately around QueryFeatures.
Duration includes provider reading/decoding and synchronous callback processing;
it is not isolated SQL execution latency. Count successfully accepted callbacks,
including partial progress before an error. Preserve business results, errors
and error chains. Pre-provider validation failures do not become provider calls.

Snapshot optional provider execution metadata once during catalog construction.
Fixed backend enums and scalar-filter execution enums describe implementation
behavior. Missing metadata, getter errors or invalid enum values become unknown
and do not invalidate Core publication. Mark SQL scalar filtering only when the
actual query has a filter and frozen metadata confirms it; never infer pushdown
from the presence of a filter alone. This marker makes no claim that spatial or
temporal validation is entirely indexed or executed in SQL.

Classify the original validated query before clearing a temporal constraint for
known absent temporal geometry. Use fixed query classes; combined predicates
become mixed. Request metrics use fixed resource, method and status classes and
count once after the feature panic guard has selected its response, including
HEAD, OPTIONS, rejection and panic errors. When the installed observer supports
the dedicated feature request interface, feature routes use that instrumentation
without the generic URL-labelled API histogram. Custom observers implementing
only the generic API interface retain their existing fallback. Tile traffic
retains its serving path.

Normalize every collector input to a bounded enum. No request URL, collection
name, object ID, filter text, SQL, credential, error value or user-defined label
is permitted. Observe value-only summaries without retaining a query or context.
An observer callback panic is contained around instrumentation delivery alone,
with a fixed diagnostic and unchanged business result. Business panics remain
subject to the separate feature HTTP recovery boundary.

## Measurement and budgets

Use owned deterministic datasets and actual provider execution for 10k, 100k,
1M and, where feasible, 10M features. Record source/compiler/environment,
dataset/schema/index hashes, query class, returned rows, nullable counts, raw
timings, allocation measurements and cancellation. Unknown counts stay unknown.
Fast queries use batches to avoid timer-resolution artifacts. Sparse samples
are reported as median and sample maximum, not statistically established p95.

Set numeric relative and absolute acceptance limits after baseline measurement
and independent review. Bind budgets to the dataset, provider, query profile,
environment and repeatable runner. Local synthetic GPKG measurements do not
establish remote database or HTTP production latency guarantees. Broad bbox or
integrity validation may still scan many candidates despite a small page limit;
document measured costs and timeout headroom.

## Verification

Tests cover private registry isolation, actual handler scraping, fixed labels,
error/cancellation/partial callback outcomes, immutable observer injection,
unknown metadata, instrumentation panic isolation and concurrency. HTTP tests
confirm one feature request event for success, errors, HEAD, OPTIONS and panic,
with no tile behavior change. Final performance gates use accepted numeric
budgets and exact source/configuration bindings.

## Related decisions

- [Feature service boundary](ADR-0001-feature-service-boundaries.md)
- [Conformance and panic containment](ADR-0012-feature-conformance-and-panic-containment.md)
