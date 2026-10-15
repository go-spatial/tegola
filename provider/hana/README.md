# HANA

For raw feature queries, configuration boundaries, transaction locking and
supported dimensional profiles, see [Feature queries](FEATURE-QUERY.md).

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
- `srid` (int): [Optional] The default SRID for the provider. When omitted the SRID is auto-detected from the geometry column. Any numeric SRID known to the database is supported, as well as a synthetic SRID registered via `crs_defn`.
- `crs_defn` (string): [Optional] A PROJ.4 definition of the coordinate reference system, registered internally under a synthetic SRID. Wins over `srid` on the same level. Requires a raw `geometry_format` (`wkb`/`wkt`/`mos`) and is not supported for `mvt_hana`. See [docs/crs.md](../../docs/crs.md).
- `geometry_format` (string): [Optional] Provider-level default geometry format for layers that do not override it: `wkb`, `wkt` or `mos`. Empty/unset means HANA's native `ST_Geometry` handling (`ST_AsBinary`). See [docs/geometry-formats.md](../../docs/geometry-formats.md).
- `mos_precision` (int): [Optional] Decimal digits carried by MOS coordinates. Only applies when `geometry_format = "mos"`.
- `mos_units` (string): [Optional] Packed linear unit of MOS coordinates: `mm`, `cm`, `dm`, `m` or `km`. Only applies when `geometry_format = "mos"`.

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
# this table uses "geom" for the geometry_fieldname; set id_fieldname because
# the HANA provider does not infer an id column by default
tablename = "gis.zoning_base_3857"
id_fieldname = "gid"
```

### Provider Layers Properties

- `name` (string): [Required] the name of the layer. This is used to reference this layer from map layers.
- `tablename` (string): [*Required] the name of the database table to query against. Required if `sql` is not defined.
- `geometry_fieldname` (string): [Optional] the name of the filed which contains the geometry for the feature. defaults to `geom`.
- `id_fieldname` (string): [Optional] the name of the feature id field. Defaults to empty (no id attribute is attached to features unless configured). See [docs/provider-contract.md](../../docs/provider-contract.md).
- `fields` ([]string): [Optional] a list of fields to include alongside the feature. Can be used if `sql` is not defined.
- `srid` (int): [Optional] the SRID of the layer. Any numeric SRID known to the database is supported, as well as a synthetic SRID registered via `crs_defn`.
- `crs_defn` (string): [Optional] layer-level PROJ.4 CRS override; wins over `srid` on the same level. Requires a raw `geometry_format` (`wkb`/`wkt`/`mos`) and is not supported for `mvt_hana`. See [docs/crs.md](../../docs/crs.md).
- `geometry_format` (string): [Optional] layer-level geometry format override: `wkb`, `wkt` or `mos`. Overrides the provider-level default.
- `mos_precision` / `mos_units` (int / string): [Optional] layer-level MOS overrides (only with `geometry_format = "mos"`). Explicit values always win over the `MapplGIS LayerInfo` self-description blob. See [docs/geometry-formats.md](../../docs/geometry-formats.md).
- `geometry_type` (string): [Optional] the layer geometry type. If not set, the table will be inspected at startup to try and infer the gemetry type. Valid values are: `Point`, `LineString`, `Polygon`, `MultiPoint`, `MultiLineString`, `MultiPolygon`, `GeometryCollection`.
- `bbox_minx_fieldname` / `bbox_maxx_fieldname` / `bbox_miny_fieldname` / `bbox_maxy_fieldname` (string): [Optional] bounds columns used by the coarse `!BBOX!` filter for raw-format (`wkb`/`wkt`/`mos`) layers, layer level overrides provider level, defaults `MINX`/`MAXX`/`MINY`/`MAXY`. Resolved bounds columns are excluded from feature tags.
- `sql` (string): [*Required] custom SQL to use use. Required if `tablename` is not defined. Supports the following tokens:
  - `!BBOX!` - [Required] will be replaced with the bounding box of the tile before the query is sent to the database. `!bbox!` and`!BOX!` are supported as well for compatibilitiy with queries from Mapnik and MapServer styles. For MOS custom SQL the token expands into a bounds-columns predicate over the configured bounds fields (with MOS raw scaling) and the provider verifies its presence at registration. The bounds columns do not need to appear in the SELECT list — the predicate resolves them in the query's own scope (e.g. the source table's columns). When they are present in the result their actual names are used; when absent, registration logs a warning and the layer uses the resolved names (layer > provider > `MINX`/`MAXX`/`MINY`/`MAXY`).
  - `!ZOOM!` - [Optional] will be replaced with the "Z" (zoom) value of the requested tile.
  - `!X!` - [Optional] will be replaced with the "X" value of the requested tile.
  - `!Y!` - [Optional] will be replaced with the "Y" value of the requested tile.
  - `!Z!` - [Optional] will be replaced with the "Z" value of the requested tile.
  - `!SCALE_DENOMINATOR!` - [Optional] horizontal scale denominator using the OGC 0.28mm rendering pixel; see scale tokens below.
  - `!PIXEL_WIDTH!` - [Optional] unbuffered pixel width in the resolved layer's CRS units.
  - `!PIXEL_HEIGHT!` - [Optional] unbuffered pixel height in the resolved layer's CRS units.
  - `!ID_FIELD!` - [Optional] the id field name.
  - `!GEOM_FIELD!` - [Optional] the geom field name.
  - `!GEOM_TYPE!` - [Optional] the geom type field name.

`*Required`: either the `tablename` or `sql` must be defined, but not both.

### Common geometry / CRS options

The HANA provider implements the common geometry contract documented in
[docs/provider-contract.md](../../docs/provider-contract.md). The
`geometry_type` layer key, `geometry_format`, `mos_precision`, `mos_units`
and the CRS keys above follow the shared semantics; mixed-content features
under an explicit `geometry_type` are permitted with a one-time warning.

### Scale tokens

At query time HANA transforms the unbuffered tile perimeter into the resolved
layer CRS and uses its enclosing extent (including sampled edges for curved
projections). `!PIXEL_WIDTH! = extent width / pixel columns` and
`!PIXEL_HEIGHT! = extent height / pixel rows`, in **layer CRS units**.
The pixel dimensions are independent of the MVT integer coordinate extent and
tile buffer. The built-in slippy tiles use the framework's
`slippy.DefaultTileSize` (currently 256). There is no map/tile-size config key;
custom Go `provider.Tile` implementations may expose
`PixelSize() (width, height uint)` for other dimensions, including rectangular
tiles. Zero dimensions are rejected.

`!SCALE_DENOMINATOR! = pixel width * meters per CRS unit / 0.00028`,
using the OGC standard rendering pixel of 0.28 mm:

- Projected CRSs use the projection engine's linear unit conversion:
  meters = 1, international feet = 0.3048, US survey feet = 1200/3937,
  and other supported PROJ.4 `+units` / `+to_meter` definitions.
  This is **projected map scale**, not geodesic ground scale; no local
  projection-distortion correction is applied. EPSG:3857 retains its
  previous SQL values byte for byte with the default tile size.
- EPSG:4326 (including HANA's planar-equivalent 1000004326) and supported
  geographic `crs_defn` definitions use degrees
  for pixel dimensions. Meters per longitude degree use the source ellipsoid's
  local parallel-arc factor `N(phi) * cos(phi) * pi / 180`, with
  `N(phi) = a / sqrt(1 - e^2 * sin^2(phi))`. Here `a` is the source semi-major
  axis, `e^2` its squared eccentricity, and `phi` the latitude obtained by
  transforming the original tile center to the layer CRS. Geographic definitions
  with `+towgs84` therefore use their own ellipsoid and source latitude.
  This is a local horizontal scale, representative at the center of a large
  tile; it does not measure a finite geodesic or a diagonal. Exact pole latitudes
  are rejected because longitude is singular there.
- An unknown projection/unit definition or an invalid/non-finite extent
  causes a clear layer-scoped query error **only when a scale token occurs
  in executable SQL**. Database-only SRS definitions must also be known to
  Tegola to use these tokens. Geographic `+proj=longlat` definitions support
  WGS84 identity and the Go fork's three-/seven-parameter datum transformations;
  see the [CRS contract](../../docs/crs.md#geographic-proj-definitions) for
  supported parameters and the remaining registration restrictions.

The normal tile extent is EPSG:3857. Custom tiles already in the layer CRS
are also supported; other tile-to-layer CRS combinations return an explicit
error. No token-looking text in quoted SQL, identifiers, or comments is
evaluated or rewritten. Startup inspection still uses representative probe
values, not a requested tile's scale.

**Compatibility:** non-WebMercator layers now receive source-unit pixel
dimensions and a CRS-aware scale denominator, replacing the old
WebMercator-meter values. SQL thresholds written around the old values
may need adjustment. All four SQL providers share this scale contract through
`provider.TileScale`; HANA additionally normalizes its planar-equivalent SRIDs.

### HANA-specific restrictions

- **Synthetic CRS** (from `crs_defn` at either level, or from a MOS
  `MapplGIS LayerInfo projection`): HANA's native `ST_Geometry` handling
  cannot be used with a synthetic SRID, so such layers must set
  `geometry_format` to `wkb`, `wkt` or `mos`. `mvt_hana` does not support
  synthetic CRS at all.
- **Planar-equivalent SRIDs**: HANA distinguishes round-earth and planar SRS.
  A round-earth SRS is queried through its planar equivalent internally
  (`PLANAR_SRID_OFFSET = 1000000000`); these internal SRIDs must not be
  confused with Tegola's synthetic SRIDs (`>= 340000001`).
- **MOS layers** do not use the database spatial index; filtering is done by
  the in-memory bounding-box check (see
  [docs/geometry-formats.md](../../docs/geometry-formats.md)).

The common provider contract (CRS precedence, `!BBOX!` semantics, startup
inspection, MOS system-info auto-configuration) is documented in
[docs/provider-contract.md](../../docs/provider-contract.md).

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
Testing is designed to work against a live SAP HANA database. To see how to set up a database check this [github actions script](https://github.com/go-spatial/tegola/blob/master/.github/workflows/on_pr_push.yml). To run the HANA tests, the following environment variables need to be set:

```bash
$ export RUN_HANA_TESTS=yes
$ export HANA_CONNECTION_STRING="hdb://myuser:mypassword@something.hanacloud.ondemand.com:443?TLSInsecureSkipVerify"
```

### SQL token lexical rules

SQL tokens use HANA lexical rules: single-quoted strings, double-quoted
identifiers, `--` comments, and block comments are protected. Backslashes are
literal characters; PostgreSQL dollar quoting and MySQL backticks/hash comments
are not treated as HANA quoting.

For MOS startup metadata checks, automatic sample limits and the distinction
between format and geometry-class inference, see the
[shared sampling contract](../../docs/provider-contract.md#mos-registration-sampling).

## Feature filtering and queryables

The optional typed filter capability publishes directly selected public
`TINYINT`, `SMALLINT`, `INTEGER`, `BIGINT`, `DECIMAL(p,s)` with `p <= 38`, native
`BOOLEAN`, and `NVARCHAR` columns. The initial admitted server revision is
`2.00.088.00.1760424921`. Other revisions and unproved column profiles retain
their existing Core capability without advertising filtering. Floating-point,
date/time, `CHAR`/`VARCHAR`, binary and LOB columns are omitted from queryables.
Temporal Unix epoch columns remain integer queryables when publicly selected.

All six scalar comparisons and `IS NULL` use physical column lineage from the
frozen catalog. NULL remains UNKNOWN under comparisons and logical operators.
Numeric literals are evaluated exactly; fractional and out-of-domain bounds are
quantized or folded without multiplying source values or changing NULL behavior.
DECIMAL result descriptors must match the locked catalog precision and scale;
the driver fixed-decimal wire format must also have sufficient coefficient
capacity. Generic decimal transport is limited to its 34-digit coefficient.

String comparisons use the column's UTF-8 bytes and a bound binary literal, so
case, combining sequences, supplementary characters, NUL and trailing spaces
remain significant. No collation-dependent text comparison or literal text cast
is used. BOOLEAN ordering is `false < true`. Selected public SQL NULL properties
remain present with a nil value; private and unselected properties remain absent.

Filters are bound before logical paging and counting within the existing single
RW/Repeatable Read transaction and exclusive source lock. Catalog revalidation,
the earliest request/default 30-second deadline, cancellation of the owned
connection, and unconditional physical connection discard still apply. See the
[feature query profile](FEATURE-QUERY.md) for source protection and cooperative
callback limits, and the [provider contract](../../docs/provider-contract.md) for
the shared typed filter interface.
