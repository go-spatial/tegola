# PostGIS

For raw OGC feature source admission, constrained `feature_sql`, and snapshot
behavior, see [PostGIS feature queries](features.md).

## Typed raw feature filtering

The admitted raw feature Queryables catalog exposes eligible public PostgreSQL
`smallint`, `integer`, `bigint`, native `boolean`, `text` and `varchar` columns.
Names are frozen public projection aliases; identity, geometry and private
physical fields are excluded. Selected public epoch columns remain integers.
Catalog metadata comes from registration proofs, never sampled results.

All admitted fields support six comparisons and NULL tests. Integer comparisons
widen stored values and exact decimal/exponent literals to PostgreSQL numeric,
so fractional and out-of-range bounds retain mathematical meaning. SQL NULL
remains UNKNOWN under comparisons and NOT. Boolean ordering is false before
true. Text/varchar comparisons use UTF-8 bytes with Unicode scalar ordinal order,
significant case, accents and trailing spaces, independently of source collation.
A NUL character in a request literal is bound as bytes; it is not inserted into
PostgreSQL text storage.

Padded `char`, floating-point, numeric-storage, binary/JSON and native DATE/TIMESTAMP
columns are omitted from Queryables until their comparison profiles are separately
proved. Some remain supported output properties; output support alone does not
establish filtering eligibility. An unavailable optional catalog leaves ordinary
feature publication usable.

Filters are compiled from the neutral typed expression into bound parameters and
qualified catalog operators inside the existing protected snapshot, before
logical paging. Configured `feature_sql` selection remains a separate conjunct.
There is no collection-wide Go filtering fallback, and numeric widening or text
conversion does not promise scalar-index use. Actual ordinary/custom filter
checks were independently reviewed on PostgreSQL 16.2 / PostGIS 3.4.1; these
results do not declare OGC conformance. See the [filtering profile](../../docs/filtering.md)
for the HTTP application contract.

