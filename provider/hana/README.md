# HANA
The HANA provider manages querying for tile requests against an [SAP HANA](https://www.sap.com/products/hana.html) database. The connection between tegola and HANA is configured in a `tegola.toml` file. An example minimum connection config:


```toml
[[providers]]
name = "test_hana"       # provider name is referenced from map layers (required)
type = "hana"            # the type of data provider must be "hana" for this data provider (required)
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
In addition to the connection configuration above, Provider Layers need to be configured. A Provider Layer tells tegola how to query HANA for a certain layer. An example minimum config:

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
  - `!SCALE_DENOMINATOR!` - [Optional] scale denominator, assuming 90.7 DPI (i.e. 0.28mm pixel size).
  - `!PIXEL_WIDTH!` - [Optional] the pixel width in meters, assuming 256x256 tiles.
  - `!PIXEL_HEIGHT!` - [Optional] the pixel height in meters, assuming 256x256 tiles.
  - `!ID_FIELD!` - [Optional] the id field name.
  - `!GEOM_FIELD!` - [Optional] the geom field name.
  - `!GEOM_TYPE!` - [Optional] the geom type field name.

`*Required`: either the `tablename` or `sql` must be defined, but not both.

**Example minimum custom SQL config**

```toml
[[providers.layers]]
name = "rivers"
# Custom sql to be used for this layer as a sub query. ST_AsBinary and !BBOX! filter are applied automatically.
sql = "(SELECT id, geom FROM gis.rivers) AS sub"
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
Testing is designed to work against a live SAP HANA database. To see how to set up a database check this [github actions script](https://github.com/go-spatial/tegola/blob/master/.github/worksflows/on_pr_push.yml). To run the HANA tests, the following environment variables need to be set:

```bash
$ export RUN_HANA_TESTS=yes
$ export HANA_CONNECTION_STRING="hdb://myuser:mypassword@something.hanacloud.ondemand.com:443?TLSInsecureSkipVerify"
```
