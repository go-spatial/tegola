[Documentation index](../README.md)

# Feature performance and observability

## Metrics

Install the existing Prometheus observer to expose dedicated feature metrics
through its metrics handler and push lifecycle. Collectors belong to that observer
instance. Feature publication without an observer continues to work.

| Metric | Meaning |
|---|---|
| `tegola_feature_requests_total` | Completed feature HTTP requests, including HEAD, OPTIONS and error responses. |
| `tegola_feature_request_duration_seconds` | Duration of the feature HTTP pipeline. |
| `tegola_feature_provider_queries_total` | Actual provider query attempts. Validation rejected before provider invocation does not increment this counter. |
| `tegola_feature_provider_query_duration_seconds` | Provider reading, decoding and synchronous callback processing time. |
| `tegola_feature_rows_returned_total` | Rows accepted by callbacks, including partial progress before a failed query. |

Request labels use fixed resource, method and status classes. Query labels use
fixed backend, query class, scalar-filter pushdown and outcome values. Unknown
metadata stays unknown. Collection names, object IDs, URLs, filter text, SQL,
credentials and error values are excluded from these labels.

The scalar SQL-filter marker requires both an actual query filter and explicit
frozen provider metadata. It does not establish spatial or temporal indexing.
Query classification preserves a valid datetime request even when the collection
has no temporal geometry and the service removes its match-all predicate.

## Repeatable performance gates

Use owned deterministic point GeoPackages at 10k, 100k, 1M and 10M features.
Measure ID, selective/broad bbox, page limits 10/100/configured maximum, datetime,
property filters, CQL2 parsing with query execution, and CRS transformation.
Record exact source/compiler/environment and dataset/schema/index hashes before
execution and check drift afterward. Verify IDs, geometry, paging and optional
count semantics before accepting a timing result.

After one warmup, measure three batches. Batch fast operations to accommodate
timer resolution; report median and sample maximum. These samples do not
establish a statistical p95. Against an independently accepted reference, apply:

| Gate | Per-query ceiling |
|---|---|
| Median time | `min(30 s, max(1 ms, reference median × 1.35))` |
| Sample maximum | `min(30 s, max(2 ms, reference sample maximum × 1.35))` |
| Mean total allocation | `reference allocation × 1.20 + 4096 bytes` |

An error, timeout, semantic mismatch or source drift fails the gate regardless
of timing. Measure both disabled and enabled observation. Freeze the reference
before comparing the candidate; retain failed and superseded attempts.

Run the repeatable matrix using the commands in the
[benchmark runner guide](../../benchmarks/features/README.md).

## Operational scope

These budgets describe a local synthetic point-GPKG profile on the recorded
hardware and toolchain. Remote databases, line/polygon complexity, cold caches,
concurrent workloads and HTTP response sizes require their own profiles.
The configured maximum page size is part of each reference.

Source-integrity checks and exact predicates can scan many candidates even with
a small page limit. The isolated reference measured a 10M broad-bbox median
of about 22.0 seconds and a sample maximum of 22.6 seconds under a 30-second
query deadline. Scalar
filter measurements allocated about 880 MB in total per query. Total allocation
is distinct from peak resident memory. These costs require capacity planning;
the encoded-response cap does not bound every provider or encoder allocation.

## See Also

- [Conformance runner](ogc-conformance.md)
- [Publication limits](../configuration.md#feature-publication)
- [Observability decision](../architecture/decisions/ADR-0013-feature-observability.md)
- [Feature reliability boundary](../architecture/decisions/ADR-0012-feature-conformance-and-panic-containment.md)
