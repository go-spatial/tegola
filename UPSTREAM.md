# Upstream provenance and fork maintenance

This repository is a fork of `go-spatial/tegola`, based on upstream master (post-v0.21.0), after
v0.21.0. The Go module is `github.com/alexeydott/tegola`. Fork changes are recorded
in [CHANGELOG.md](CHANGELOG.md); operating contracts are indexed in
[docs/README.md](docs/README.md).

## Releases and current source

The immutable source tag `v0.21.0-fork.2` identifies commit
`db4e8ee73a3ddfe3b3c8e054c59e833b697dce85` (2026-10-02). It is a historical release
baseline, not the current master. Subsequent source changes include WFS and
conditional feature mutations, native MOS writes and writable tile-cache generations.
See [release scope](docs/release/feature-api.md) for the tagged read profile and
[write scope](docs/wfs-scope-limitations.md) for the current editing profile.

Fork tags use `v0.21.0-fork.N`. Build metadata is injected through
`internal/build.Version`, `GitRevision` and `GitBranch` using linker flags.
A plain build can still report the historical `v0.21.0-fork.1` fallback; use
`tegola version` and the full revision to identify a binary. A source tag does
not establish that release assets, a deployed installation or OGC certification exist.

## Fork extensions

| Area | Current implementation | Contract |
| --- | --- | --- |
| Providers | PostGIS, MySQL/MariaDB, GeoPackage/raw SQLite and SAP HANA; native MVT variants remain separate from source features | [Provider contract](docs/provider-contract.md) |
| Features | Explicit publication, JSON/HTML, paging, Queryables, typed CQL2 text subset, datetime and admitted public CRS | [HTTP API](docs/api.md), [filtering](docs/filtering.md) |
| Writes | Opt-in REST mutations and WFS transactions on admitted provider layers, schema validation, transaction domains and revision checks | [Write scope](docs/wfs-scope-limitations.md), [recovery](docs/operational.md) |
| MOS | Raw geometry decoding plus transactional geometry/separate-bounds writes on admitted MySQL and SQLite profiles | [Geometry formats](docs/geometry-formats.md) |
| CRS | Standard projections and owned custom horizontal etmerc/datum transformations; unsupported vertical/grid profiles fail explicitly | [CRS contract](docs/crs.md) |
| Tile caching | Memory/file/multilevel and external backends; seeded-map layer reuse; authenticated metatile maintenance | [Server guide](server/README.md) |
| Writable caching | Ordinary viewing uses router-local cache generations; editor header bypass; committed/unknown writes invalidate generations; HTTP no-store | [Write scope](docs/wfs-scope-limitations.md) |
| Viewer and runtime | Embedded MapLibre viewer with attribute editing; optional Lambda adapter over the HTTP router | [Viewer](ui/README.md), [Lambda](cmd/tegola_lambda/README.md) |

Native-provider evidence is bounded by the versions and profiles in the
[provider matrix](docs/provider-matrix.md). An implementation or fixture pass is
not proof of unrestricted production acceptance. HANA feature writes,
LockFeature, distributed invalidation and wire-level fault/restore acceptance
are not implied by the supported read or write paths.

## Upstream contribution candidates

The following fixes are present in this fork and can be evaluated for upstream
submission. Reproduce against the actual upstream revision before proposing a
patch; this document does not claim the latest upstream still contains a bug
or that a pull request has been submitted.

- GCS: distinguish a missing object from other backend read failures.
- Hostname validation: reject malformed configured URLs while preserving the
  empty-hostname request-derived behavior.
- HANA: identifier quoting, native placeholders and SQL row cleanup.
- File cache: unique temporary files and safe missing-file purge races.
- HTTP gzip adaptation: decode a buffered response once and update content length.
- Geometry dependency: validate WKB element/point counts before allocating.

Dependency changes belong in the standalone geom/proj forks, not in Tegola's
frozen historical third_party snapshots.

## Dependencies and sync surface

Tegola uses tagged `github.com/alexeydott/geom` and `github.com/alexeydott/proj`
modules with generated vendor contents and no local replacements. See
[dependency ownership](third_party/README.md) and `go.mod` for authoritative pins.
The geom fork uses protobuf APIv2. The embedded viewer uses Vue 3, MapLibre GL JS
5 and Vite; lockfiles and the install-script policy are maintained in `ui/`.

AWS S3 still uses `aws-sdk-go` v1 and Azure Blob uses the legacy
`azure-storage-blob-go` package. Migration to newer SDKs remains separate work.

To compare with an already configured and refreshed upstream remote:

```sh
git merge-base HEAD upstream/master
git log upstream/master..HEAD
git log HEAD..upstream/master
```

Review provider contracts, generated protobuf code, dependency pins and viewer
assets separately when integrating upstream changes. Preserve license notices
and distinguish inherited behavior from fork extensions.

## Verification policy

CI definitions live in `.github/workflows/`; a workflow file does not establish
a successful run. Record actual commands and results for the exact candidate,
including skipped native databases and cloud services. For local verification:

```sh
CGO_ENABLED=0 go test -mod=vendor -count=1 ./...
CGO_ENABLED=1 go test -mod=vendor -count=1 ./...
golangci-lint run ./...
```

CGO tests require a C compiler; live provider suites require their configured
services and explicit opt-in variables. Use the build and test instructions in
[development](docs/development.md). `.golangci.yml` enables correctness-oriented
linters; its Rows.Close and io.Closer.Close exclusions do not replace SQL
iteration/error regressions. Tests are also required for errors that are checked
but then incorrectly swallowed.

## See Also

- [Changelog](CHANGELOG.md)
- [Support and maintenance](docs/maintenance-status.md)
- [Windows release builds](docs/windows-release.md)