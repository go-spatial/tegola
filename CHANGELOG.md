## Unreleased

- Normalize WFS Update/Delete `typeNames` before validating feature IDs and property references, retaining namespace validation and rejecting conflicting type attributes.
- Require Go 1.26.9 and update `golang.org/x/net` to v0.60.0 to include the October 2026 HTTP and template security fixes.

- Cache ordinary viewing in writable runtimes; bypass ordinary tile caching with `X-Tegola-Editor-Active: true`. Isolate cache generations after successful or uncertain commits and across restarts, including concurrent renders and metatile maintenance.
- Allow authenticated explicit tile-cache status and regeneration in writable feature/WFS runtimes with HTTP `no-store`.

- Align WFS nullable geometry input with DescribeFeatureType: explicit `xsi:nil` in Insert, Update and Replace writes SQL NULL only for nullable geometry, preserving omitted-update geometry and rejecting malformed nil values.

- Support explicit `xsi:nil` scalar NULL values in WFS Insert, Update and Replace, enforcing storage nullability and rejecting nil elements with content. Omitted Update `Value` remains unsupported; empty string values retain their existing meaning. See [WFS write limits](docs/wfs-scope-limitations.md).

- Add transactional raw MOS geometry/bounds maintenance for admitted MySQL/InnoDB and GeoPackage-provider SQLite tables, plus immutable custom `etmerc` horizontal transforms with explicit three/seven-parameter datum shifts. MySQL 5.5 can opt into metadata-based table identity with documented external-DDL limitations; fresh transaction-recorder schemas use explicit Unicode-safe text storage. See [MOS write contracts](docs/geometry-formats.md#mos-writes-with-separate-bounds-columns) and [separate runtime evidence](docs/provider-matrix.md#mos-bounds-and-custom-crs-validation). PostGIS bounds writes and custom vertical CRS remain outside this extension.

- Resolve FES literals against Queryables, validate conditional mutations against the selected output CRS/format, and preserve WFS commit status/correlation headers without exposing private receipts. Deadlock failures cannot report a later successful commit. See the [provider evidence](docs/provider-matrix.md) for native and injected-fault boundaries.

- Tighten protocol and write admission: reject production writes in the stock CLI without an authenticator, preserve storage nullability and validate RFC3339 inputs, normalize write operation names in OpenAPI, admit create only with generated keys, prepare MySQL service schema before write publication, and retain transaction correlation IDs. WFS resolves advertised qualified names, preserves negotiated versions and mounted public URLs, and no longer advertises unsupported XML BBOX filters. See [write contracts](docs/wfs-scope-limitations.md) and [provider evidence](docs/provider-matrix.md) for tested profiles.
- Verify raw MOS mutations on MySQL 8.4.11 and the GeoPackage provider's SQLite BLOB profile, including six geometry families, holes, quantization, annotated-source preservation and CRS-aware readback. Add separate bounds-backed MOS read-only checks and a GeoPackage-provider HTTP MOS roundtrip. Derived bbox-column writes, MariaDB/HANA MOS writers and untested precision/CRS profiles are not implied by these results.

- Add opt-in WFS read/transaction adapters and a draft-derived Feature API write profile for admitted PostGIS, MySQL/MariaDB and GeoPackage layers. Writes use schema validation, transactional revisions and conditional requests. LockFeature remains unavailable; writable runtimes invalidate router-local tile cache generations after commits. No full WFS/Part 4 conformance or unrestricted production acceptance is claimed. See [write scope and operating limits](docs/wfs-scope-limitations.md).
- Add a source-feature attribute editor with mutation-schema forms, conditional JSON Patch, local undo/redo and preserved conflict drafts. Existing geometry is retained; unsafe numeric editing and unknown write outcomes stop further writes. Geometry drawing and lossless large-number editing require external clients.

- Fix OGC Features handling for GeoPackage Queryables, polar bbox predicates and limit errors; preserve exact integer/REAL filter comparisons and reject non-finite scalar values.

- Accept MySQL identifier aliases with or without `AS` in direct startup geometry samples, avoiding unnecessary derived-table materialization. Preserve the three-valid-MOS early stop and the 16-row inspection window.

## v0.21.0-fork.2 — 2026-10-02

Source tag: `db4e8ee73a3ddfe3b3c8e054c59e833b697dce85`.

- Include the implemented OGC Features service, Queryables/CQL2, CRS and Jivan migration guides.
- Integrate MOS metadata probes and per-layer/MySQL latency diagnostics with the OGC provider contracts.
- Build Windows binaries with CGO/GeoPackage, the embedded viewer and the exact release tag.

The detailed fork changes below retain their original scope. See
[Feature API release notes](docs/release/feature-api.md) for acceptance boundaries.

Fork-specific changes on top of upstream master (post-v0.21.0, 2024-12-19). Fork releases follow the version scheme `v0.21.0-fork.N`; the first fork release is `v0.21.0-fork.1`.

Features

* Add opt-in OGC API Features publication through the existing router and standard providers, with JSON/HTML, bbox/datetime, link-driven paging, Queryables, the documented CQL2 text subset and supported public CRS. Runtime capability declarations and scoped conformance evidence do not imply certification. See [Feature API release notes](docs/release/feature-api.md).
* Provide a Jivan replacement migration guide, compatibility dispositions and source provenance. The optional Lambda adapter shares the HTTP router across buffered REST v1, HTTP v2, Function URL v2 and ALB multi-value events. Public release and deployed AWS acceptance remain separate actions.
* Add per-layer tile timing and MySQL query-phase/pool-contention diagnostics. Slow operations and deadlines are logged at WARN, fast successes at DEBUG; render deadlines and pool ownership remain unchanged. See [latency diagnostics](docs/development.md#tile-latency-diagnostics).
* Align tile HTTP cache headers for HEAD and GET requests; clean `curl -I` probes now report the same freshness policy as browser tile downloads.
* Add opt-in `webserver.tile_http_max_age` for successful anonymous query-free tile responses, consistently across HIT/MISS/shared rendering. Preserve encoding variation and maintenance `no-store`; exclude errors and parameterized requests. Default zero preserves previous behavior.
* Unified CRS contract for all standard providers (`mysql`, `gpkg`, `postgis`, `hana`): `srid` and `crs_defn` (full PROJ.4 definition) at provider and layer level, explicit configuration wins over source auto-detection. Tile bounds (`!BBOX!`) are reprojected into the layer CRS so filter, data and MVT encoding always agree on one CRS.
* Built-in EPSG registry (`basic/epsg`) and synthetic internal SRIDs (>= 340000001) for custom PROJ.4 definitions and MOS system-info projections.
* Shared geometry formats for all standard providers: `wkb`, `wkt` and the packed `mos` payload, with `mos_precision` / `mos_units` configuration and system-info auto-configuration (`MapplGIS LayerInfo`).
* GPKG provider reworked: native GeoPackage binary, WKB/WKT/MOS geometry formats, `gpkg_contents.srs_id` detection, RTree `!BBOX!` filtering for table layers and in-memory filtering (with optional bounds columns) for raw tables without GeoPackage metadata.
* MySQL / MariaDB provider with `auto` / `mysql` / `mariadb` geometry formats.
* HANA MVT provider (`mvt_hana`) alongside `mvt_postgis`.
* Memory and multilevel tile caches.
* Tile endpoint modes `?tile=status|update|getupdated` and cache invalidation via `?dirty`.
* Providers can be excluded from the binary at build time with the `noMysqlProvider`, `noGpkgProvider`, `noPostgisProvider` and `noHanaProvider` build flags (e.g. `go build -tags 'noMysqlProvider'`); `tegola version` reports the active set.
* Async metatile regeneration. The `?tile=update` and `?tile=getupdated` cache-maintenance endpoints no longer regenerate the full 8x8 metatile (up to 64 tile renders and cache writes) on the request path. `?tile=update` now returns `202 Accepted` and schedules the regeneration on a bounded background worker pool (previously `204` after regenerating everything synchronously); `?tile=getupdated` returns the requested tile after a single render and schedules the same background regeneration. Regeneration is single-flight per metatile — concurrent requests join the regeneration already in flight instead of starting another one — and background failures are logged at WARN and retried on the next request instead of being cached. Graceful shutdown cancels and drains in-flight regenerations before the cache is torn down. No configuration changes.

Breaking changes

* MySQL provider: the `proj4` config key was removed in favour of `srid` / `crs_defn`.
* PostGIS provider: `tablename` and `sql` are strictly mutually exclusive (startup error).
* PostGIS provider: `pool_*` connection-pool keys were introduced/renamed; review your `[[providers]]` blocks.
* **Redis cache: URI-only configuration.** The `redis` cache connection is now configured exclusively by the `uri` key (e.g. `uri = "redis://:secret@127.0.0.1:6379/3"`; schemes `redis://`, `rediss://`, `unix://`; DB via path or `?db=`, tuning via query params such as `?pool_size=2&dial_timeout=3s`). The legacy `network`, `address`, `password`, `db` and `ssl` keys are no longer accepted: configs that set them — or that lack a valid `uri` — now fail at startup with an error showing the exact before→after migration. Pool size and dial timeout fall back to go-redis defaults (10×GOMAXPROCS / 5s) instead of the previous 2 / 3s.
* `provider/postgis`, `provider/hana`: an MVT query naming a layer that is not registered with the provider now fails with a clear request error (`ErrLayerNotFound`, before any SQL) instead of logging a warning and continuing with a zero-valued layer (upstream behaviour). Custom SQL whose SELECT list omits the bounds columns still registers with a WARN only — that bounds-contract gate is unchanged.
* `provider/gpkg`: raw-table GeoPackage layers whose geometry column has no `rtree_<table>_<column>` shadow table now register with a WARN and a fallback tile-query plan (plain-bbox on the table's bounds columns, or full-table-scan + in-memory exact filtering when there are none) instead of failing registration. The plan is chosen once at registration via a `sqlite_master` probe; tile requests keep exactly one SQL round trip. Add the index with `SELECT CreateRTreeIndex(<table>,<geom>)` to restore the RTree plan.

Maintenance

* Go module dependencies upgraded (`go.mod` / `vendor` refresh): `redis/go-redis` v9.22.0, `SAP/go-hdb` v1.18.11, `gdey/tbltest` bumped to its final upstream revision, and the unused `arolek/p` dependency dropped.
* MVT protobuf code generation migrated from gogo to `google.golang.org/protobuf` (`protoc-gen-go` v1.36.12); the wire format is unchanged and the fork no longer depends on `github.com/golang/protobuf`.
* Built-in viewer: `maplibre-gl` upgraded to v5, the deprecated `babel-eslint` replaced with `@babel/eslint-parser`, and npm install scripts denied by default (`ui/.npmrc` supply-chain hardening).
* Logging consolidation on `log/slog` — all tegola logging now flows through a single stdlib `log/slog` backend. The `internal/log` facade keeps its API but is reimplemented on a `slog` TextHandler (output format changed from JSON to greppable single-line text with `level=WARN`/`level=ERROR`), with a new `Logger() *slog.Logger` accessor and nil-writer-safe `NewLoggerTo`. Ad-hoc `log.Printf`/`fmt.Print` diagnostics across cache, basic, maths, container, draw, `cmd/tegola` and server were routed through the facade: verbose breadcrumbs/timings at DEBUG (hidden at default level), anomalies and pre-crash diagnostics at WARN. No new dependencies; message texts preserved verbatim.
* SQL token substitution (`!BBOX!` et al.) is now SQL-context-aware across all providers (`gpkg`, `mysql`, `postgis`, `hana`) via a shared internal scanner (`internal/sqltoken`): tokens inside string literals, quoted identifiers, comments, and PostgreSQL dollar-quoted strings are left verbatim instead of being substituted or uppercased. Token detection follows the same rule — a `!BBOX!` mention in a comment or literal no longer satisfies custom-SQL requirements nor triggers the raw-format token rejection. Rendering, parameter replacement, validation and probes now select backend-specific lexical rules: PostgreSQL arrays and hash operators remain executable, while PostgreSQL escape strings, SQLite backslashes and MySQL comment syntax are handled separately. Legacy helper APIs retain their previous behavior; provider README files document supported SQL modes.
* Consolidate provider test setup into a shared `provider/test/fixture` harness for SQL-row fixtures, database/schema lifecycle, configuration assembly, and explicit tile bounds. Migrate GPKG, HANA, MySQL, and PostGIS tests, remove duplicate setup implementations, and define a request-aware MVT mock callback while preserving canned responses. Add harness unit tests and usage documentation. Test infrastructure only; production behavior is unchanged.
* Restore the standalone geom fork's dependency checksums and protobuf-compatible Go minimum; enforce read-only nested-module checks in CI. Document and smoke-test checkout-based downstream consumption, guard the local-replacement inventory, and verify independently vendored consumers build offline. The later module migration below completes remote-only consumption with published fork dependencies.

Module migration

* The Go module is now `github.com/alexeydott/tegola`, with direct dependencies on published `github.com/alexeydott/geom v0.1.1` and `github.com/alexeydott/proj v0.3.1`. Local dependency replacements are removed. Go consumers must update imports, including public geom types; server configuration and HTTP endpoints retain their existing contracts. See `third_party/README.md` for migration and remote-consumer verification.

Bugs

* Serve layer URLs from a seeded whole-map MVT on layer-cache misses, avoiding redundant provider queries and geometry rendering. Preserve individual-layer cache precedence and source TTL. File-cache startup now resolves and logs the absolute cache root.
* Automatic MOS detection accepts a completed short SQL result with one valid geometry, even when other rows are discarded. Automatically detected MOS layers skip undecodable rows during class inference and tile rendering, preserving valid features.
* Add optional MOS custom-SQL layer `bbox_table` to qualify bounds predicates in joins across all SQL providers without changing result column names or introducing derived tables.
* Avoid materializing a startup sample for explicit MOS metadata checks in all SQL providers. MySQL uses a direct `LIMIT 0` for supported SELECT forms, preventing registration timeouts with expensive views on MySQL 5.5. Geometry-class and automatic-format sampling transfer only geometry cells: proven simple identifier projections use a direct bounded SELECT, while complex forms retain a conservative derived fallback that may materialize. Streamed registration samples are checked against a fixed 64 MiB geometry-cell cap before decoding; custom-SQL sampling retrieves only the cap plus one byte to detect oversize values.
* Explicit MOS custom SQL now checks metadata without sampling or unpacking geometries for format detection. With an explicit geometry type, startup performs no format/class decoding. Automatic MOS detection stops after three valid geometries within its 16-row search window.
* Reject synthetic SRID hash collisions without rebinding existing layers or cached transformations; explicit registrations cannot overwrite an ID owned by another `crs_defn`. Cover registration order and stored layer IDs with regression tests.
* In-process simplification is enabled by default below its configured maximum zoom again. `ZEpislon()` now converts the tolerance from MVT coordinate units into WebMercator meters before simplification. The default is 10 units in the 4096-unit MVT extent, not 10 screen pixels. Layer/global opt-outs remain available. Polygons with holes and multi-component MultiPolygons now simplify when bounded validation confirms simple rings, unchanged winding/containment and disjoint filled interiors. Unsafe or inconclusive candidates preserve the original geometry, including existing boundary contacts.
* All four SQL providers now share CRS-aware pixel-width, pixel-height and scale-denominator calculations. PostGIS, MySQL and GeoPackage use source-CRS units rather than assuming WebMercator; custom tile pixel dimensions are respected. Geographic CRSs now use the source ellipsoid's local parallel-arc factor `N(phi) * cos(phi) * pi / 180` at the transformed tile center instead of the WGS84-radius spherical approximation, including geographic `crs_defn` with `+towgs84`. The horizontal denominator retains the OGC 0.00028 m pixel; geographic pixel dimensions remain in degrees. Non-WebMercator SQL thresholds may need adjustment.
* Fix geographic datum transformations in the Go `proj` fork: `longlat` and its aliases now apply the existing three-/seven-parameter `datumToWGS84` / `datumFromWGS84` math for datums in the fork's table and ellipsoid definitions with `+towgs84`, including custom ellipsoid parameters and synthetic `crs_defn` SRIDs. WGS84 identity remains supported. The two-dimensional API assumes zero input height in each direction. Unsupported grids, prime meridians, units and axes remain explicit registration errors; see the CRS contract for details.
* Tile status reports queued background metatile regeneration as `updating: true`, as well as work that has already started.
* Correct the MySQL point test fixture to standard 21-byte WKB and assert the decoded coordinates.
* Fix PostGIS registration probes for native geometry `!BBOX!` operands, compact zoom comparisons and trailing SQL line comments; skip NULL geometry samples during type discovery. Deferred MySQL queries with scale tokens require a resolved or explicitly configured source CRS instead of using provisional units.

* Custom SQL structural validation no longer requires the bounds columns in the SELECT result: they are validated only when they are part of the SQL result (their actual names are persisted for the bounds predicate), and a layer whose SELECT list omits them is registered with a warning under the resolved names (`layer > provider > MINX/MAXX/MINY/MAXY`) — the `!BBOX!` predicate resolves the bounds columns in the query's own scope. A missing `!BBOX!`/`!BOX!` token or a missing configured geometry column remain startup errors. Applies to all providers (`mysql`, `postgis`, `hana`, `gpkg`).
* MySQL provider: the connection DSN is now built from `mysqlDriver.NewConfig()`, preserving the go-sql-driver authentication defaults. Previously a bare config struct literal disabled `allowNativePasswords`, so accounts using `mysql_native_password` failed to connect with "this user requires mysql native password authentication".
* MySQL provider: `BIGINT UNSIGNED` (`uint64`) and `FLOAT` (`float32`) column values arriving from the driver's binary protocol hit the tag type switch's error fallback, so the affected tags were silently dropped from the tile while an `unexpected type for mysql column data` error was logged per row. Both types are now encoded into the MVT (`uint64` via the native `uint_value`, `float32` widened through its shortest decimal form so the tag matches the text protocol's value).
* Fixed line simplification producing self-intersecting geometries (needles/flipped spikes) on non-`mvt_postgis` providers. The Douglas-Peucker simplifier now uses true segment distances and topology validation (with a conservative fallback to the unsimplified input), ring normalization no longer drops spike vertices, and simplification no longer pre-truncates coordinates to integers. Simplified MVT tile bytes will change for `gpkg`, `mysql`, `hana`, `postgis` without server-side simplification, and generic MVT encoding; `mvt_postgis` is unaffected.
* HANA: compute pixel-width/height SQL tokens in the resolved layer CRS and scale denominators with CRS-aware unit conversion and the OGC 0.28 mm pixel. Support projected meters, feet/custom linear units, and geographic degrees converted using the source ellipsoid and center latitude; reject unsupported CRS/unit data explicitly. Preserve default WebMercator SQL values and context-safe substitution. Non-WebMercator SQL scale thresholds may need adjustment.
## 0.17.0 (2023-07-27)

Features

* [Adds configurable query parameters to tile endpoints](https://github.com/go-spatial/tegola/pull/867) (@bemyak)
* [feat: add Google Cloud Storage cache adapter](https://github.com/go-spatial/tegola/pull/891) (@matheusmatos)
* [HANA database provider. With this update, Tegola can use the HANA database as a backend.](https://github.com/go-spatial/tegola/pull/893)(@mrylov) 

Enhancements

* [MapLibre - Vue3 - Vite Migration v2](https://github.com/go-spatial/tegola/pull/926) (@mapl)

Maintenance
* [fix: viewer and update dependencies](https://github.com/go-spatial/tegola/pull/916) (@iwpnd)

Bugs

* [Updated built in viewer to resolve some security issues -- #914 ](https://github.com/go-spatial/tegola/pull/916) (@iwpnd)
* [Prometheus will not panic now if one has both MVT and Postgres Providers -- Fix for #886](https://github.com/go-spatial/tegola/pull/915) (@iwpnd)

## 0.16.0 (2022-12-01)

Features

* Added option to use Zap logging to get JSON based logs (@iwpnd)

Enhancements

* Add ttools helper for easier local testing. (@iwpnd)
* (UI) Upgraded eventsource from 1.1.0 to 1.1.1 (dependbot)
* (UI) Upgraded shell-quote from 1.7.2 to 1.7.3 (dependbot)
* (UI) Upgraded terser from 4.8.0 to 4.8.1 (dependbot)
* Fix for #870 filter by zoom level for min/max zoom 0 (@iwpnd)
* setting max_zoom to 0 will set it to 1 (@iwpnd)
* seeding command now as a --log-threshold value to control logging of tiles that take longer than the given time. (@dwoznicki)
* handle gpkg GEOMETRY as an unknown geometry and not break the capabilities if a layer contains such a geometry. (@roelarents)
* [test: use T.Setenv to set env vars in tests](https://github.com/go-spatial/tegola/pull/882) (@Juneezee)

Documentation

* Updated README to focus on `mvt_postgis` provider instead of `postgis`

Bugs

* Minor code clean up (@dechristopher, @bemyak)

## 0.15.0 (2022-05-18)

Features

* Redis SSL connection via redis.ParseURL (#815 @iwpnd) 
* Updated providers/postgis to use pgx4. This enables use of Postgres versions 12+ (@iwpnd #820)
* providers/postgis: allow connection URI and add additional config parameters (@iwpnd #841)
* expose SetLogLevel in cli (@iwpnd part of #831)
* Add a dont_clean option (@roelarents  #847)

Enhancements

* ci: publish edge image on push or pr to default_branch_ref (@iwpnd) 
* chore: switch to internal/log (@iwpnd #837)
* chore: remove logAndError (@iwpnd #839)
* chore: add docker-compose local dev env (@iwpnd #840)
* removed go-bindata for embedding the internal viewer in favor of the native go embed (@ARolek #843)
* fix: replace environment variables in webserver headers (@iwpnd #597, #844)

Documentation

* Update S3 cache properties documentation around `force_path_style` (@flowrean #835)

## 0.14.0 (2021-11-05)

Features:

* Added Observability to postgis data provider (@gdey #780)
* Added observability to tile cache (@gdey #766)
* Allow link in attribution (@underspica #798)

Bugs:

* Fixed file cache: `Error: error reading from cache: too many open files` (@ARolek #586)

Maintenance: 

* Refactor tegola_lambda CI to use Amazon Linux for building (@arolek #790)
* Upgrade mattn/go-sqlite3 (@gdey)
* Split out httpAPI and httpViewer instrumentation  (@gdey)
* Refactored BuildInfo into a package (@gdey)
* Update TOML package (@ear7h #799)

## 0.13.0 (2021-03-23)
**Features**

* cache/s3: Allow forcing path style for s3 compatible API requests (#745 @johngian)
* observability: [experimental] Initial implementation of observability support for Prometheus. Provides go runtime metrics and a "/metrics". (# #714 @gdey )

**Bug Fixes**

* Fixed MacOS release builds don't include support for GeoPackage providers (#736 @flother )
* ci: Fixed GH Action env syntax (@ARolek)
* docker: Version bumps in Dockerfile (@ARolek)
* dataprovider/postgis/mvt: Fix SQL parsing for MVT provider (#744 @johngian)

**Maintenance**

* cache/s3: updated deprecated function (@alrs)
* dataprovider/postgis: Updated mvt_postgis README regarding 4326 projection (@ARolek)
* ui: Bumps elliptic from 6.5.3 to 6.5.4. dependency (#749 @dependabot)

## 0.12.1 (2020-09-04)
**Bug Fixes**
* fixed the internal viewer not using the most recent version (@ARolek)

## 0.12.0 (2020-08-26)
**Features**
* proj: implemented the go-spatial/proj package (@meilinger)
* mvt_providers: brought in mvt_postgis enabling support for ST_AsMVT from postgis. (#556 @gdey)
* viewer: the maps layer list can now be hidden (@mapl)

**Bug Fixes**
* cache seeding: handle map zooms with no layers (#698 @ARolek)  
* provider/postgis: fix dropped errors (@alrs)
* server: fix dropped test error (@alrs)
* providers/postgis: When ignoring UnknownGeometryType log the issue (@gdey).

**Maintenance**
* mvt: Split out Transformation, Simplification, and Clipping from encodeGeometry (#224 @ARolek).
* mvt: remove local mvt package and vendor from geom (@ear7h)
* basic: remove tegola.Geometry type from basic.ToWebMercator (#622 @ear7h)
* basic: normalized table tests to style of table tests (@gdey)

## 0.11.0 (2020-05-04)
**Features**
* Added SSL Support (#82 @ear7h)
* lambda: global state for database connection caching (#609 @ARolek) 

**Bug Fixes**
* server: return tiles that lie on a Map's boundary (#633 @ear7h)
* postgis: use `id` table field as a tile tag (#383 @ear7h)

**Maintenance**
* atlas: remove usage of tegola.Tile (#636 @ear7h)
* atlas: use geom package simplify function (@ear7h)

## 0.10.2 (2019-08-30)
**Features**
* added tegola_lambda_cgo to the build pipeline. Geopackage can now be used with tegola_lambda (@ARolek)

## 0.10.1 (2019-09-03)
**Bug Fixes**
* server: fixed cache middleware removing incorrect path prefix when a `uri_prefix` was set. (@ARolek)
* server: fixed internal viewer URIs when a `uri_prefix` was set. (@ARolek)
* server: fixed internal viewer file paths when a `uri_prefix` was set. (@ARolek #361)

## 0.10.0 (2019-08-30)
**Features**
* cache/redis: add configurable key expiration time. #600 (@tierpod)
* server: configurable webserver URI Prefix #136 (@ARolek)

**Bug Fixes**
* server: Content-Length for non-gzipped responses (@thomersch)
* cmd: tegola command fails when running version subcommand without a config. #626 (@ear7h)

**Maintenance**
* Upgraded internal viewer Mapbox GL JS to 1.0.0 (@ARolek)
* Skip S3 tests on external pull requests (@gdey)

**Breaking Changes**
* If a `webserver.hostname` is set in the config the port is no longer added to the hostname. When setting the `hostname` it's now assumed the user wants full control of `hostname:port` combo.

## 0.9.0 (2019-04-09)
**Features**
* Add support for --no-cache command line flag override (#517 @tierpod)
* Add support for per map tile buffers (#501 @tierpod)
* Added map config option dont_clip to turn off layer clipping (#562 @paumas)

**Bug Fixes**
* Removed superfluous `sort.Int` to improve geoprocessing performance (#567 @vahid-sohrabloo)
* provider/postgis: no error thrown when geoFieldname is missing (#590 @ARolek)
* server: User defined http response headers are not added to OPTIONS requests (#594 @ARolek)
* atlas: Handle empty geometry collections (#429 @paumas, @ARolek)

**Maintenance**
* Update sqlite driver. Driver understands strings now. (@gdey)
* Updated CI to use Xenil (@gdey)

## 0.8.1 (2018-11-07)
**Bug Fixes**
- fixed double seeding when using tegola cache seed tile-list with the --map flag (#553 @arolek)

## 0.8.0 (2018-11-01)
**Features**
- provider/gpkg: use aliases and quotes in query for all column names (#486 @olt)
- provider/gpkg: improve column name extraction (#486 @olt)
- provider/postgis: added support for `!pixel_width!`, `!pixel_height!` and `!scale_denominator!` SQL tokens (#477 @olt)
- provider/postgis: Support for sub-queries as "tablename" (#467 @olt)
- provider/postgis: Set default_transaction_read_only when connecting to PostgreSQL (#369)
- cache/s3: added default mime-type of `application/vnd.mapbox-vector-tile` and made it configurable (#459 @stvno)
- mvt: don't require IDs for mvt features. (#337, #338 @ARolek)
- server: gzip encoding of tiles (#438 @ARolek)
- server: set proper MIME type for vector tiles (#511 @ARolek)
- server: configurable response headers (#519 @tierpod, @ARolek)
- server: Improve display of tile rendering times (#484 @ARolek)
- docker: Add CA certificates to Docker container. Refactored container and pipeline (#385 @gdey, @stvno, @ARolek)
- cmd: added support for `TEGOLA_PPROF_MUTEX_RATE` and `TEGOLA_PPROF_BLOCK_RATE` env vars when `pprof` is enabled. (@gdey)

**Bug Fixes**
- provider/gpkg: fixed index query for geomFieldname != geom (#486 @olt)
- provider/postgis: fixed error not thrown if the database user does not have permissions to access table (#538 @ARolek)
- cache seeding: invalid value for bounds () with 10e3 notation (#539 @gdey)
- fixed the way cache seed / purge with a tile-list or tile-name works. min and max zooms now must be provided for tegola to include parent and child tiles (@gdey)

**Breaking Changes**
- **IMPORTANT**: if you have a current tile cache in place, you will need to purge it entirely as tegola now expects the cache to persist gzipped tiles.
- `cors_allowed_origin` is no longer supported under the `webserver` config section. The same functionality can be implemented using the `[webserver.headers]` config which allows for configuring almost any response header. Use `Access-Control-Allow-Origin = "yourdomain.com"` in the config moving forward.
- Docker container now uses the `ENTRYPOINT` command. Users of the Docker container will need to update the commands they're passing to the container.

## 0.7.0 (2018-08-10)

- `Documentation`: Typo, grammar, clarity fixes. (#345 @erictheise)
- `inspector` : Sort property names in feature inspector (#367 @erictheise)
- `geom/encoding/wkb/` : Added Fuzzing framework for wkb (#53 @chebizarro)
- `server/`: Fix nil pointer dereference when using implicit zooms. (#387  @ear7h)
- `server/`: For MinZoom and MaxZoom default to appropriate values. (#354 @ear7h) 
- `server/`: For invalid x,y values we should return a non-200 response code. (#334 @ear7h)
- `server/`: Empty layer should return 404 (#375)
- `server/`: Bunch of new build tags for leaving out features. (#397)
- `server/` : Enviromental Var subtitution [breaking change] (#353 @ear7h)
- `server/`: Fixed TileJSON minZoom and maxZoom values when two layers with same zoom exists. (@paumas)
- `maths/makevalid`: Simplify, optimize unique. (#344 @paulmach)
- `cache/s3`: Add configurable ACL and Endpoint support for S3 cache to allow for S3 compliant stores outside of AWS (i.e. [minio](https://www.minio.io/features.html)). (#413 @stvno)
- `cache/s3`: Add configurable Cache-Control headers. (#448 @stvno)
- `cmd/tegola_lambda`: Support for running tegola on AWS Lambda. (#388) Instructions can be found in the README.md in the package.
- `cache/file`: On Windows files are written with invalid names (#422 @TNT0305)
- `providers/postgis`: Support for SSL (#426 @nickelbob)
- `providers/postgis`: Fixed panic when encountering 3D geometries (#89)
- `providers/postgis`: Gracefully handle NULL geometries (#429)
- `providers/postgis`: Add support for !bbox! (Mapnik) and !BOX! (MapServer) tokens (#443 @olt)
- `providers/postgis`: Add `geometry_type` option to avoid table inspection (#466 @olt)
- `providers/postgis`: Added `TEGOLA_` prefix to `SQL_DEBUG` (#489)
- `cache/azblob`: Support for Azure blob store as a cache backend (#425)
- Enable Go pprof profiler with `TEGOLA_HTTP_PPROF` environment (@olt)

## 0.6.0 (2018-02-26)

- `provider/postgis`: Added: connection parameterization for tegola unit-test suite (#221)
- `provider/postgis`: Fixed: Using !ZOOM! token can cause nil geom type on style generation (#232)
- `provider/postgis`: Refactor postgis provider to use provider.Tiler interface (#265)
- `provider/gpkg`: Add GeoPackage as a Provider (#161)
- `wkb`: Fixed: WKT for collection doesn't do much (#227, @remster)
- `server`: Fixed: A GET request for a Tile with a negative row value is successful (#229)
- `server`: Fixed: Tile request returns 200 when using invalid map (#250)
- `internal/log`: Added: Logger outputs file:line of log/standard.go along with timestamp in output. (#231)
- `tegola`: Fixed / Added: Configurable tile buffer (#107)
- `config`: Added: Support environment variables in config file (#210)
- `config`: Added: Support for turning off simplification per layer (#165)
- `server`: Added: Configurable CORS header (#28)
- `server`: Fixed: Tile cache middleware not receiving response code 200 (#263)
- `server`: Fixed: /maps/:map/:layer/:z/:x/:y not filtering to correct layer (#252)
- `server`: Fixed: style generator handling of nil geoms (#302)
- `server`: Removed configurable request logger in server package (#255)
- `server`: Added: Configurable layer simplification (#165)
- `mvt/feature`: Fixed: 2 pt lines are being disregarded (#280)
- `maths/clip/`: Fixed: Line clipping panics when linestring has 0 points (#290)
- `cache/file`: Fixed: Caching at higher levels than specified by maxZoom (#311)
- `cache/s3`: Fixed: Caching at higher levels than specified by maxZoom (#311)
- `cache/redis`: Added: Redis cache support (#300 - @ear7h)
- `encoding/geojson`: Added: geojson data types and encoding. (#288)
- Write Dockerfile to build tegola & create minimal deployment images (#244)
- Wire docker image build into CI (#245)
- Fixed: clipping & simplification bugs (#282)
- `Documentation`: Document the layer name property in the example config (#333 @pnorman)

**Additional Notes**
- tegola now has a public docker image which can be found at https://hub.docker.com/r/gospatial/tegola/. 

## 0.5.0 (2017-12-12)

- Added: Command line `cache seed` and `cache purge` commands (#64)
- Added: Support for Amazon S3 as a cache backend (#64)
- Added: More robust command line interface (#64)
- Added: No-Cache headers to `/capabilities`, `/capabilities/:map_name` and `/maps/:map_name/style.json` endpoints. (#176)
- Fixed: Possible Panic if a feature without an ID is added before a feature with an ID; when constructing Layers (#195)

Breaking changes:
- To use tegola as a web server, use the command `tegola serve --config=/path/to/config.toml`

## 0.4.2 (2017-11-28)

- Fixed: Performance affected by unused log statements (#197, @remster)

## 0.4.1 (2017-11-21)

- Fixed: regression in providers/postgis. EXECUTE_SQL environment debug was dropped.
- Fixed: Filecache: concurrent map read and map write on Set() (#188)
- Fixed: Filecache: invalid fileKey on cache init (Windows) (#178)
- Fixed: Clean up context canceled log (#170)

## 0.4.0 (2017-11-11)

- Fixed: configurable max_connections param for PostGIS provider
- Fixed: 504 returned when attempting to retrieve a tile at negative zoom (#163)
- Fixed: Using WGS84 yields squishes tiles along Y-axis (#156)
- Fixed: Capabilities endpoints not returning zoom range for all layers with the same name (#153)
- Fixed: Default config.toml not found in (#157)
- Fixed: Config validation fails when layers are overlapping but in different map configs (#158)
- Fixed: PostGIS: hstore tags should not override column tags (#154)
- Added: Filesystem cache (#96)
- Added: Clipping & Make Valid (whew!) (#56)

## v0.4.0-beta (2017-10-09)

- Fixed: Panic when PostGIS tries to query a layer that does not exist (#78)
- Fixed: Viewer not indicating colors correctly for polygons (#146)
- Fixed: stacked scrollbars showing in the embedded viewer (#148)
- Fixed: Invalid tilejson scheme (#149)
- Added: Support for X-Forwarded-Proto (#135, @mojodna)
- Added: Support for user defined layer names (#94)
- Updated: MVTProvider interface to return LayerInfo (#131)

## v0.4.0-alpha (2017-08-21)

- Added: hstore support for PostGIS driver. (#71)
- Added: experimental clipping support. (#56). To enable set the environment variable TEGOLA_CLIPPING=mvt
- Added: !ZOOM! token support for PostGIS SQL statements. (#88)
- Added: Support for debug=true query string param in /capabilities endpoints. (#99)
- Added: Config validation for layer name collision. (#81)
- Added: "center" property to map config (#84)
- Added: "bounds" property to map config
- Added: "attribution" property to map config
- Added: Support numeric (decimal) types (#113)
- Added: Configurable Webserver->HostName with fallbacks (#118)
- Added: AddFeatures performance improvements (#121)

## v0.3.2 (2017-03-13)

- Changed: MVT version from 2.1 to 1 per issue (#102)

## v0.3.1 (2017-01-22)

- Enhanced the /capabilities endpoint with bounds, center,tiles and capabilities values.
- Added: /capabilities/:map_name endpoint which returns TileJSON about a map.
- Added: configuration values for map -> center and map -> bounds. These values will be included in the /capabilities and /capabilities/:map_name responses.
- Fixed: bug where the HTTP port was not being read correctly from the config file.
- Added: http(s) prefix to tile URLs returned the /capabilities endpoints

## v0.3.0 (2016-09-11)

- Support for fetching individual layers from a map (i.e. /maps/:map_name/:layer_name/:z/:x/:y)
- Added a `/capabilities` endpoint with information about the tegola version, maps and map layers.
- Fixed an issue where the TOML config parser was not reporting config syntax errors.

## v0.2.0 (2016-08-16)

- Fixed: issue with PostGIS driver not handling nil tag values.
- Fixed: issue building PostGIS queries when tablename is used instead of sql in config file.
- Fixed: issue when table field names could be Postgres keywords.
- Added: concurrent layer fetching from data providers.
- Added: remote config loading over http(s).

## v0.1.0 (2016-07-29)
