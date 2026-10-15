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
- `srid` (int): [Optional] the SRID of the geometries in the GeoPackage. When set explicitly it wins over the value inferred from `gpkg_contents.srs_id` or the geometry headers. Defaults to Web Mercator (3857) when nothing can be inferred.
- `crs_defn` (string): [Optional] a full PROJ.4 coordinate system definition used instead of a numeric `srid` (see below). Wins over `srid` at the same level.

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
- `srid` (int): [Optional] layer-level SRID override. Wins over the provider-level value and over the SRID inferred from the GeoPackage.
- `crs_defn` (string): [Optional] layer-level full PROJ.4 definition used instead of a numeric `srid`. Wins over `srid` at the same level.
- `geometry_fieldname` (string): [Optional] the name of the geometry field. defaults to `geom`. Note: for layers backed by a GeoPackage `tablename` (native `gpkg` format), the geometry column is taken from `gpkg_geometry_columns` and this setting is ignored. Use a custom `sql` layer (or a raw `geometry_format`) if you need to point at a different geometry column.
- `sql` (string): [*Required] custom SQL to use. Required if `tablename` is not defined. Supports the following WHERE-clause tokens:
  - `!BBOX!` - spatial filter token; requirements depend on the effective geometry format (see [the shared contract](../../docs/provider-contract.md)). For native indexed GeoPackage SQL, the recommended pattern joins `rtree_feature_table_geom` on the actual feature ID and selects its `minx`, `miny`, `maxx`, `maxy` bounds. This is an indexed example, not a requirement to join an RTree for every custom query. Raw WKB/WKT rules and missing-bounds fallback are documented below.
  - `!ZOOM!` - [Optional] will be replaced with the "Z" (zoom) value of the requested tile.
  - `!X!` - [Optional] will be replaced with the "X" value of the requested tile.
  - `!Y!` - [Optional] will be replaced with the "Y" value of the requested tile.
  - `!Z!` - [Optional] will be replaced with the "Z" value of the requested tile.
  - `!SCALE_DENOMINATOR!` - [Optional] scale denominator, assuming 90.7 DPI (i.e. 0.28mm pixel size).
  - `!PIXEL_WIDTH!` - [Optional] the unbuffered pixel width in the resolved layer CRS units.
  - `!PIXEL_HEIGHT!` - [Optional] the unbuffered pixel height in the resolved layer CRS units.
  - `!ID_FIELD!` - [Optional] the ID field name.
  - `!GEOM_FIELD!` - [Optional] the geometry field name.
  - `!GEOM_TYPE!` - [Optional] the geometry type if known, otherwise an empty string.

  The registration probe always executes custom SQL without a spatial filter:
  `!BBOX!` is neutralized to `1=1`, position / zoom tokens (`!X!`, `!Y!`,
  `!Z!`, `!SCALE_DENOMINATOR!`, `!PIXEL_WIDTH!`, `!PIXEL_HEIGHT!`) are
  permissive, and the probe is capped at 16 sample rows. The layer is
  registered with its configured CRS. Scale tokens use the shared
  [CRS-aware scale contract](../../docs/crs.md).

  `*Required`: either the `tablename` or `sql` must be defined, but not both.

**Example minimum custom SQL config**

```toml
[[providers.layers]]
name = "a_points"
sql = "SELECT fid, geom, amenity, religion, tourism, shop, si.minx, si.miny, si.maxx, si.maxy FROM land_polygons lp JOIN rtree_land_polygons_geom si ON lp.fid = si.id WHERE !BBOX!"
```

### Custom coordinate systems (`crs_defn`)

When the GeoPackage stores geometries in a system that has no EPSG code (or
you prefer not to rely on one), a full PROJ.4 definition can be provided as
`crs_defn` instead of a numeric `srid`, at provider or layer level. The
definition is passed to proj directly and validated at startup; it is
registered under a synthetic internal SRID (≥ 340000001) and used for both
`!BBOX!` reprojection and geometry conversion to Web Mercator. A `crs_defn`
always wins over a numeric `srid` configured at the same level.

```toml
[[providers]]
name = "sample_gpkg"
type = "gpkg"
filepath = "/path/to/my/sample_gpkg.gpkg"
# provider-level default for all layers:
crs_defn = "+proj=merc +lon_0=0 +k_0=1 +x_0=0 +y_0=0 +ellps=WGS84 +datum=WGS84 +units=m +no_defs"
```

