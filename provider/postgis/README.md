# PostGIS
The PostGIS provider manages querying for tile requests against a Postgres database with the [PostGIS](http://postgis.net/) extension installed. The connection between tegola and Postgis is configured in a `tegola.toml` file. An example minimum connection config:


```toml
[[providers]]
name = "test_postgis"       # provider name is referenced from map layers (required)
type = "postgis"            # the type of data provider must be "postgis" for this data provider (required)

uri = "postgres://tegola:supersecret@localhost:5432/tegola?sslmode=prefer" # PostGIS connection string (required)

host = "localhost"                # PostGIS database host (deprecated)
port = 5432                       # PostGIS database port (deprecated)
database = "tegola"               # PostGIS database name (deprecated)
user = "tegola"                   # PostGIS database user (deprecated)
password = "supersecret"          # PostGIS database password (deprecated)
max_connections = 10              # PostGIS max connections (deprecated)
max_connection_idle_time = "30m"  # PostGIS max connection idle time (deprecated)
max_connection_lifetime = "1h"    # PostGIS max connection life time (deprecated)
```

### Connection Properties

Establishing a connection via connection string (`uri`) will become the default connection method as of v0.16.0.
Connecting via host/port/database is flagged for deprecation as of v0.15.0 but will be possible until v0.16.0 still.

- `uri` (string): [Required] PostGIS connection string
- `name` (string): [Required] provider name is referenced from map layers
- `type` (string): [Required] the type of data provider. must be "postgis" to use this data provider
- `srid` (int): [Optional] The default SRID for the provider. Defaults to WebMercator (3857) but also supports WGS84 (4326)

#### Connection string properties

**Example**

```
# {protocol}://{user}:{password}@{host}:{port}/{database}?{options}=
postgres://tegola:supersecret@localhost:5432/tegola?sslmode=prefer&pool_max_conns=10
```

**Options**

- `sslmode`: [Optional] PostGIS SSL mode. Default: "prefer"
- `pool_max_conns`: [Optional] The max connections to maintain in the connection pool. Defaults to 100. 0 means no max.
- `pool_max_conn_idle_time`: [Optional] The maximum time an idle connection is kept alive. Defaults to "30m".
- `max_connection_lifetime` [Optional] The maximum time a connection lives before it is terminated and recreated. Defaults to "1h".

### [DEPRECATED] Connection Properties

- `uri` (string): [Required] PostGIS connection string
- `name` (string): [Required] provider name is referenced from map layers
- `type` (string): [Required] the type of data provider. must be "postgis" to use this data provider
- `host` (string): [deprecated] PostGIS database host
- `port` (int): [deprecated] PostGIS database port (required)
- `database` (string): [deprecated] PostGIS database name
- `user` (string): [deprecated] PostGIS database user
- `password` (string): [deprecated] PostGIS database password
- `srid` (int): [Optional] The default SRID for the provider. Defaults to WebMercator (3857) but also supports WGS84 (4326)
- `ssl_mode`: (string): [Optional]. PostGIS SSL mode. Default is "prefer".
- `max_connections` (int): [deprecated] The max connections to maintain in the connection pool. Defaults to 100. 0 means no max.
- `max_connection_idle_time` (duration string): [deprecated] The maximum time an idle connection is kept alive.
- `max_connection_lifetime` (duration string): [deprecated] The maximum time a connection lives before it is terminated and recreated.

## Provider Layers
In addition to the connection configuration above, Provider Layers need to be configured. A Provider Layer tells tegola how to query PostGIS for a certain layer. An example minimum config:

```toml
[[providers.layers]]
name = "landuse"
# this table uses "geom" for the geometry_fieldname and "gid" for the id_fieldname so they don't need to be configured
tablename = "gis.zoning_base_3857"
```

### Provider Layers Properties

- `name` (string): [Required] the name of the layer. This is used to reference this layer from map layers.
- `tablename` (string): [*Required] the name of the database table to query against. Required if `sql` is not defined.
- `geometry_fieldname` (string): [Optional] the name of the filed which contains the geometry for the feature. defaults to `geom`.
- `id_fieldname` (string): [Optional] the name of the feature id field. defaults to `gid`.
- `fields` ([]string): [Optional] a list of fields to include alongside the feature. Can be used if `sql` is not defined.
- `srid` (int): [Optional] the SRID of the layer. Supports `3857` (WebMercator) or `4326` (WGS84).
- `geometry_type` (string): [Optional] the layer geometry type. If not set, the table will be inspected at startup to try and infer the gemetry type. Valid values are: `Point`, `LineString`, `Polygon`, `MultiPoint`, `MultiLineString`, `MultiPolygon`, `GeometryCollection`.
- `sql` (string): [*Required] custom SQL to use use. Required if `tablename` is not defined. Supports the following tokens:
  - `!BBOX!` - [Required] will be replaced with the bounding box of the tile before the query is sent to the database. `!bbox!` and`!BOX!` are supported as well for compatibilitiy with queries from Mapnik and MapServer styles.
  - `!ZOOM!` - [Optional] will be replaced with the "Z" (zoom) value of the requested tile.
  - `!X!` - [Optional] will be replaced with the "X" value of the requested tile.
  - `!Y!` - [Optional] will be replaced with the "Y" value of the requested tile.
  - `!Z!` - [Optional] will be replaced with the "Z" value of the requested tile.
  - `!SCALE_DENOMINATOR!` - [Optional] scale denominator, assuming 90.7 DPI (i.e. 0.28mm pixel size)
  - `!PIXEL_WIDTH!` - [Optional] the pixel width in meters, assuming 256x256 tiles
  - `!PIXEL_HEIGHT!` - [Optional] the pixel height in meters, assuming 256x256 tiles
  - `!ID_FIELD!` - [Optional] the id field name
  - `!GEOM_FIELD!` - [Optional] the geom field name
  - `!GEOM_TYPE!` - [Optional] the geom type field name

`*Required`: either the `tablename` or `sql` must be defined, but not both.

**Example minimum custom SQL config**

```toml
[[providers.layers]]
name = "rivers"
# custom SQL to be used for this layer. Note: that the geometery field is wrapped
# in ST_AsBinary() and a !BBOX! token is supplied for querying the table with the tile bounds
sql = "SELECT gid, ST_AsBinary(geom) AS geom FROM gis.rivers WHERE geom && !BBOX!"
```

## Environment Variable support
Helpful debugging environment variables:

- `TEGOLA_SQL_DEBUG`: specify the type of SQL debug information to output. Supports the following values:
  - `LAYER_SQL`: print layer SQL as they’re parsed from the config file.
  - `EXECUTE_SQL`: print SQL that is executed for each tile request and the number of items it returns or an error.
  - `LAYER_SQL:EXECUTE_SQL`: print `LAYER_SQL` and `EXECUTE_SQL`.

Example:

```
$ TEGOLA_SQL_DEBUG=LAYER_SQL tegola serve --config=/path/to/conf.toml
```

## Testing
Testing is designed to work against a live PostGIS database. To see how to set up a database check this [github actions script](https://github.com/go-spatial/tegola/blob/master/.github/worksflows/on_pr_push.yml). To run the PostGIS tests, the following environment variables need to be set:

```bash
$ export RUN_POSTGIS_TESTS=yes
$ export PGHOST="localhost"
$ export PGPORT=5432
$ export PGDATABASE="tegola"
$ export PGUSER="postgres"
$ export PGUSER_NO_ACCESS="tegola_no_access" # used for testing errors when user does not have read permissions on a table
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
