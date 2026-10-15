[← Configuration](configuration.md) · [Back to README](../README.md) · [Provider Contract →](provider-contract.md)

# Development, Debugging, and Builds

This guide covers the CLI, local debugging, and source builds.

## CLI overview

The root command supports `cache`, `help`, `serve`, and `version`. Use `tegola --help` or `tegola <command> --help` for the installed binary's current flags.

```text
tegola is a vector tile server
Usage:
  tegola [command]

Available Commands:
  cache       Manipulate the tile cache
  help        Help about any command
  serve       Use tegola as a tile server
  version     Print the version number of tegola
```

## SQL Debugging

The following environment variables can be used for debugging:

`TEGOLA_SQL_DEBUG` selects the SQL debug information to output. It accepts two values:

- `LAYER_SQL` will print layer SQL as they are parsed from the config file.
- `EXECUTE_SQL` will print SQL that is executed for each tile request, and the number of items it returns or an error.

### Run with SQL debugging

```bash
$ TEGOLA_SQL_DEBUG=LAYER_SQL tegola serve --config=/path/to/conf.toml
```

## Tile latency diagnostics

Standard feature-provider rendering emits `tile layer timing` records with map,
layer, provider layer, Z/X/Y, `elapsed_ms`, `features_received`, and `outcome`.
Time includes fetching and per-feature projection/encoding, but excludes final
tile protobuf serialization and compression. This applies across MySQL, PostGIS,
HANA and GeoPackage feature providers; native MVT providers bypass this path.

These records describe tile rendering, not raw Feature API requests. The
dedicated [feature metrics](testing/feature-performance-observability.md) keep
their separate fixed-label request/provider-query contract.

MySQL also emits `mysql tile query complete` once per query attempt:

| Field | Meaning |
| --- | --- |
| `query_ms` | Pool acquisition plus driver query execution until rows are returned |
| `rows_decode_ms` | Streaming rows, scanning and geometry decoding; includes further network reads |
| `callback_ms` | Processing buffered features through the tile encoder |
| `total_ms`, `attempt` | Whole attempt duration and one-based retry number |
| `failure_phase`, `outcome` | Failing phase and `ok`, `deadline`, `canceled`, or `error` |
| `rows`, `features` | Rows read and decoded features; features can exceed completed callbacks on failure |
| `pool_open`, `pool_in_use`, `pool_idle`, `pool_max_open` | Pool snapshot at completion |
| `pool_wait_count_delta_global`, `pool_wait_ms_delta_global` | Changes in cumulative pool counters across the attempt |

Pool deltas include **all overlapping queries on the same pool**, not just this
request. They indicate contention but cannot isolate an individual query's wait.
Compare records by layer and Z/X/Y; concurrent attempts may cover the same waits.

Errors, deadlines and successful operations lasting at least one second appear
at WARN. Fast success and ordinary cancellation appear at DEBUG (`--log-level DEBUG`).
The new records omit SQL, values, credentials and raw error text. Existing DEBUG
and SQL-debug output can include SQL, so keep complete debug logs private.
Cache HITs do not render layers and produce no new rendering records.

The shared cache-MISS render deadline remains 30 seconds. Startup inspection
timeouts and connection-establishment timeouts are separate. Diagnose a long
`query_ms` alongside pool waits, a long `rows_decode_ms`, or a long `callback_ms`
before changing limits. A completion record becomes available when the provider
returns; a driver that ignores cancellation may delay that record.

## Client-side debugging

A debug layer can show tile outlines and Z/X/Y values. Add `debug=true` to the tile URL template to include it:

```
http://localhost:8080/maps/mymap/{z}/{x}/{y}.vector.pbf?debug=true
```

The requested tile will include a `debug` layer with two features:

- `debug_outline` is a line feature that traces the border of the tile
- `debug_text` is a point feature in the middle of the tile. Its `zxy` tag contains the `Z`, `X`, and `Y` values, formatted as `Z:0, X:0, Y:0`.

## Building from source

Tegola is written in [Go](https://golang.org/) and requires [Go 1.26.9](https://go.dev/dl/) or higher to compile from source.
(CI builds with the Go version pinned in `go.mod`.)
To build tegola from the source, make sure you have Go installed and have cloned the repository.
Build the embedded viewer first when it is required:

```sh
npm --prefix ui ci --ignore-scripts --no-audit --no-fund
npm --prefix ui run build
git restore -- ui/dist/.keep
```

Then build the Go executable:

```bash
go build -mod=vendor -o tegola ./cmd/tegola
```

CGO and a C compiler are required for GeoPackage. See the Windows guide for a complete release workflow.

You will now have a binary named `tegola` in the repository root which is ready to run; follow the [Quick Start](../README.md#quick-start) after preparing a configuration file.

### Build flags

The following build flags can be used to turn off certain features of tegola:

- `noAzblobCache` - turn off the Azure Blob cache backend.
- `noS3Cache` - turn off the AWS S3 cache backend.
- `noRedisCache` - turn off the Redis cache backend.
- `noPostgisProvider` - turn off the PostGIS data provider.
- `noGpkgProvider` — disable the GeoPackage provider. GeoPackage uses CGO and is also disabled when `CGO_ENABLED=0`.
- `noHanaProvider` - turn off the SAP HANA data provider.
- `noMysqlProvider` - turn off the MySQL data provider.
- `noViewer` - turn off the built-in viewer.
- `pprof` - enable [Go profiler](https://golang.org/pkg/net/http/pprof/). Set `TEGOLA_HTTP_PPROF_BIND` to start the profile server (for example, `TEGOLA_HTTP_PPROF_BIND=localhost:6060`).
- `noPrometheusObserver` - turn off support for the Prometheus metrics endpoint.
- `nopgxregisterdefaulttypes` - skip pgx default type registration in the PostGIS provider (rarely needed; used to work around pgx driver conflicts).

Example of using the build flags to turn off the Redis cache backend, the GeoPackage provider and the built-in viewer.

```bash
go build -tags 'noRedisCache noGpkgProvider noViewer'
```

### Setting version information

A plain build uses the historical `v0.21.0-fork.1` fallback from
`internal/build.Version`; it does not identify the current release. Inject
version and full revision metadata for a traceable binary and inspect
`tegola version` afterwards. Use linker flags:

```bash
# first set some env to make it easier to read:
BUILD_PKG=github.com/alexeydott/tegola/internal/build
VERSION="v0.21.0-fork.2+git.$(git rev-parse --short=8 HEAD)"
GIT_BRANCH=$(git branch --no-color --show-current)
GIT_REVISION=$(git rev-parse HEAD)

# build the go binary
go build -mod=vendor -ldflags "-w -X ${BUILD_PKG}.Version=${VERSION} -X ${BUILD_PKG}.GitRevision=${GIT_REVISION} -X ${BUILD_PKG}.GitBranch=${GIT_BRANCH}" ./cmd/tegola
```

## See Also

- [Windows release builds](windows-release.md) — Windows packaging workflow
- [Contributing](../CONTRIBUTING.md) — build, test, and contribution guidance
- [Configuration](configuration.md) — provider and cache setup
