# PostGIS MVT Provider

The PostGIS MVT provider manages querying for tile requests against a Postgres database (version 12+) with the [PostGIS](http://postgis.net/)(version 3.0+) extension installed and leverages [ST_AsMVT](https://postgis.net/docs/ST_AsMVT.html) to handle the MVT encoding at the database. 

The connection between tegola and PostGIS is configured in a `tegola.toml` file. An example minimum connection config:


```toml
[[providers]]
name = "test_postgis"       # provider name is referenced from map layers (required)
type = "mvt_postgis"        # the type of data provider must be "mvt_postgis" for this data provider (required)
host = "localhost"          # PostGIS database host (required)
port = 5432                 # PostGIS database port (required)
database = "tegola"         # PostGIS database name (required)
user = "tegola"             # PostGIS database user (required)
password = ""               # PostGIS database password (required)
```

### Connection Properties

- `name` (string): [Required] provider name is referenced from map layers.
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
[[providers.layers]]
name = "landuse"
# MVT data provider must use SQL statements
# this table uses "geom" for the geometry_fieldname and "gid" for the id_fieldname so they don't need to be configured
# Wrapping the geom with ST_AsMVTGeom is required. 
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

## Example mvt_postgis and map config

```toml
[[providers]]
name = "test_postgis"
type = "mvt_postgis"
host = "localhost"
port = 5432
database = "tegola"
user = "tegola"
password = ""

  [[providers.layers]]
  name = "landuse"
  sql = "SELECT ST_AsMVTGeom(geom,!BBOX!) AS geom, gid FROM gis.landuse WHERE geom && !BBOX!"

[[maps]]
name = "cities"
center = [-90.2,38.6,3.0]  # where to center of the map (lon, lat, zoom)

  [[maps.layers]]
  name = "landuse"
  provider_layer = "test_postgis.landuse"
  min_zoom = 0
  max_zoom = 14
```

## Example mvt_postgis and map config for SRID 4326

When using a 4326 projection with ST_AsMVT the SQL statement needs to be modified. ST_AsMVTGeom is expecting data in 3857 projection so the geometries and the `!BBOX!` token need to be transformed prior to `ST_AsMVTGeom` processing them. For example:

```toml
[[providers]]
name = "test_postgis"
type = "mvt_postgis"
host = "localhost"
port = 5432
database = "tegola"
user = "tegola"
password = ""
srid = 4326 # setting the srid on the provider to 4326 will cause the !BBOX! value to use the 4326 projection.

  [[providers.layers]]
  name = "landuse"
  # the !BBOX! token used in the WHERE clause is not reprojected so it can match 4326 data
  # the matched data AND the !BBOX! are then reporojected to 3857 prior to being passed to ST_AsMVTGeom
  sql = "SELECT ST_AsMVTGeom(ST_Transform(geom, 3857),ST_Transform(!BBOX!,3857)) AS geom, gid FROM gis.landuse WHERE geom && !BBOX!"

[[maps]]
name = "cities"
center = [-90.2,38.6,3.0]  # where to center of the map (lon, lat, zoom)

  [[maps.layers]]
  name = "landuse"
  provider_layer = "test_postgis.landuse"
  min_zoom = 0
  max_zoom = 14

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
