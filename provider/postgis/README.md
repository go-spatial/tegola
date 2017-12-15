# PostGIS
The PostGIS provider manages querying for tile requests against a Postgres database with the PostGIS extension installed. The connection between tegola and postgis is configured in a `tegola.toml` file. An example minimum config:


```toml
[[providers]]
name = "test_postgis"       # provider name is referenced from map layers (required)
type = "postgis"            # the type of data provider. currently only supports postgis (required)
host = "localhost"          # postgis database host (required)
port = 5432                 # postgis database port (required)
database = "tegola"         # postgis database name (required)
user = "tegola"             # postgis database user (required)
password = ""               # postgis database password (required)
```

### Connection Properties

- `name` (string): [Required] provider name is referenced from map layers (required)
- `type` (string): [Required] the type of data provider. must be "postgis" to use this data provider
- `host` (string): [Required] postgis database host
- `port` (int): [Required] postgis database port (required)
- `database` (string): [Required] postgis database name
- `user` (string): [Required] postgis database user
- `password` (string): [Required] postgis database password
- `srid` (int): [Optional] The default SRID for the provider. Defaults to WebMercator (3857) but also supports WGS84 (4326)
- `max_connections` : [Optional] The max connections to maintain in the connection pool. Default is 100. 0 means no max.

## Provider Layers
In addition to the provider top level configuration, the provider needs to have Provider Layers configured. A Provider Layer tells tegola how to query PostGIS for a certain layer. An example minimum config:

```toml
[[providers.layers]]
name = "landuse"                    # will be encoded as the layer name in the tile
tablename = "gis.zoning_base_3857"  # this table uses "geom" for the geometry field name and "gid" for the id_fieldname so they're not required
```

### Provider Layers Properties

- `name` (string): [Required] the name of the layer. This is used to reference this layer from map layers.
- `tablename` (string): [*Required] the name of the database table to query against. Required if `sql` is not defined.
- `geometry_fieldname` (string): [Optional] the name of the filed which contains the geometry for the feature. defaults to `geom`
- `id_fieldname` (string): [Optional] the name of the feature id field. defaults to `gid`
- `fields` ([]string): [Optional] a list of fields to include alongside the feature. Can be used if `sql` is not defined.
- `srid` (int): [Optional] the SRID of the layer. Supports `3857` (WebMercator) or `4326` (WGS84).
- `sql` (string): [*Required] custom SQL to use use. Required if `tablename` is not defined. Supports the following tokens:
  - !BBOX! - [Required] will be replaced with the bounding box of the tile before the query is sent to the database.
  - !ZOOM! - [Optional] will be replaced with the "Z" (zoom) value of the requested tile.


`*Required`: either the `tablename` or `sql` must be defined, but not both.

**Example minimum custom SQL config**

```toml
[[providers.layers]]
name = "rivers"                     # will be encoded as the layer name in the tile
# custom SQL to be used for this layer. Note: that the geometery field is wraped
# in ST_AsBinary() and a !BBOX! token is supplied for querying the table with the tile bounds
sql = "SELECT gid, ST_AsBinary(geom) AS geom FROM gis.rivers WHERE geom && !BBOX!"
```

## Testing
Testing is designed to work against a live PostGIS database. To run the PostGIS tests, the following environment variables need to be set:

```bash
$ export RUN_POSTGIS_TESTS=yes
```
