# Tegola

![On push](https://github.com/alexeydott/tegola/actions/workflows/on_pr_push.yml/badge.svg?branch=master)
[![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](https://github.com/go-spatial/tegola/blob/master/LICENSE.md)

> A Go spatial server for Mapbox Vector Tiles, OGC API Features and opt-in WFS transactions.

Tegola is a fork of [go-spatial/tegola](https://github.com/go-spatial/tegola), based on upstream master after v0.21.0. Fork changes and remaining limitations are recorded in [CHANGELOG.md](CHANGELOG.md) and [UPSTREAM.md](UPSTREAM.md).

The source tag `v0.21.0-fork.2` is the 2026-10-02 read-profile release. Current master includes later editing and cache changes; check the binary revision with `tegola version`. See [release scope](docs/release/feature-api.md) and [write support](docs/wfs-scope-limitations.md).

## Quick Start

1. Download the binary for your platform from the [release page](https://github.com/alexeydott/tegola/releases).
2. Create a TOML configuration with a provider and map. See [Configuration](docs/configuration.md) and the [provider contract](docs/provider-contract.md).
3. From the directory containing the downloaded binary, start the server with the configuration file:

   **Linux / macOS**

   ```sh
   ./tegola serve --config=config.toml
   ```

   **Windows PowerShell**

   ```powershell
   .\tegola.exe serve --config=config.toml
   ```

4. Open `http://localhost:8080/` for the embedded viewer, or request `http://localhost:8080/capabilities` to inspect configured maps.

For a source build or Windows release package, see [Development and Builds](docs/development.md) and [Windows release builds](docs/windows-release.md).

## Key Features

- Mapbox Vector Tile v2 output and an embedded viewer with an automatically generated style.
- PostGIS, GeoPackage, MySQL/MariaDB, and SAP HANA spatial data providers.
- Opt-in OGC API Features, Queryables, the documented CQL2 text profile, datetime and public CRS.
- Conditional REST mutations and WFS transactions on explicitly admitted provider layers; embedded attribute editing.
- Raw MOS geometry with separate bounds and custom horizontal CRS for documented MySQL/SQLite profiles.
- Memory, file, multilevel, GCS, S3, Redis, and Azure Blob cache backends.
- Geometry processing, CRS reprojection, cache seeding, and cache invalidation.
- HTTP tile operations, AWS Lambda support, HTTPS, and Prometheus observability.

## Example

A configured map named `roads` can be requested as a vector tile:

```text
http://localhost:8080/maps/roads/12/1234/567
```

See the [API reference](docs/api.md) for endpoints and tile-cache operations.

## Documentation

| Guide | Description |
|---|---|
| [Documentation index](docs/README.md) | All project guides and package references |
| [API reference](docs/api.md) | HTTP endpoints and tile operations |
| [Feature API release](docs/release/feature-api.md) | Historical tagged read profile and later source changes |
| [Editing and WFS](docs/wfs-scope-limitations.md) | Write admission, clients and protocol limitations |
| [Provider support](docs/provider-matrix.md) | Tested native profiles and verification boundaries |
| [Jivan migration](docs/migration/jivan-to-tegola.md) | Move legacy feature clients to Tegola |
| [Configuration](docs/configuration.md) | TOML, providers, caches, and environment variables |
| [Development and builds](docs/development.md) | Debugging, build flags, and source builds |
| [Provider contract](docs/provider-contract.md) | Shared provider configuration and behavior |
| [CRS contract](docs/crs.md) | Coordinate reference system configuration |
| [Geometry formats](docs/geometry-formats.md) | WKB, WKT, MOS, and geometry behavior |
| [Support and maintenance](docs/maintenance-status.md) | Current capabilities and operating boundaries |
| [Windows release](docs/windows-release.md) | Windows release build process |

## Contributing and Support

- [Contributing](CONTRIBUTING.md) — build, tests, and pull request process.
- [Security policy](SECURITY.md) — report a vulnerability.
- [Go module consumers](third_party/README.md) — import paths and dependency migration.
- Looking for a vector tile style editor? Try [fresco](https://github.com/go-spatial/fresco).

## License

MIT. See [LICENSE.md](LICENSE.md).
