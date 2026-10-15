# Published dependency forks

Tegola now depends directly on tagged Go modules:

| Module | Version | Repository |
| --- | --- | --- |
| `github.com/alexeydott/geom` | `v0.1.1` | https://github.com/alexeydott/geom |
| `github.com/alexeydott/proj` | `v0.3.1` | https://github.com/alexeydott/proj |

The root module is `github.com/alexeydott/tegola`. There are no dependency
`replace` directives. Remote consumers receive the patched dependencies through
ordinary Go module resolution; they do not need a Tegola checkout or local
geom/proj replacements. `vendor/` is generated from those published versions
and supports offline Tegola builds.

## Migration for Go consumers

Change imports from `github.com/go-spatial/tegola`, `github.com/go-spatial/geom`
and `github.com/go-spatial/proj` to the corresponding `github.com/alexeydott/`
paths. Fork geom types have a distinct Go package identity: an upstream
`geom.Point` or `geom.Extent` is not interchangeable with the fork type.
Tegola configuration files and HTTP endpoints are unaffected by this namespace
migration. Pin Tegola to a reviewed commit or release:

```sh
go get github.com/alexeydott/tegola@<commit-or-tag>
go mod tidy
go test ./...
```

No `replace` directives are necessary. As usual, initial downloads require
network access or a populated Go module cache. Consumers can run `go mod vendor`
and subsequently build using `-mod=vendor` without network access.

## Source ownership and provenance

The standalone fork repositories are the source of truth for dependency
changes. They preserve the upstream Git history and license notices; their
`FORK.md` files identify the upstream bases and Tegola audit snapshot. The first
published versions use go-spatial/geom v0.1.0 and go-spatial/proj v0.3.0 as bases,
with Tegola fixes through commit `f1111b31` and the alexeydott namespace migration.

The directories `third_party/go-spatial/geom` and `third_party/go-spatial/proj`
remain as **frozen historical snapshots**, preserving audit references. They
are not build inputs and must not be edited to update a dependency. Their old
module paths are historical, not an instruction to restore local replacements.

For a dependency change:

1. Edit and test the standalone fork, retaining copyright notices.
2. Publish a new immutable version tag; never move an existing tag.
3. Update Tegola's requirements, run `go mod tidy` and `go mod vendor`.
4. Run the root tests and external-consumer check.

Geom uses protobuf APIv2; keep its generated MVT code and `go_package` path in
sync with `github.com/alexeydott/geom`. Proj includes thread-safe registration,
cache invalidation and projected/geographic three-/seven-parameter datum shifts.

## Portability verification

```sh
go run -mod=vendor ./ci/check-dependencies
# After publishing the Tegola commit:
go run -mod=vendor ./ci/check-dependencies -revision <published-commit-or-tag>
```

The default check uses a replacement only for the Tegola checkout under test;
geom and proj must resolve from their published, pinned versions. The remote
mode fetches Tegola without replacements into an isolated module cache. Both
exercise public fork geom types, protobuf APIv2, degenerate MVT handling and
datum transformations, then verify an independently vendored offline consumer.
Historical snapshots are excluded from active dependency verification.
