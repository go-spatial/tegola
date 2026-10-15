# Jivan source provenance

[Documentation index](../README.md) · [Migration guide](jivan-to-tegola.md)

## Audited source

The migration reference is [go-spatial/jivan at commit `c9fba2bb5188ba43c27539740195085e92f0ad7d`](https://github.com/go-spatial/jivan/tree/c9fba2bb5188ba43c27539740195085e92f0ad7d). Its 44-file source archive was inspected for configuration, models, templates, routes, provider behavior and notices. The archive SHA-256 is `f079e7fad6a71d6d58a3864fa3d0ec9b918c8e9ed2a6e14b973f7a42d54a5447`.

Jivan informed the route inventory and migration requirements. OGC requirements and Tegola's accepted contracts govern the replacement implementation. The source was used as a reference; the inventory below does not claim that these files were incorporated into Tegola.

| Reference path | Recorded SHA-256 | File notice |
| --- | --- | --- |
| `server/routes.go` | `150cb7f2d21c497ad6d10a69deab511d2b9108d257e5445a4df05874726f9e5e` | Copyright (c) 2018 Jivan Amara; Copyright (c) 2018 Tom Kralidis |
| `server/handlers.go` | `e76f4b8810a98654d2772fcc553a18d7b22b75e3c1fcaac5e00e08d3b30b1945` | Copyright (c) 2018 Jivan Amara; Copyright (c) 2018 Tom Kralidis |
| `data_provider/provider.go` | `2b1b1e412881c20a38ae6deae33781d5eb4bb20f7553b648f567528fbc11d4f4` | Copyright (c) 2018 Jivan Amara |
| `wfs3/features.go` | `09b48f36f42a3f73334fb3db7bf5e59c780dcfc8409dc15dd0f505f89a4a6e4b` | No file-specific copyright header; repository license applies to the reference inventory. |
| `wfs3/openapi3.go` | `e5736f5ce415cb2944edbf545b62b415a8454a11d62cdc43bfada5b70e066ff7` | Copyright (c) 2018 Jivan Amara; Copyright (c) 2018 Tom Kralidis |
| `wfs3/conformance.go` | `ad5c55dc2e9692ef6ca24234397589ca64622c5c7570ccc3448006de5fdf6a87` | Copyright (c) 2018 Jivan Amara; Copyright (c) 2019 Tom Kralidis |
| `server/server_awslambda.go` | `11d7a8660c7ca6a3297e9c260f6e2a4bc90cef570d7fec5e9455bba98527f3cc` | Copyright (c) 2018 Jivan Amara |
| `config/config.go` | `48edf21bef5f0531c3b39e0f510530e0b019ac2cddbad192f75b438ccffee982` | Copyright (c) 2018 Tom Kralidis |
| `wfs3/html_templates.go` | `2692692f57091d96e0170a8488435409315536cf5b86638e687f7b4560fc2761` | Copyright (c) 2018 Tom Kralidis |
| `LICENSE` | `9ea8a12deea1b232d639881e35b0540f8678315ebcc27053a9d8359bd78ad040` | Copyright (c) 2018 |

## Implementation provenance and limits

The audited Tegola implementation range is `c65beeb8519f425ff8365c76e54e93baf8e17b07` through `a4913f8d`. Comparison of the pinned Jivan text sources with changed Tegola product files found no substantial exact code blocks under the audit's screening rule: at least five consecutive matching lines containing at least three substantive lines, excluding ordinary braces and syntax. This mechanical comparison cannot establish the absence of adaptation, common ancestry or other reuse.

The contributing agents disclosed the following for their own scopes:

- Implementation disclosure: provider contracts, MySQL, shared geometry/query contracts, service/CRS/OpenAPI, metrics and benchmarks were authored from existing Tegola code, accepted designs and normative sources; no Jivan source, models or templates were intentionally copied or adapted.
- Implementation disclosure: GeoPackage/HANA integrations, geometry helpers and reliability tests were authored from Tegola contracts and database documentation; no Jivan code was copied.
- Implementation disclosure: service, HTTP and HTML work and migration prose were newly authored; pinned Jivan behavior influenced the migration requirements, without intentional source or template copying.
- Implementation disclosure: query harnesses, codecs, parser, HTTP acceptance and conformance fixtures were newly authored. Migration fixtures use Tegola fixtures; Jivan was inspected for behavior comparison.
- The orchestrator disclosed newly written design and documentation work, without copying Jivan implementation code.

These are bounded author disclosures, not an independent authorship guarantee or a legal clean-room certification. No substantial reused Jivan implementation has been confirmed by this audit. Any later copied or adapted material must be recorded with its source path and revision, retaining the applicable source notices.

## Lambda migration addendum

The later Lambda adapter work is outside the historical comparison range above. The implementation record states that the new [`server/lambda/handler.go`](../../server/lambda/handler.go), its handler tests and [`cmd/tegola_lambda/main.go`](../../cmd/tegola_lambda/main.go) wiring were newly authored from Tegola's existing Lambda entry point, the existing algnhsa/AWS dependencies and the accepted transport design. No Jivan Lambda source or text was copied or adapted in that work. The [`server/lambda/migration_acceptance_test.go`](../../server/lambda/migration_acceptance_test.go) exercises the new adapter and independently declared Tegola migration fixtures; its author likewise disclosed no copied Jivan source.

This addendum records the authors' bounded disclosures for these paths, rather than extending the earlier mechanical comparison to unexamined later code. The final release evidence binds the reviewed files to their frozen hashes. Sharing algnhsa with Jivan does not make either adapter a copy of the other or alter that dependency's own license.

## Notices and dependencies

The [preserved Jivan license](../../third_party/jivan/LICENSE) is an exact copy of the pinned repository's MIT license, including its 2018 copyright notice. It acknowledges the referenced source conservatively; it does not assert that every Tegola feature derives from Jivan. Named notices in the table remain visible even though the repository license itself contains no author name.

Tegola's [root MIT license](../../LICENSE.md) remains unchanged. No `go.mod`, `go.sum`, vendored dependency or historical dependency snapshot changed within the audited implementation range. The existing Lambda dependency `github.com/akrylysov/algnhsa` has its own Apache-2.0 license in the vendored source; it is not covered by the Jivan MIT notice. Published geom/proj fork provenance remains documented in [third-party dependencies](../../third_party/README.md).

The referenced source archives and author-audit records are local evidence, not shipped copies of the Jivan implementation. The operator migration does not change or archive the independently managed Jivan repository.

## See also

- [Pinned behavior inventory](jivan-feature-matrix.md)
- [Migration guide](jivan-to-tegola.md)
- [Feature Service architecture](../architecture/feature-service.md)
