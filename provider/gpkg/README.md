# GeoPackage
This provider connects to GeoPackage databases (See http://www.geopackage.org/ http://www.opengeospatial.org/standards/geopackage)

The connection between tegola and a GeoPackage is configured in a `tegola.toml` file. An example minimum connection config:

```toml
[[providers]]
name = "sample_gpkg"
type = "gpkg"
filepath = "/path/to/my/sample_gpkg.gpkg"
```

### Connection Properties

- `name` (string): [Required] provider name is referenced from map layers.
- `type` (string): [Required] the type of data provider. must be "gpkg" to use this data provider.
- `filepath` (string): [Required] The system file path to the GeoPackage file you wish to connect to.

## Provider Layers
In addition to the connection configuration above, Provider Layers need to be configured. A Provider Layer tells tegola how to query a GeoPackage for a certain layer. An example minimum config:

```toml
[[providers.layers]]
name = "land_polygons"
tablename = "land_polygons"
id_fieldname = "fid"
```

### Provider Layers Properties

- `name` (string): [Required] the name of the layer. This is used to reference this layer from map layers.
- `tablename` (string): [*Required] the name of the database table to query against. Required if `sql` is not defined.
- `id_fieldname` (string): [Optional] the name of the feature id field. defaults to `fid`
- `fields` ([]string): [Optional] a list of fields (column names) to include as feature tags. Can be used if `sql` is not defined.
- `sql` (string): [*Required] custom SQL to use use. Required if `tablename` is not defined. Supports the following WHERE-clause tokens:
  - !BBOX! - [Required] will be replaced with the bounding box of the tile before the query is sent to the database.  To support this token, your custom SQL must do a couple of things. 
    - You must join your feature table to the spatial index table: i.e. `JOIN feature_table ft rtree_feature_table_geom si ON ft.fid = rt.si`
	- Include the following fields in your SELECT clause: si.minx, si.miny, si.maxx, si.maxy
	- Note that the id field for your feature table may be something other than `fid`
  - !ZOOM! - [Optional] Currently allowed, but does nothing.


`*Required`: either the `tablename` or `sql` must be defined, but not both.

**Example minimum custom SQL config**

```toml
[[providers.layers]]
name = "a_points"
sql = "SELECT fid, geom, amenity, religion, tourism, shop, si.minx, si.miny, si.maxx, si.maxy FROM land_polygons lp JOIN rtree_land_polygons_geom si ON lp.fid = si.id WHERE !BBOX!"
```