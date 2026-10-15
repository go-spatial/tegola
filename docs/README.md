[Back to README](../README.md) · [API Reference →](api.md)

# Documentation Index

Tegola documentation describes current HTTP behavior, configuration, providers,
geometry, editing and builds. The immutable `v0.21.0-fork.2` tag is a historical
read-profile release; current master also contains later WFS/MOS write and cache
changes. See [release scope](release/feature-api.md) and
[support boundaries](maintenance-status.md).

## Guides

* [API reference](api.md) — HTTP endpoints and tile operations.
* [WFS and feature editing](wfs-scope-limitations.md) — write profile, embedded attribute editor, provider limits and operational requirements.
* [Write provider evidence](provider-matrix.md) — native versions, covered scenarios and explicit NOT_RUN boundaries.
* [Write operations and recovery](operational.md) — backups, restore, schema admission and uncertain commit outcomes.
* [Queryables and filtering](filtering.md) — public scalar catalogs and the initial CQL2 text profile.
* [Configuration](configuration.md) — TOML, providers, caches, and environment variables.
* [Development and builds](development.md) — debugging, build flags, and source builds.
* [Provider contract](provider-contract.md) — settings and runtime behavior shared by standard providers.
* [CRS contract](crs.md) — coordinate reference systems, SRIDs, and reprojection.
* [Geometry formats](geometry-formats.md) — WKB, WKT, MOS, and collection behavior.
* [Support and maintenance](maintenance-status.md) — capabilities and operating boundaries.
* [Windows release builds](windows-release.md) — Windows build and packaging workflow.
* [OGC conformance testing](testing/ogc-conformance.md) — pinned suite, stable fixtures and report provenance.
* [Feature performance and observability](testing/feature-performance-observability.md) — dedicated metrics, measured budgets and operational scope.

## Architecture and migration

The implemented application profile is described in the [API reference](api.md)
and [publication configuration](configuration.md). Backend guides describe the
admitted tested profiles and their limits. Runtime conformance declarations use
the admitted registry and all-collection capability intersection; certification
and deployment verification require separate evidence. Architecture pages describe the current code; migration inventories retain their stated historical source scope.

* [Feature-service architecture](architecture/feature-service.md) - existing contracts and integration points.
* [Architecture decisions](architecture/decisions/README.md) - durable design decisions and their applicability.
* [OGC API Features editions](architecture/ogc-api-features.md) - selected normative sources and verification boundary.
* [Jivan route inventory](migration/jivan-feature-matrix.md) - registered resources and internal/deployment behavior.
* [Jivan migration guide](migration/jivan-to-tegola.md) - publication configuration, client changes, verification and rollback.
* [Jivan compatibility matrix](migration/jivan-compatibility-matrix.md) - migration categories and explicit differences.
* [Jivan provenance](migration/jivan-provenance.md) - source influence, reuse inventory and license boundaries.
* [Feature API release scope](release/feature-api.md) - capability scope, verification, cutover and rollback.

## Provider Guides

* [GeoPackage](../provider/gpkg/README.md) — native binary, raw tables, RTree, and bounds columns.
* [PostGIS](../provider/postgis/README.md) — PostGIS and the `mvt_postgis` variant.
* [MySQL / MariaDB](../provider/mysql/README.md) — provider configuration and behavior.
* [SAP HANA](../provider/hana/README.md) — HANA and the `mvt_hana` variant.
* [Third-party modules](../third_party/README.md) — published `geom` and `proj` modules and consumer migration.

## Package References

Tegola is organized as a Go package monolith. The main request path crosses
these packages; the CLI initializes configuration and registrations, then the
server handles requests against the configured atlas:

```text
cmd/tegola → config + provider/cache registration → atlas → server
                                              server → provider(s) → MVT response
```

| Package | Role and where to start |
|---|---|
| `cmd/` | Executable entry points and Cobra commands. Start at [`cmd/tegola/main.go`](../cmd/tegola/main.go), then follow command setup under `cmd/tegola/cmd/`; `cmd/tegola_lambda/` is the Lambda entry point. |
| `config/` | Loads and validates TOML configuration consumed by the command setup. |
| `atlas/` | Holds configured maps, provider layers, cache and observer integrations; it connects request handling to registered runtime components. |
| `server/` | HTTP router, endpoint handlers, middleware, tile cache behavior, and embedded viewer routes. Start at [`server/server.go`](../server/server.go). |
| `feature/`, `ogc/wfs/` | Shared mutation coordination and WFS XML/GML protocol adapters; providers retain native transactions. |
| `ogc/features/`, `ogc/cql2/` | Feature publication/query service and typed CQL2 parsing. |
| `server/lambda/` | Optional buffered Lambda event adapter over the assembled HTTP router. Start at [`handler.go`](../server/lambda/handler.go); supported event modes and transport limits are defined in [ADR-0014](architecture/decisions/ADR-0014-lambda-router-adapter.md). Local invocation is separate from deployed AWS verification. |
| `provider/` | Provider interfaces and standard SQL/spatial backends such as `postgis/`, `gpkg/`, `mysql/`, and `hana/`. Standard providers return features for Tegola to process. |
| `mvtprovider/` | Database-side MVT provider implementations. They return encoded tiles and bypass standard feature geometry processing. |
| `cache/` | Cache interfaces, tile keys, and backend implementations such as `memory/`, `file/`, `redis/`, and `multilevel/`. Backend-specific setup notes are under `cache/<backend>/README.md`. |
| `observability/` | Observer interfaces and integrations, including Prometheus metrics. |
| `basic/`, `maths/`, `mapbox/`, `mos/` | Geometry types and operations, math helpers, MVT encoding, and MOS support used across providers and tile generation. |
| `internal/` | Shared implementation details for this module, including build metadata, environment parsing, logging, and SQL token handling. |
| `ui/` | Source assets for the embedded web viewer. |

For focused package contracts, see [`provider/geometrycodec`](../provider/geometrycodec/doc.go) for shared geometry decoding and MOS configuration, and [`provider/crsconfig`](../provider/crsconfig/doc.go) for CRS resolution and synthetic SRID registration.

## Repository Policies and History

* [Contributing](../CONTRIBUTING.md) — build, test, and contribution process.
* [Security policy](../SECURITY.md) — vulnerability reporting.
* [Changelog](../CHANGELOG.md) — release history.
* [Upstream provenance](../UPSTREAM.md) — fork changes, maintenance, and deferred work.

## See Also

- [API reference](api.md) — start with the HTTP endpoints
- [Configuration](configuration.md) — configure providers and caches
- [Maintenance status](maintenance-status.md) — current project status
