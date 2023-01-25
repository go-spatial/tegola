# HANA MVT Provider

The HANA MVT provider manages querying for tile requests against a HANA database with [Vector Tiles support](https://help.sap.com/docs/HANA_CLOUD_DATABASE/bc9e455fe75541b8a248b4c09b086cf5/8cd683c4bb664fd8a71fc3f19ffa7e42.html) to handle the MVT encoding at the database. 

The connection between tegola and HANA is configured in a `tegola.toml` file. An example minimum connection config:


```toml
[[providers]]
name = "test_hana"       # provider name is referenced from map layers (required)
type = "mvt_hana"        # the type of data provider must be "mvt_hana" for this data provider (required)
uri = "hdb://myuser:mypassword@something.hanacloud.ondemand.com:443?" # HANA connection string (required)
```

### Connection Properties

- `uri` (string): [Required] HANA connection string
- `name` (string): [Required] provider name is referenced from map layers
- `type` (string): [Required] the type of data provider. must be "hana" to use this data provider
- `srid` (int): [Optional] The default SRID for the provider. Defaults to WebMercator (3857) but also supports WGS84 (4326)

#### Connection string properties

**Example**

```
# {protocol}://{user}:{password}@{host}:{port}/{database}?{options}=
hdb://myuser:mypassword@something.hanacloud.ondemand.com:443?TLSInsecureSkipVerify&timeout=3600&max_connections=30
```

**Options**

- `timeout`: [Optional] Driver side connection timeout in seconds.
- `TLSRootCAFile` [Optional] Path,- filename to root certificate(s).
- `TLSServerName` [Optional] ServerName to verify the hostname. By setting TLSServerName=host, the provider will set TLSServerName same as 'host' value in `uri`.
- `TLSInsecureSkipVerify` [Optional] Controls whether a client verifies the server's certificate chain and host name.
- `max_connections`: [Optional] The max connections to maintain in the connection pool. Defaults to 100. 0 means no max.
- `max_connection_idle_time`: [Optional] The maximum time an idle connection is kept alive. Defaults to "30m".
- `max_connection_life_time` [Optional] The maximum time a connection lives before it is terminated and recreated. Defaults to "1h".

## Provider Layers

In addition to the connection configuration above, Provider Layers need to be configured. A Provider Layer tells tegola how to query HANA for a certain layer. When using the HANA MVT Provider the `ST_AsMVTGeom()` MUST be used. An example minimum config using the `sql` config option:

```toml
[[providers.layers]]
name = "landuse"
# MVT data provider can use both table names and SQL statements. Internally, the provider wraps an SQL query by using 
# ST_AsMVT and ST_AsMVTGeom functions.
tablename = "gis.zoning_base_3857"
```

### Provider Layers Properties

- `name` (string): [Required] the name of the layer. This is used to reference this layer from map layers.
- `geometry_fieldname` (string): [Optional] the name of the filed which contains the geometry for the feature. Defaults to `geom`.
- `id_fieldname` (string): [Optional] the name of the feature id field. defaults to `gid`.
- `geometry_type` (string): [Optional] the layer geometry type. If not set, the table will be inspected at startup to try and infer the gemetry type. Valid values are: `Point`, `LineString`, `Polygon`, `MultiPoint`, `MultiLineString`, `MultiPolygon`, `GeometryCollection`.
- `srid` (int): [Optional] the SRID of the layer. Supports `3857` (WebMercator) only.
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
- `buffer` (int): [Optional] the buffer distance by which the clipped geometry may exceed the tile's area. Defaults to 256.  
- `clip_geometry` (bool): [Optional] the flag to control whether the geometry is clipped to the tile bounds or not. Defaults to `TRUE`.

## Example mvt_hana and map config

```toml
[[providers]]
name = "test_hana"
type = "mvt_hana"
uri = "hdb://myuser:mypassword@something.hanacloud.ondemand.com:443?" # HANA connection string (required)
srid = 3857                                                           # The only supported srid is 3857 (optional)

  [[providers.layers]]
  name = "landuse"
  sql = "SELECT geom, gid FROM gis.landuse WHERE !BBOX!"

[[maps]]
name = "cities"
center = [-90.2,38.6,3.0]  # where to center of the map (lon, lat, zoom)

  [[maps.layers]]
  name = "landuse"
  provider_layer = "test_hana.landuse"
  min_zoom = 0
  max_zoom = 14
```

## Example mvt_hana and map config for SRID 4326

When using a 4326 projection with ST_AsMVT the SQL statement needs to be modified. `ST_AsMVTGeom` is expecting data in 3857 projection so the geometries and the `!BBOX!` token need to be transformed prior to `ST_AsMVTGeom` processing them. For example:

```toml
[[providers]]
name = "test_hana"
type = "mvt_hana"
uri = "hdb://myuser:mypassword@something.hanacloud.ondemand.com:443?" # HANA connection string (required)
srid = 3857                                                           # The only supported srid is 3857 (optional)

  [[providers.layers]]
  name = "landuse"
  sql = "SELECT * FROM (SELECT id, name, geom.ST_Transform(3857) AS geom FROM ne_50m_rivers_lake_centerlines) AS sub WHERE !BBOX!"

[[maps]]
name = "cities"
center = [-90.2,38.6,3.0]  # where to center of the map (lon, lat, zoom)

  [[maps.layers]]
  name = "landuse"
  provider_layer = "test_hana.landuse"
  min_zoom = 0
  max_zoom = 14
```

## Testing

Testing is designed to work against a live SAP HANA database. To see how to set up a database check this [github actions script](https://github.com/go-spatial/tegola/blob/master/.github/worksflows/on_pr_push.yml). To run the HANA tests, the following environment variables need to be set:

```bash
$ export RUN_HANA_TESTS=yes
$ export HANA_CONNECTION_STRING="hdb://myuser:mypassword@something.hanacloud.ondemand.com:443?TLSInsecureSkipVerify"
```