Use `+proj=etmerc` instead of `+proj=tmerc` for transverse Mercator
definitions (vendored proj limitation). Note the RTree spatial index bounds
must be stored in the same CRS as the geometries for `!BBOX!` filtering to
work correctly.

### Common geometry / CRS options

The GPKG provider implements the common geometry contract documented in
[docs/provider-contract.md](../../docs/provider-contract.md):

- `geometry_type` (string): [Optional, **layer level only**] explicit layer
  geometry type
  (`Point`, `LineString`, `Polygon`, `MultiPoint`, `MultiLineString`,
  `MultiPolygon`, `GeometryCollection`). Skips geometry-class inference (empty data is allowed); automatic MOS format
  detection requires three valid samples, or one in a completed short result. Structural
  validation of custom SQL still runs. Mixed content is permitted with a
  one-time warning.
- `geometry_format` (string): [Optional] `gpkg` (GeoPackage native binary,
  the default), `wkb`, `wkt` or `mos`. Valid at provider level (defaults for
  all layers) and at layer level (overrides). With `gpkg` the GeoPackage
  binary header is parsed and the embedded WKB body is decoded; with `mos`
  the header is skipped and the raw MOS payload is used directly.
- `mos_precision` (int): [Optional] decimal digits carried by MOS
  coordinates. Only applies when the effective geometry format is `mos`.
  Valid at provider and layer level.
- `mos_units` (string): [Optional] packed linear unit of MOS coordinates
  (`mm`, `cm`, `dm`, `m` or `km`). Only applies when the effective geometry
  format is `mos`. Valid at provider and layer level.

```toml
[[providers]]
name = "sample_gpkg"
type = "gpkg"
filepath = "/path/to/my/sample_gpkg.gpkg"
geometry_format = "mos"
mos_precision = 2
mos_units = "m"

[[providers.layers]]
name = "lines"
tablename = "lines"
geometry_type = "LineString"
```

### Raw tables

A layer with `geometry_format` set to a raw format (`wkb`, `wkt` or `mos`) or
a raw table that has no `gpkg_contents` / `gpkg_geometry_columns` metadata
does not need GeoPackage metadata at all: the table is registered from
`PRAGMA table_info` and the RTree spatial index is **not** used. Every
`tablename` layer — including raw tables missing from the GeoPackage
metadata — is first checked against the shared structural MapplGIS contract
(see [docs/provider-contract.md](../../docs/provider-contract.md)); a
detected MapplGIS table is served via the `mos` format automatically, with
no explicit `geometry_format` needed. A plain raw table (not detected as
MapplGIS) that is missing from the GeoPackage metadata **requires** an
explicit raw `geometry_format` — without one registration fails with a
`table does not exist` error, because the native `gpkg` path looks the table
up in `gpkg_geometry_columns` and will not find it. The `!BBOX!`
token still works, but rows are filtered in memory, so performance depends on
table size.

**Performance warning:** without an RTree join every query scans the table
and decodes all candidate geometries. For large tables prefer the native
`gpkg` format (which joins the RTree index table named `rtree_` + table name
+ `_` + geometry column name, e.g. `rtree_land_polygons_geom` for table
`land_polygons` with geometry column `geom`), or declare per-row bounds
columns so the coarse `!BBOX!` filter can be applied by SQLite.

If the table has numeric bounds columns named (case-insensitively)
`minx`, `maxx`, `miny` and `maxy`, the provider detects them and applies a
coarse SQL-level `!BBOX!` filter before in-memory refinement. The units of
the stored bounds values must match what the provider compares against:

* for `wkb` / `wkt` layers the bounds columns hold plain layer-CRS
  coordinates (the same units as the decoded geometries);
* for `mos` layers the bounds columns hold the quantized raw MOS values as
  stored in the database (`raw value = coordinate * 10^mos_precision /
  unitFactor`, e.g. millimetres with `mos_precision = 0` and
  `mos_units = "mm"`), because the extent is scaled by `rawScale` before the
  comparison.

The bounds column names are configurable with the common
`bbox_minx_fieldname` / `bbox_maxx_fieldname` / `bbox_miny_fieldname` /
`bbox_maxy_fieldname` keys (layer level overrides provider level, per
field; defaults `MINX`/`MAXX`/`MINY`/`MAXY`). Resolved bounds columns are
excluded from feature tags.

