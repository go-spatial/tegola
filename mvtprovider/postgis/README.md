# PostGIS MVT Provider

The PostGIS MVT provider manages querying for tile requests against a Postgres database (version 12+) with the [PostGIS](http://postgis.net/)(version 3.0+) extension installed and leverages [ST_AsMVT](https://postgis.net/docs/ST_AsMVT.html) to handle the MVT encoding at the database. 

The connection between tegola and PostGIS is configured in a `tegola.toml` file. An example minimum connection config:


```toml
[[mvt_providers]]
name = "test_postgis"       # provider name is referenced from map layers (required)
type = "postgis"            # the type of data provider must be "postgis" for this data provider (required)
host = "localhost"          # PostGIS database host (required)
port = 5432                 # PostGIS database port (required)
database = "tegola"         # PostGIS database name (required)
user = "tegola"             # PostGIS database user (required)
password = ""               # PostGIS database password (required)
```

### Connection Properties

- `name` (string): [Required] provider name is referenced from map layers. please note that when referencing an mvt_provider form a map layer the provider name must be prexied with `mvt_`. See example config below.
- `type` (string): [Required] the type of data provider. must be "postgis" to use this data provider
- `host` (string): [Required] PostGIS database host
- `port` (int): [Required] PostGIS database port (required)
- `database` (string): [Required] PostGIS database name
- `user` (string): [Required] PostGIS database user
- `password` (string): [Required] PostGIS database password
- `max_connections` (int): [Optional] The max connections to maintain in the connection pool. Defaults to 100. 0 means no max.
- `srid` (int): [Optional] The default SRID for the provider. Defaults to WebMercator (3857) but also supports WGS84 (4326)

## Provider Layers

In addition to the connection configuration above, Provider Layers need to be configured. A Provider Layer tells tegola how to query PostGIS for a certain layer. When using the PostGIS MVT Provider the `ST_AsMVTGeom()` MUST be used. An example minimum config using the `sql` config option:

```toml
[[mvt_providers.layers]]
name = "landuse"
# this table uses "geom" for the geometry_fieldname and "gid" for the id_fieldname so they don't need to be configured
sql = "SELECT ST_AsMVTGeom(geom,!BBOX!) AS geom, gid FROM gis.landuse WHERE geom && !BBOX!"
```

### Provider Layers Properties

- `name` (string): [Required] the name of the layer. This is used to reference this layer from map layers.
- `geometry_fieldname` (string): [Optional] the name of the filed which contains the geometry for the feature. defaults to `geom`.
- `id_fieldname` (string): [Optional] the name of the feature id field. defaults to `gid`.
- `geometry_type` (string): [Optional] the layer geometry type. If not set, the table will be inspected at startup to try and infer the gemetry type. Valid values are: `Point`, `LineString`, `Polygon`, `MultiPoint`, `MultiLineString`, `MultiPolygon`, `GeometryCollection`.
- `srid` (int): [Optional] the SRID of the layer. Supports `3857` (WebMercator) or `4326` (WGS84).
- `sql` (string): [Required] custom SQL to use use. Supports the following tokens:
  - `!BBOX!` - [Required] will be replaced with the bounding box of the tile before the query is sent to the database. `!bbox!` and`!BOX!` are supported as well for compatibilitiy with queries from Mapnik and MapServer styles.
  - `!X!` - [Optional] will replaced with the "X" value of the requested tile.
  - `!Y!` - [Optional] will replaced with the "Y" value of the requested tile.
  - `!Z!` - [Optional] will replaced with the "Z" value of the requested tile.
  - `!ZOOM!` - [Optional] will be replaced with the "Z" (zoom) value of the requested tile.
  - `!SCALE_DENOMINATOR!` - [Optional] scale denominator, assuming 90.7 DPI (i.e. 0.28mm pixel size)
  - `!PIXEL_WIDTH!` - [Optional] the pixel width in meters, assuming 256x256 tiles
  - `!PIXEL_HEIGHT!` - [Optional] the pixel height in meters, assuming 256x256 tiles
  - `!ID_FIELD!` - [Optional] the id field name
  - `!GEOM_FIELD!` - [Optional] the geom field name
  - `!GEOM_TYPE!` - [Optional] the geom type if defined otherwise ""

## Example mvt_provider and map config

**Important**: When referencing the `provider` in the `map` section of the config, you MUST prepend `mvt_` to the `provider_layer` value. This indicates to tegola that the provider is an MVT provider so tegola knows which provider section to perform the lookup. 

Example:

```toml
[[mvt_providers]]
name = "test_postgis"       
type = "postgis"            
host = "localhost"          
port = 5432                 
database = "tegola"         
user = "tegola"             
password = ""

  [[mvt_providers.layers]]
  name = "landuse"
  sql = "SELECT ST_AsMVTGeom(geom,!BBOX!) AS geom, gid FROM gis.landuse WHERE geom && !BBOX!"

[[maps]]
name = "cities"
center = [-90.2,38.6,3.0]  # where to center of the map (lon, lat, zoom)

    [[maps.layers]]
    name = "landuse"
    # note the mvt_ prefix on the name of the provider.
    provider_layer = "mvt_test_postgis.landuse" # note the addition of "mvt_" to the provider name
    min_zoom = 0
    max_zoom = 14
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
