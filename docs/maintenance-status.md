[← Geometry Formats](geometry-formats.md) · [Back to README](../README.md) · [Windows Release →](windows-release.md)

# Support and maintenance

This page describes the maintained source and its operating boundaries.
Release history is in [CHANGELOG.md](../CHANGELOG.md); source provenance and
upstream integration are in [UPSTREAM.md](../UPSTREAM.md).

## Release versus current source

`v0.21.0-fork.2` is the immutable 2026-10-02 source release. Current master also
contains later WFS, feature-write, MOS and cache changes. Do not identify a
current binary by that tag alone: inspect `tegola version`, its Git revision,
build tags, configuration and loaded viewer assets.

## Maintained capabilities

| Area | Behavior | Reference |
| --- | --- | --- |
| Tiles | MVT output, standard/native MVT providers, geometry processing and tile diagnostics | [API](api.md), [provider contract](provider-contract.md) |
| Feature reads | Published collections, stable identifiers, typed filtering, paging, datetime and admitted CRS | [API](api.md), [filtering](filtering.md) |
| Editing | Conditional REST writes and WFS transactions for explicitly admitted schemas; native geometry/null/bounds handling | [Write scope](wfs-scope-limitations.md), [geometry](geometry-formats.md) |
| Cache | Seeded whole-map tiles can serve layer URLs; authenticated metatile maintenance; server cache generations after writes | [Server guide](../server/README.md) |
| Provider evidence | Separate native spatial and MOS profiles with database versions and unperformed scenarios | [Provider matrix](provider-matrix.md) |
| Builds | Go module/vendor builds, CGO GeoPackage, embedded viewer and revision-tagged Windows binaries | [Development](development.md), [Windows builds](windows-release.md) |

## Operating boundaries

- Write publication is opt-in and admission-based. The stock CLI does not supply
  a production authentication adapter; do not use a demo write profile as an
  authenticated production deployment.
- Successful and uncertain transactions invalidate tile generations within the
  writable router. External SQL changes and other reader processes are not
  coordinated. Writable caches start cold after restart; obsolete generation
  storage requires backend expiry or cleanup.
- Custom projections are horizontal. Grid/vertical transformations, unsupported
  axes and units are not silently approximated. See [CRS limits](crs.md).
- Native MVT providers return encoded tiles; they are not source-feature providers.
- The built-in viewer edits attributes. External geometry editors are clients of
  the documented API and are not bundled as a complete geometry-editing viewer.
- No full WFS-T/Part 4 certification, distributed locking or LockFeature support
  is claimed. Conformance declarations describe the admitted runtime profile.
- Native HANA, cloud services and deployed Lambda require separate environment
  verification. Local fixture tests do not establish their acceptance.

## Maintenance work

AWS/Azure SDK migration remains outside the current provider/cache changes.
Upstream submissions and successful remote CI runs must be verified separately;
source files or local tests do not prove either has occurred.

For a defect report, provide the exact revision and configuration (redact
credentials), a minimal request/data fixture, expected versus observed behavior
and relevant logs. Follow [contribution guidance](../CONTRIBUTING.md) and use the
[security policy](../SECURITY.md) for sensitive reports.

## See Also

- [Upstream provenance](../UPSTREAM.md)
- [Provider matrix](provider-matrix.md)
- [Write operations and recovery](operational.md)