The PostGIS provider manages querying for tile requests against a Postgres
database with the [PostGIS](http://postgis.net/) extension installed.
The connection between tegola and Postgis is configured in a `tegola.toml` file.
An example minimum connection config:

```toml
[[providers]]
# provider name is referenced from map layers (required)
name = "test_postgis"

# the type of data provider must be "postgis" for this data provider (required)
type = "postgis"

# PostGIS connection string (required)
uri = "postgres://tegola:supersecret@localhost:5432/tegola?sslmode=prefer" #

# PostGIS connection config run time parameter to label
# the origin of a connection
# The default is "tegola"
# (optional)
application_name = "tegola"

# PostGIS connection config run time parameter (optional)
# A read-only SQL transaction cannot alter non-temporary tables.
# This parameter controls the default read-only status of each new transaction.
# The default is TRUE (read-only). Tegola only reads from PostGIS, so the
# read-only default protects your data. Set it to "off" (or an empty value)
# if a layer's custom SQL needs write access.
# (optional)
default_transaction_read_only = "true"
```

## Connection Properties

-   `uri` (string): [Optional] PostGIS connection string
-   `name` (string): [Required] provider name is referenced from map layers
-   `type` (string): [Required] the type of data provider. enum: postgis, mvt_postgis. 
-   `srid` (int): [Optional] The default SRID for the provider. Defaults to WebMercator (3857) but also supports WGS84 (4326)
-   `crs_defn` (string): [Optional] a full PROJ.4 coordinate system definition used instead of a numeric `srid` (e.g. `"crs_defn" = "+proj=etmerc +lat_0=0 +lon_0=61 +k_0=1 +x_0=500000 +y_0=0 +ellps=krass +units=m +no_defs"`). Validated at startup; wins over `srid` at the same level. Layer-level `crs_defn` overrides the provider-level one.

### env mode vs uri mode

Connection related environment variables take full precedence over `uri` when any trigger (PGHOST,PGPASSWORD,PGUSER,PGDATABASE,PGPORT) is present 
in the environment. With this we follow [libpq environment variables](https://www.postgresql.org/docs/current/libpq-envars.html) and allow users to configure they're connection entirely through the environment.

```
if env trigger variables present:
    mode = env
    ignore uri, uses env vars to establish connection
    can be enriched with config values
else:
    mode = uri, either uri controls it all, or we can enrich with config values
```

> [!WARNING]
> Configuring multiple postgis providers in the same application requires the 
> use of a a connection string.

### Connection string properties

#### Example

```
# {protocol}://{user}:{password}@{host}:{port}/{database}?{options}=
postgres://tegola:supersecret@localhost:5432/tegola?sslmode=prefer&pool_max_conns=10
```

#### Options

Tegola uses [pgx](https://github.com/jackc/pgx/blob/master/pgxpool/pool.go#L111) to manage
PostgresSQL connections that allows the following configurations to be passed
as parameters.

-   `sslmode`: [Optional] PostGIS SSL mode. Default: "prefer"
-   `pool_min_conns`: [Optional] The minimum number of connections to keep in the connection pool. Defaults to 0.
-   `pool_min_idle_conns`: [Optional] The minimum number of idle connections to keep in the connection pool. Defaults to 0.
-   `pool_max_conns`: [Optional] The max connections to maintain in the connection pool. Defaults to the greater of 4 or the number of CPUs.
-   `pool_max_conn_idle_time`: [Optional] The maximum time an idle connection is kept alive. Defaults to "30m".
-   `pool_max_conn_lifetime` [Optional] The maximum time a connection lives before it is terminated and recreated. Defaults to "1h".
-   `pool_max_conn_lifetime_jitter` [Optional] Duration after `max_conn_lifetime` to randomly decide to close a connection.
-   `pool_health_check_period` [Optional] Is the duration between checks of the health of idle connections. Defaults to 1m

### Pool sizing via TOML

The pool count keys (`pool_min_conns`, `pool_min_idle_conns`, `pool_max_conns`)
can also be set in the TOML provider config. Use natural TOML integers
(the legacy quoted-string form is still accepted for backwards compatibility):

```toml
[[providers]]
name = "test_postgis"
type = "postgis"
uri = "******localhost:5432/tegola?sslmode=prefer"
pool_min_conns = 1
pool_min_idle_conns = 2
pool_max_conns = 10
```

### TLS / SSL configuration

SSL can be configured three ways. The provider resolves the effective values
at startup in the following order:

1. Defaults: `ssl_mode = "prefer"`, empty `ssl_key`, `ssl_cert`, `ssl_root_cert`.
2. TOML config keys on the provider:

   ```toml
   [[providers]]
   name = "my_postgis"
   type = "postgis"
   uri = "postgres://tegola:password@localhost:5432/tegola"
   ssl_mode = "verify-full"
   ssl_key = "/path/to/client-key.pem"
   ssl_cert = "/path/to/client-cert.pem"
   ssl_root_cert = "/path/to/ca-cert.pem"
   ```

3. Overrides: in **env mode** (any of `PGHOST`/`PGUSER`/… triggers present)
   non-empty `PGSSLMODE`, `PGSSLKEY`, `PGSSLCERT` and `PGSSLROOTCERT`
   environment variables override the TOML values. In **uri mode** a
   non-empty `sslmode` query parameter in the `uri` overrides the TOML
   `ssl_mode` (the URI `sslkey`/`sslcert`/`sslrootcert` parameters are not
   read by the planner — use the TOML keys for those).

`ssl_key` and `ssl_cert` must be provided together (client certificate
authentication); `ssl_root_cert` enables CA verification independently.
Valid `ssl_mode` values follow libpq: `disable`, `allow`, `prefer`,
`require`, `verify-ca`, `verify-full`.

## Provider Layers

In addition to the connection configuration above, Provider Layers need to be configured. A Provider Layer tells tegola how to query PostGIS for a certain layer. An example minimum config:

```toml
[[providers.layers]]
name = "landuse"
# this table uses "geom" for the geometry_fieldname; set id_fieldname
# explicitly (e.g. "gid") if you want MVT feature IDs to be populated
tablename = "gis.zoning_base_3857"
```

### Provider Layers Properties

-   `name` (string): [Required] the name of the layer. This is used to reference this layer from map layers.
-   `tablename` (string): [*Required] the name of the database table to query against. Required if `sql` is not defined.
-   `geometry_fieldname` (string): [Optional] the name of the filed which contains the geometry for the feature. defaults to `geom`.
-   `id_fieldname` (string): [Optional] the name of the feature id field. Defaults to empty: no id column is added to generated table SQL and MVT feature IDs are left unset (the database-side `ST_AsMVT` receives `NULL` as the feature id name). Set it to your table's primary key (commonly `gid`) to populate feature IDs.
-   `fields` ([]string): [Optional] a list of fields to include alongside the feature. Can be used if `sql` is not defined.
-   `srid` (int): [Optional] the SRID of the layer. Defaults to the provider-level `srid`. Supports any numeric SRID available in the database's `spatial_ref_sys` (a `crs_defn` on the same level wins over a numeric `srid`).
-   `crs_defn` (string): [Optional] a full PROJ.4 coordinate system definition used instead of a numeric `srid`. Validated at startup; wins over `srid` at the same level and over the provider-level `crs_defn`/`srid`.
-   `geometry_type` (string): [Optional] the layer geometry type. If not set, the table will be inspected at startup to try and infer the gemetry type. Valid values are: `Point`, `LineString`, `Polygon`, `MultiPoint`, `MultiLineString`, `MultiPolygon`, `GeometryCollection`.
-   `sql` (string): [*Required] custom SQL to use use. Required if `tablename` is not defined. Supports the following tokens:
    -   `!BBOX!` - [Required] will be replaced with the bounding box of the tile before the query is sent to the database. `!bbox!` and`!BOX!` are supported as well for compatibilitiy with queries from Mapnik and MapServer styles.
    -   `!ZOOM!` - [Optional] will be replaced with the "Z" (zoom) value of the requested tile.
    -   `!X!` - [Optional] will be replaced with the "X" value of the requested tile.
    -   `!Y!` - [Optional] will be replaced with the "Y" value of the requested tile.
    -   `!Z!` - [Optional] will be replaced with the "Z" value of the requested tile.
    -   `!SCALE_DENOMINATOR!` - [Optional] scale denominator, assuming 90.7 DPI (i.e. 0.28mm pixel size)
    -   `!PIXEL_WIDTH!` - [Optional] the unbuffered pixel width in the resolved layer CRS units
    -   `!PIXEL_HEIGHT!` - [Optional] the unbuffered pixel height in the resolved layer CRS units
    -   `!ID_FIELD!` - [Optional] the id field name
    -   `!GEOM_FIELD!` - [Optional] the geom field name
    -   `!GEOM_TYPE!` - [Optional] the geom type field name

`*Required`: either the `tablename` or `sql` must be defined, but not both.

### Common geometry / CRS options

The PostGIS provider implements the common geometry contract documented in
[docs/provider-contract.md](../../docs/provider-contract.md). The keys below
are valid at provider level (defaults for all layers) and at layer level
(overrides), exactly as described there:

- `geometry_type` (string): [Optional] explicit layer geometry type
  (`Point`, `LineString`, `Polygon`, `MultiPoint`, `MultiLineString`,
  `MultiPolygon`, `GeometryCollection`). Skips geometry-class inference (empty data is allowed); automatic MOS format
  detection requires three valid samples, or one in a completed short result. Structural
  validation of custom SQL still runs. Mixed content is permitted with a
  one-time warning.
- `geometry_format` (string): [Optional] `wkb`, `wkt` or `mos`. Empty/unset
  uses the PostGIS native geometry handling. With a raw format the column is
  read as bytes/text (e.g. `ST_AsBinary(geom)`) instead of using the native
  spatial API.
- `mos_precision` (int): [Optional] decimal digits carried by MOS
  coordinates. Only applies when the effective geometry format is `mos`.
- `mos_units` (string): [Optional] packed linear unit of MOS coordinates
  (`mm`, `cm`, `dm`, `m` or `km`). Only applies when the effective geometry
  format is `mos`.
- `bbox_minx_fieldname` / `bbox_maxx_fieldname` / `bbox_miny_fieldname` /
  `bbox_maxy_fieldname` (string): [Optional] bounds columns used by the
  coarse `!BBOX!` filter for raw-format (`wkb`/`wkt`/`mos`) layers, layer
  level overrides provider level, defaults `MINX`/`MAXX`/`MINY`/`MAXY`.
  Resolved bounds columns are excluded from feature tags. For MOS custom SQL
  the `!BBOX!` token is **required** and expands into a bounds-columns
  predicate over these fields (with MOS raw scaling); the provider verifies
  the token at registration. The bounds columns do not need to appear in the
  SELECT list — the predicate resolves them in the query's own scope (e.g.
  the source table's columns). When they are present in the result their
  actual names are used; when absent, registration logs a warning and the
  layer uses the resolved names (layer > provider > `MINX`/`MAXX`/`MINY`/
  `MAXY`).

```toml
[[providers]]
name = "test_postgis"
type = "postgis"
uri = "******localhost:5432/tegola?sslmode=prefer"
geometry_format = "mos"
mos_precision = 2
mos_units = "m"

[[providers.layers]]
name = "rivers"
tablename = "gis.rivers"
geometry_type = "LineString"
```

When `crs_defn` creates a synthetic Tegola-only SRID (≥ 340000001), generated
table SQL uses SRID 0 for the database-side envelope and geometry comparison;
the source coordinates are still interpreted with the configured PROJ.4
definition before MVT encoding. Custom SQL must likewise compare geometries and
`!BBOX!` in a common database SRID (typically by using SRID 0 for both sides,
or by applying `ST_SetSRID` explicitly). Synthetic IDs must not be passed to
PostGIS functions as registered EPSG codes.

#### Example minimum custom SQL config

```toml
[[providers.layers]]
name = "rivers"
# custom SQL to be used for this layer. Note: that the geometery field is wrapped
# in ST_AsBinary() and a !BBOX! token is supplied for querying the table with the tile bounds
sql = "SELECT gid, ST_AsBinary(geom) AS geom FROM gis.rivers WHERE geom && !BBOX!"
```

## Environment Variable support

Helpful debugging environment variables:

-   `TEGOLA_SQL_DEBUG`: specify the type of SQL debug information to output. Supports the following values:
    -   `LAYER_SQL`: print layer SQL as they’re parsed from the config file.
    -   `EXECUTE_SQL`: print SQL that is executed for each tile request and the number of items it returns or an error.
    -   `LAYER_SQL:EXECUTE_SQL`: print `LAYER_SQL` and `EXECUTE_SQL`.

Example:

```bash
$ TEGOLA_SQL_DEBUG=LAYER_SQL tegola serve --config=/path/to/conf.toml
```

## Testing

Testing is designed to work against a live PostGIS database. To see how to set
up a database check this [github actions script](https://github.com/go-spatial/tegola/blob/master/.github/workflows/on_pr_push.yml).
To run the PostGIS tests, the following environment variables need to be set:

```bash
$ export RUN_POSTGIS_TESTS=yes
$ export PGURI="postgres://postgres:postgres@localhost:5432/tegola"
$ export PGURI_NO_ACCESS="postgres://tegola_no_access:@localhost:5432/tegola" # used for testing errors when user does not have read permissions on a table
$ export PGPASSWORD=""
$ export PGSSLMODE="disable"
```

If you're testing SSL, the following additional env vars can be set:

```bash
$ export PGSSLMODE="" // disable, allow, prefer, require, verify-ca, verify-full
$ export PGSSLKEY=""
$ export PGSSLCERT=""
$ export PGSSLROOTCERT=""
```

### Scale token units

Pixel dimensions use the unbuffered tile extent transformed to the layer CRS,
with 256×256 pixels by default. Custom tiles may expose `PixelSize()`.
The scale denominator converts horizontal pixel size to meters and divides by
the OGC 0.00028 m rendering pixel. Projected CRSs use their registered linear
units (including feet); supported geographic CRSs use the source ellipsoid's
local parallel-arc factor `N(phi) * cos(phi) * pi / 180`, where
`N(phi) = a / sqrt(1 - e^2 * sin^2(phi))`. The ellipsoid and the transformed tile
center latitude belong to the layer CRS, including `crs_defn` with `+towgs84`.
Pixel dimensions remain in degrees. The denominator represents local horizontal
scale at the center, not a finite geodesic or diagonal distance. Singular pole
latitudes and unsupported CRS/units
produce an error when a scale token is executed. EPSG:3857 defaults are unchanged;
non-WebMercator SQL thresholds must use the new CRS-aware values.

### SQL token lexical rules

SQL tokens use PostgreSQL lexical rules: array brackets and JSON/hash operators
remain executable SQL; quoted identifiers, ordinary and dollar-quoted strings,
and comments are preserved. The scanner assumes `standard_conforming_strings=on`;
use `E'...'` for backslash escapes. Nondefault `standard_conforming_strings=off`
is not supported by token scanning.

### MOS registration sampling

An effective `geometry_format = "mos"` at provider or layer level disables
storage-format sampling. Registration obtains result-column metadata with a
zero-row query; geometry-column, bounds-token and explicit CRS validation
still apply. If `geometry_type` is also configured, no geometry is unpacked
for startup format/class inference. Otherwise class inference remains a
separate probe. Explicit MOS does not acquire the inferred `sql-sample` tag.

Automatic MOS detection stops after **three successfully decoded, nonempty
MOS geometries**, within at most **16 result rows**. If the result ends before
that threshold, **one valid MOS geometry is sufficient**. Reaching the 16-row
budget without three successes is not treated as the end of a short result. NULL, malformed, empty
and SystemInfo values do not count. Setting `geometry_type` alone does not
select a storage format or disable automatic format detection.

For automatically detected MOS, undecodable feature rows are skipped with a
warning at tile rendering, consistently with the probe. A bad row does not
hide other valid features. Explicit-format error policies are unchanged.
