# Synthetic feature-query regression runner

Run with Go 1.26.9 and CGO enabled. The runner creates only missing synthetic
datasets in the specified directory; existing datasets are queried read-only.
Use a dedicated benchmark directory. It never requires a user GeoPackage or DB credentials.

```powershell
go run -mod=vendor ./benchmarks/features -data-dir D:/temp/tegola-feature-bench
go run -mod=vendor ./benchmarks/features -data-dir D:/temp/tegola-feature-bench -observe -reference approved-baseline.jsonl
```

`-sizes 10000,100000` selects a quick subset. The full default matrix covers
10k/100k/1M/10M native points, ID, selective/broad bbox, limits10/100/1000,
datetime, parsed property filter, CQL2 parse plus query, and EPSG3857 output.
ID/time/property indexes and an RTree exist. Source-integrity/absence checks
can still scan SQL rows; indexes do not promise constant-time execution.

Each call has a 30-second context. A warmup and three measured batches record
median, sample maximum and mean total allocation per query. Fast classes batch20
to reduce timer granularity. Three samples are **not** a statistically credible p95.
The independent analytic grid oracle checks all returned IDs, source/output
coordinates, properties, paging, and exact-versus-unknown count semantics outside timing.
Unknown matched counts are valid after lookahead and remain unknown.

The optional reference is a frozen JSONL output from the same toolchain, host,
dataset hashes, query/profile and observer mode policy. Accepted local budgets:
median <=min(30s,max(1ms,reference*1.35)), sampled maximum <=min(30s,max(2ms,reference*1.35)),
and mean allocation <=reference*1.20+4096 bytes. An error, timeout, semantic failure
or exceeded budget exits nonzero. Capture pre/post source, binary and dataset hashes.
Observer-enabled and disabled runs should be reported separately.

These budgets measure local point-profile regressions. They are not an HTTP,
remote database, polygon-complexity, concurrent-load or deployment SLA.
Large GPKG scalar filters perform required same-snapshot integrity checks and
can allocate substantially; record measured costs rather than extrapolating a universal budget.