For custom SQL with `geometry_format = "mos"` the `!BBOX!` token is
**required** and expands into the bounds-columns predicate over the
configured bounds fields (with MOS raw scaling); the provider verifies at
registration that the token is present. The bounds columns do not need to
appear in the SELECT list — the predicate resolves them in the query's own
scope (e.g. the source table's columns). When they are present in the result
their actual names are used; when absent, registration logs a warning and
the layer uses the resolved names (layer > provider > `MINX`/`MAXX`/`MINY`/
`MAXY`). Custom SQL with the native `gpkg`
binary format may use `!BBOX!` as a source-CRS bounds comparison over the
same columns (no MOS scaling). Bounds-backed MOS custom SQL must configure
`srid` or `crs_defn` explicitly (a missing CRS is a startup error);
`mos_precision` and `mos_units` are optional with the normative paired
defaults.

Example table layout for a `wkb` layer:

```sql
CREATE TABLE land_polygons (
  fid INTEGER PRIMARY KEY,
  geom BLOB,
  minx REAL, maxx REAL, miny REAL, maxy REAL
)
```

```toml
[[providers.layers]]
name = "land_polygons"
tablename = "land_polygons"
id_fieldname = "fid"
geometry_format = "wkb"
```

For raw custom-SQL layers `geometry_fieldname` and `id_fieldname` must point
at existing columns; if the configured `id_fieldname` does not exist the
table primary key is used as the id field with a warning, while a missing
geometry column is a startup error.

### Empty layers

If a configured custom-SQL layer currently returns no rows, Tegola logs a
warning and keeps the layer registered without an inferred geometry type. This
allows data to appear later without requiring a restart. A tile request still
executes the configured SQL normally.
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

SQL tokens use SQLite lexical rules. Backslashes in strings are literal;
double-quoted, bracketed, and backtick identifiers are protected. PostgreSQL
dollar quoting and MySQL hash comments are not interpreted as SQLite quoting.

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

## Feature mutations

Feature writes are opt-in and apply only to separately admitted direct tables.
Per-collection operations, authentication and conditional revisions are configured
under `[features.write]`; WFS transactions use the same mutation coordinator.
Create requires a database-generated key. Read/tile support alone does not admit
writes, and custom tile SQL does not supply a writable table mapping.

The MOS write profile maintains four resolved `bbox_*_fieldname` columns in the
same transaction as encoded geometry. Attribute-only updates preserve geometry,
opaque annotations and bounds. An admitted custom `etmerc` source projection is
converted using explicit datum semantics. Detected canonical MapplGIS tables have
narrower update/delete rules and do not admit creation. See
[geometry write limits](../../docs/geometry-formats.md#mos-writes-with-separate-bounds-columns),
[write configuration](../../docs/configuration.md#explicit-wfs-and-write-publication)
and [provider validation](../../docs/provider-matrix.md).

## Raw feature queries

The optional `FeatureQuerier` capability queries table-backed layers with a
schema-proven single-column unique `INTEGER` identity. Existing tile layers
remain available when this feature profile is unsupported. Custom SQL is not
eligible for raw feature publication. IDs above SQLite's signed integer range
produce no match; they are never converted to negative IDs.

Queries use a read transaction and ordered candidate chunks. Exact geometry
intersection precedes logical offset and limit; one additional exact match
establishes `HasMore`. An exhausted query can report an exact matched total;
otherwise that total remains unknown. NULL and decoded empty geometries match
spatial constraints; NULL identities and MOS metadata do not invent features.
Malformed feature geometry fails even for auto-detected MOS, independently of
the legacy tile warning/skip policy.

### Temporal geometry

Configure either an instant or two interval endpoint fields on a provider layer:

```toml
temporal_field = "observed_at"
temporal_storage = "unix_milliseconds"
# Alternatively:
# temporal_start_field = "valid_from"
# temporal_end_field = "valid_until"
```

Supported storage profiles are `unix_seconds`, `unix_milliseconds`,
`unix_microseconds`, and `unix_nanoseconds`. Values must be SQLite INTEGERs;
TEXT/REAL values are not coerced. Field mappings resolve during registration,
independently of property projection. Missing, mixed, partial or duplicate
endpoint mappings fail registration. Explicit temporal configuration on custom
SQL is unsupported. No temporal keys declares absent temporal geometry.

Intervals intersect inclusively. A NULL endpoint is open; both NULL endpoints
mean absent temporal geometry and match valid temporal queries. Reversed source
intervals fail validation. Exact integer bound conversion preserves negative
epochs, nanosecond boundaries and query dates outside the storage range.

### Spatial query limits

Raw feature layers support `spatial_dimension = "xy"`, `"xyz"`, or
`"mixed_xy_xyz"`. Raw WKB/WKT default to XY. Explicit XYZ/mixed configuration
requires `vertical_crs = "http://www.opengis.net/def/crs/OGC/0/CRS84h"`, declaring
WGS84 ellipsoidal height in metres. Native GeoPackage derives dimension from
`gpkg_geometry_columns.z` (0/1/2); an explicit dimension must agree. Missing or
invalid native dimensional schema is an unsupported feature capability. Native
inferred XYZ without a vertical binding remains usable through the legacy tile
path. Mandatory measures are unsupported; optional measures do not reject rows
without M. MOS remains XY.

The raw codec supports base XY and ISO-WKB Z families and explicit WKT Z. It
retains Z and checks encountered bodies against the declared profile. Native
NONE/XY envelopes permit XYZ bodies; XYZ envelopes require Z bodies. XYM/XYZM
envelopes, actual M/ZM and EWKB profiles are unsupported. The native empty flag
must agree with a complete decoded empty body; a header alone is malformed in
raw queries. The legacy tile decoder is separate and retains its prior policy.

Six-coordinate bounds test XYZ geometry against a closed 3D box, including
segment correlation and planar polygon surfaces with holes. Nonplanar surfaces
are unsupported. XY members have an unconstrained vertical dimension and use
exact horizontal intersection; they retain XY coordinates without invented
height. Horizontal queries validate and retain source Z while selecting the XY
projection. Exact predicates precede pagination and counts.

XYZ/mixed transformations admit canonical WGS84 EPSG:4326, EPSG:3857 and northern
and southern WGS84 UTM zones only. Custom/synthetic source definitions and other
vertical references are unsupported. Converters are independently owned and
do not consult mutable projection registrations. Transformed vertices define
the query-frame geometry; a polygon made nonplanar by projection is unsupported.

Same-CRS queries use available RTree/raw bounds for ordinary candidates and
separate conservative branches for absent or unclassifiable geometry. The
absence branch may scan source metadata. Native/WKB Polygon, Multi and
GeometryCollection payloads are conservatively inspected outside their stored
bounds when SQL cannot prove they are nonempty; MOS non-Point payloads have the
same limitation. This is not a claim that every geometry family avoids scanning.
ISO-WKB Z families currently use the conservative binary classification path,
so XYZ points/lines can also require inspection outside stored bounds. Raw WKB
all-NaN points are absence candidates even with misleading bounds.
Cross-CRS bounds use ordered source chunks and exact transformed geometry
matching; no heuristic inverse envelope is used to discard candidates.

The SQL predicate profile accepts at most 512 requested IDs and 128
ordinary bounds. Unsupported profiles/ranges preserve `errors.Is(ErrUnsupported)`;
field selection is restricted to resolved public properties. No feature query
calls `TileFeatures` or constructs a tile.

### Optional scalar filtering

Ordinary table feature profiles can expose integer declarations (including
`MEDIUMINT`, `INT2` and `INT8`), `BOOLEAN`/`BOOL`, numeric declarations
(`REAL`, `DOUBLE`, `DOUBLE PRECISION`, `FLOAT`, `NUMERIC`, `DECIMAL`), and text
declarations (`TEXT`, `CHAR`, `CHARACTER`, `VARCHAR`, `NCHAR`, `NATIVE CHARACTER`,
`NVARCHAR`, `CLOB`). Declaration case and size parameters are normalized. Names
are exact public column names; identity, geometry, bounds and private fields
are excluded. Date/time and other declarations are not advertised. Optional
catalog admission failure preserves Core queries and tile rendering; custom
feature SQL remains unsupported.

Filtered requests verify frozen table declarations and database encoding in
their read snapshot, then check every advertised scalar domain with SQL before
applying the client predicate. INTEGER values must have SQLite integer storage;
booleans must be integer 0 or 1. Numbers require integer or finite REAL storage.
TEXT requires a UTF-8 database, valid UTF-8 bytes
and at most **1 MiB per source cell**. NULL remains NULL. This integrity check
can scan the source table even when the requested filter matches no rows.
It does not materialize the table in Go. A provider-owned SQLite driver installs
a fixed, bounded UTF-8 validator on every connection; tile and unfiltered query
paths retain their existing behavior.

String comparisons use UTF-8 byte ordering with significant trailing spaces;
numeric comparisons preserve exact numeric literals and SQL three-valued logic.
Integer storage uses exact integer bounds; REAL storage uses directed float64
bounds without silently rounding the literal.
The SQL filter is combined with bbox/datetime/identity predicates before exact
geometry matching, counts and pagination. These capabilities do not themselves
declare an OGC conformance class.
