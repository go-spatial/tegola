[← API Reference](api.md) · [Back to README](../README.md) · [Development and Builds →](development.md)
# Configuration

## Feature publication

Raw feature publication is disabled by default and uses explicit source mappings,
independently of map presentation layers:

```toml
[features]
enabled = true
basepath = "/features"
default_limit = 100
max_limit = 10000
max_response_bytes = 16777216
query_timeout_ms = 30000
title = "Feature data"

[[features.collections]]
id = "hydro"
provider_layer = "gpkg.hydro"
title = "Hydrography"
description = "Published hydrographic features"
```

The defaults are `/features`, a page limit of 100 and a maximum of 10000. Limits
must be positive, and the default cannot exceed the maximum. Provider bindings
use exactly one dot between provider and layer names. Public IDs and base-path
segments use ASCII letters, digits, `-`, `.`, `_` and `~`; empty, `.` and `..`
segments are invalid. Base paths start with `/` and have no trailing slash.
Existing map, capabilities, metrics and embedded viewer namespaces are reserved.

`max_response_bytes` defaults to 16 MiB and must be at least 1024 bytes.
`query_timeout_ms` defaults to 30000 milliseconds and must be a positive,
representable Go duration. Both apply to feature publication through CLI and
Lambda. A deadline relies on provider cancellation support. The response cap
measures the selected JSON or HTML representation before headers are committed;
it also applies to HEAD and does not guarantee a bound on all intermediate
allocations. Oversized responses return a generic error. See the
[HTTP policy](api.md#errors-and-conformance-status).

Only explicitly mapped standard-provider layers are exposed. Missing sources,
MVT-only providers and unsupported query metadata fail startup. Disabled
publication still validates configuration syntax but does not resolve sources
or register feature routes. GeoPackage supports eligible table-backed layers.
MySQL/MariaDB and PostGIS also provide constrained feature selections; consult
their backend eligibility rules before mapping a collection. HANA admits the
tested source profiles under its transaction-owned source-lock protocol, which
blocks source writers and DDL for the query duration. Unsupported native storage
and CRS profiles remain explicit capability boundaries.

Layer-level `feature_sql` selects a raw feature domain independently of tile
`sql`. It accepts one physical table, direct column projections with optional
aliases and parameterized scalar filters. Identity, geometry and mapped temporal
columns must be projected. Joins, computed projections, wildcards, ordering,
paging and tile macros are unsupported. Malformed selections fail registration.
For custom tile SQL, omitting `feature_sql` leaves raw publication unsupported.
See [safe feature SQL](architecture/decisions/ADR-0007-safe-provider-feature-sql.md),
[MySQL/MariaDB](../provider/mysql/README.md) and
[PostGIS](../provider/postgis/features.md), and
[HANA](../provider/hana/FEATURE-QUERY.md) for exact profiles. HANA source protection
is specified in [ADR-0008](architecture/decisions/ADR-0008-hana-feature-source-lock.md).

Actual MySQL/MariaDB, PostGIS and HANA admitted ordinary/custom, dimensional,
exact temporal and nullable-property profiles have passed independent source
and runtime review. Native storage rejection or normalization is recorded
separately from successful feature queries. These results do not establish an
OGC conformance declaration.

Published reads follow the server's existing read-access policy. Protect them
through the deployment's access controls when required. Feature responses use
`Cache-Control: no-store` and do not use the tile cache. The configured server
URI prefix also applies to the feature base path.

Discovery resources and implemented conformance declarations are available under
the base path; configuring publication alone establishes no OGC conformance
claim. See the [feature architecture](architecture/feature-service.md) and
[publication decision](architecture/decisions/ADR-0004-feature-publication-and-runtime.md).

## Cache backend guides

| Registry name | Backend guide |
|---|---|
| `azblob` | [Azure Blob](../cache/azblob/README.md) |
| `file` | [File](../cache/file/README.md) |
| `gcs` | [Google Cloud Storage](../cache/gcs/README.md) |
| `memory` | [Memory](../cache/memory/README.md) |
| `multilevel` | [Multilevel](../cache/multilevel/README.md) |
| `redis` | [Redis](../cache/redis/README.md) |
| `s3` | [S3](../cache/s3/README.md) |

The tile cache is independent of the data provider, so the same cache
configuration works for PostGIS, GeoPackage, MySQL, and other providers. For a
process-local in-memory cache:

```toml
[cache]
type = "memory"
# Optional: do not cache tiles above this zoom.
max_zoom = 18
# Optional: lazy expiration in seconds; 0 keeps entries until restart.
ttl = 3600
```

The memory cache is volatile and shared by all maps and layers in the running
Tegola process. `max_zoom` controls which tiles are inserted, while total
memory usage is limited only by available process memory; use a persistent or
distributed backend when cache contents must survive restarts or be shared
between instances.

For a two-level memory-plus-file cache, configure each backend independently:

```toml
[cache]
type = "multilevel"

[cache.memory]
max_zoom = 18
ttl = 60

[cache.file]
basepath = "./cache/maps"
max_zoom = 22
ttl = 86400
```

The `multilevel` cache requires both nested sections. The `memory` section
supports `max_zoom` (optional, defaults to the maximum Tegola zoom) and `ttl`
(optional seconds, `0` means no expiration until restart). The `file` section
requires `basepath` and also supports `max_zoom` (optional, defaults to the
maximum Tegola zoom) and `ttl` (optional seconds, `0` means no expiration).

Reads check memory first and then file; a file hit is promoted to memory.
Writes and purges are sent to both levels. A failed promotion is logged while
the successful file hit is still returned. A non-cancellation memory read
error is logged and falls back to file; if both reads fail, both errors are
returned. Write and purge errors from either level are returned to the caller.

The Tegola config file uses the [TOML](https://github.com/toml-lang/toml) format. The following example shows how to configure a `mvt_postgis` data provider. The `mvt_postgis` provider uses PostGIS's `ST_AsMVT()` function for the encoding of the vector tile.

Under the `maps` section, map layers are associated with data provider layers and their `min_zoom` and `max_zoom` values are defined.

### Example config using Postgres 12+ / PostGIS 3.0 ST_AsMVT():

```toml
# register a MVT data provider. MVT data providers have the prefix "mvt_" in their type
# note mvt data providers can not be conflated with any other providers of any type in a map
# thus a map may only contain a single mvt provider.
[[providers]]
name = "my_postgis"         # provider name is referenced from map layers (required).
type = "mvt_postgis"        # the type of data provider must be "mvt_postgis" for this data provider (required)
uri = "postgresql://tegola:<password>@localhost:5432/tegola?ssl_mode=prefer" # database connection string

  [[providers.layers]]
  name = "landuse"
  # MVT data provider must use SQL statements
  # this table uses "geom" for the geometry_fieldname and "gid" for the id_fieldname so they don't need to be configured
  # Wrapping the geom with ST_AsMVTGeom is required.
  # If you want to use the configurable parameters defined in maps.params make sure to include the token in the SQL statement
  sql = "SELECT ST_AsMVTGeom(geom,!BBOX!) AS geom, gid FROM gis.landuse WHERE geom && !BBOX! !PARAM!"

# maps are made up of layers
[[maps]]
name = "zoning"                           # used in the URL to reference this map (/maps/zoning)

  [[maps.layers]]
  name = "landuse"                        # name is optional. If it's not defined the name of the ProviderLayer will be used.
  provider_layer = "my_postgis.landuse"   # must match a data provider layer
  min_zoom = 10                           # minimum zoom level to include this layer
  max_zoom = 16                           # maximum zoom level to include this layer

  # configure addition URL parameters: /maps/:map_name/:layer_name/:z/:x/:y?param=value
  # which will be passed to the database queries
  [[maps.params]]
  name          = "param"         # name used in the URL
  token         = "!PARAM!"       # token to replace in providers.layers.sql query
  type          = "string"        # one of: int, float, string, bool
  sql           = "AND param = ?" # SQL to replace the token in the query. ? will be replaced with a parameter value. If omitted, defaults to "?"
  # if neither default_value nor default_sql is specified, the URL parameter is required to be present in all queries
  # either
  default_value = "value"         # if parameter is not specified, this value will be passed to .sql parameter
  # or
  default_sql   = " "             # if parameter is not specified, this value will replace the .sql parameter. Useful for omitting query entirely
```

- More information on PostgreSQL SSL modes can be found [here](https://www.postgresql.org/docs/current/libpq-ssl.html).
- More information on the `mvt_postgis` provider can be found [here](../mvtprovider/postgis)

## MySQL 5.5 table identity

MySQL 5.5 supports InnoDB, but lacks the native InnoDB physical-table identity
catalog used by the feature provider. A provider-level opt-in enables a legacy
metadata snapshot contract:

```toml
[[providers]]
name = "legacy_test"
type = "mysql"
# Other connection settings are deployment-specific.
allow_legacy_table_identity = true
```

The option defaults to `false` and applies only to MySQL 5.5, not MariaDB version
compatibility strings or modern MySQL. It does not bypass missing privileges or
catalog failures on newer servers. InnoDB, keys, column metadata, write policies
and schema revalidation remain required. The fallback uses creation time and
schema metadata; it cannot prove physical incarnation after an external table
drop/recreate with identical shape in the same second. External DDL is unsupported
while such a collection is published. Stop publication before DDL and rebuild
the service's identity/revision state before re-enabling writes.

Fresh MySQL audit/service schema uses explicit Unicode-safe text encoding so
non-ASCII identifiers and attribute values are independent of the database's
default charset. Existing service-table upgrades require separate operational
validation; creating a fresh schema is not evidence of an in-place migration.
See [write operations](operational.md) and [provider evidence](provider-matrix.md).

## Tile cache maintenance and editing

A configured cache serves ordinary tile requests even when feature writes are
enabled. The write-enabled router uses a fresh namespace at startup and advances
it after a successful or unknown mutation commit. Client applications can request
uncached tiles during editing with `X-Tegola-Editor-Active: true`; ordinary viewing
omits this header. Writable tile responses use HTTP `no-store` independently of
the server-side cache. See [tile policy](api.md#tile-cache-policy).

Explicit status/regeneration operations require separate authorization:

```toml
[webserver.tile_operations]
enabled = true
token = "${TEGOLA_TILE_OPERATIONS_TOKEN}"
rate_per_minute = 60
max_concurrent = 4
```

Use a generated secret in `TEGOLA_TILE_OPERATIONS_TOKEN`, and send it as
`X-Tegola-Tile-Operations-Token`. An enabled gate without a nonempty token fails
closed. Limits are shared by clients of the process; zero/negative limit settings
use the defaults shown above. Status consumes the rate budget but no mutation
concurrency slot. This token authorizes tile maintenance only, not feature writes.

For a cross-origin browser client, the configured CORS headers must permit
`X-Tegola-Tile-Operations-Token` and, when used, `X-Tegola-Editor-Active` in
`Access-Control-Allow-Headers`. A token embedded in a public HTML page is visible
to its users; choose the deployment's authorized client boundary accordingly.
Old writable cache namespaces need backend expiry or cleanup. Reader replicas and
external database writers need a separate invalidation strategy; the built-in
router generation is process-local.

## HTTP bind address

`webserver.port` and the `serve --port` option take an address, not a bare
port number:

```toml
[webserver]
port = ":8083"
```

```powershell
.\tegola.exe serve --config .\config.toml --port :8083
# Bind only to localhost when testing:
.\tegola.exe serve --config .\config.toml --port 127.0.0.1:18083 --no-cache
```

An explicit `--port` overrides `webserver.port`. `--port 8083` fails with
`listen tcp: address 8083: missing port in address`, even if the TOML value
is correct. Omit the flag to use the configured address.

For joined MOS queries and registration timeouts, see
[bounds-backed MOS SQL](geometry-formats.md#bounds-backed-mos-sql-bbox-over-bounds-columns)
and [MySQL troubleshooting](../provider/mysql/README.md#troubleshooting-mos-custom-sql).

## Environment Variables

### Config TOML

Environment variables can be injected into the configuration file. One caveat is that the injection has to be within a string, though the value it represents does not have to be a string.

The above config example could be written as:

```toml
# register data providers
[[providers]]
name = "test_postgis"
type = "mvt_postgis"
uri = "${POSTGIS_CONN_STR}"  # database connection string
srid = 3857
max_connections = "${POSTGIS_MAX_CONN}"
```

### Runtime options

For standard providers other than `mvt_postgis`, `TEGOLA_OPTIONS` accepts comma- or space-delimited options:

- `DontSimplifyGeo` turns off simplification for all layers.
- `SimplifyMaxZoom={{int}}` sets the maximum zoom at which simplification applies (14 by default).

## Explicit WFS and write publication

WFS and writes are opt-in. The stock `tegola serve` executable has no production
authentication adapter. It rejects enabled writes with `auth_mode = "production"`
or an omitted auth mode at startup, instead of starting unusable write routes.
An HTTP proxy or an Authorization header alone does not install an authenticator
in Tegola.

The following production configuration is for a custom host that supplies the
authentication adapter to both Feature API and WFS handlers; it is not a working
stock CLI authentication setup:

```toml
[wfs]
enabled = true
basepath = "/wfs"
versions = ["1.1.0", "2.0.0"]

[features.write]
enabled = true
auth_mode = "production"
require_if_match = true

[[features.write.collections]]
id = "sites" # must already be a published feature collection
operations = ["create", "replace", "update", "delete"]
```

Production requires an installed authenticator; anonymous writes are denied.
`auth_mode = "dev"` deliberately permits anonymous writes and is only appropriate
for a trusted disposable test deployment. The provider must separately admit
the physical layer for writing. Enabling WFS alone never enables transactions.
Create additionally requires a database-generated primary key; an ordinary
integer primary key may still be admitted for update/delete when create is not
configured. Operation names are case-insensitive, including OpenAPI publication.
Read [write scope and operating limits](wfs-scope-limitations.md) before enabling
this profile, including the cache policy required on all reader replicas.

## See Also

- [API Reference](api.md) — HTTP endpoints and query parameters
- [Provider contract](provider-contract.md) — shared provider configuration
- [CRS contract](crs.md) — `srid` and `crs_defn` behavior

## Lambda executable scope

The stock `cmd/tegola_lambda` entry point binds tile maps and read-only feature
publication. It does not bind WFS, feature writes/authentication,
`tile_http_max_age` or `[webserver.tile_operations]` from this configuration.
See the [Lambda guide](../cmd/tegola_lambda/README.md) before reusing a CLI
configuration in that executable.