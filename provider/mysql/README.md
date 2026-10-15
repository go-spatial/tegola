# MySQL / MariaDB Provider

The `mysql` provider serves MVT tiles from MySQL/MariaDB spatial tables and explicitly configured raw geometry tables. MySQL 5.5/InnoDB raw MOS feature reads and writes use the opt-in legacy identity profile; this does not add modern native spatial functions to MySQL 5.5. It supports both the `tablename` and custom `sql` layer configuration modes, plus the full set of SQL tokens available in the postgis provider.

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

The opt-in feature API uses an independent source profile. A tile `sql` statement
does not establish a raw feature source. Use an ordinary `tablename`, or supply
`feature_sql` for a constrained selection over one physical table:

```toml
feature_sql = "SELECT id, geom, name, start_time, end_time FROM parcels WHERE active = 1"
temporal_start_field = "start_time"
temporal_end_field = "end_time"
temporal_storage = "unix_nanoseconds"
spatial_dimension = "xy"
```

`feature_sql` accepts direct column projections, aliases, and a bounded predicate
grammar. Joins, expressions, functions, macros, parameters, comments, and multiple
statements are unsupported. Predicate literals are bound values; integer and
decimal comparisons retain their exact catalog types. Float predicates and text
collations outside the supported binary UTF-8/ASCII profile are unsupported.
Malformed configuration fails registration; unsupported source capabilities
leave the tile layer available and cannot publish a feature collection.

The source must be an InnoDB base table with a proven single-column unique
integer ID. Unsigned IDs retain all 64 bits; NULL IDs are excluded. Computed or
generated ID, geometry, and temporal columns are unsupported. Stored
auto-increment IDs are admitted with the same uniqueness proof. Registration
freezes columns, indexes, CRS and the engine's physical `TABLE_ID`. Queries check
the snapshot against that metadata, including table replacement, before delivery.
The modern profile needs visibility of the relevant InnoDB catalog, including the
documented `PROCESS` requirement; the provider does not grant privileges. The
explicit MySQL 5.5 legacy profile instead uses a weaker metadata identity; see
[legacy admission](../../docs/configuration.md#mysql-55-table-identity). See the
[MySQL InnoDB catalog](https://dev.mysql.com/doc/mysql-infoschema-excerpt/8.0/en/information-schema-innodb-tables-table.html)
and [MariaDB InnoDB catalog](https://mariadb.com/docs/server/reference/system-tables/information-schema/information-schema-tables/information-schema-innodb-tables/information-schema-innodb_sys_tables-table).

Each request uses one repeatable-read transaction and bounded ID chunks. The
modern profile marks the transaction read-only; MySQL 5.5 lacks that transaction
syntax, so the admitted legacy profile executes only the same SELECT queries
without requesting the unsupported access mode. Strict decoding and exact spatial/temporal predicates precede offset,
limit and matched counts. Candidate scanning can cover the entire selected
source. Same-definition MOS queries can prune candidates with an admitted
four-column bounds mapping; other profiles or cross-CRS requests can require
ordered scans. Exact geometry matching remains authoritative. A
partially consumed result can report an unknown matched total. Callback errors
and cancellation stop delivery without retries.

Ordinary `fields` defines the public property subset; omitted or empty `fields`
selects eligible columns. Custom SQL projections define public property labels.
ID, geometry, configured bounds, and physical `min_zoom`/`max_zoom` columns remain
private through aliases. Temporal columns are read privately when an ordinary
field subset excludes them; explicitly projected custom temporal columns are
public. Decimal properties remain exact JSON numbers, binary properties use
base64, and unsupported or nonfinite property values fail strict decoding.

Explicit raw WKB/WKT supports `xy`, `xyz`, and `mixed_xy_xyz` source profiles.
XYZ/mixed requires `vertical_crs = "CRS84h"`, WGS84 ellipsoidal heights in metres,
and a supported horizontal conversion preserving that height reference. MOS is
XY. Native MySQL/MariaDB geometry is admitted only with the checked server/schema
XY profile and immutable source CRS; each native row's SRID is checked. The
initial native MariaDB profile admits exact version 10.7.4 and the 10.11/11.4
branches. The immutable 10.7.4
[storage definitions](https://raw.githubusercontent.com/MariaDB/server/mariadb-10.7.4/sql/spatial.h),
[geometry implementation](https://raw.githubusercontent.com/MariaDB/server/mariadb-10.7.4/sql/spatial.cc), and
[WKB/SRID functions](https://raw.githubusercontent.com/MariaDB/server/mariadb-10.7.4/sql/item_geofunc.cc)
establish XY native storage, WKB export preserving coordinate order, and SRID
label extraction. This admission does not imply a native XYZ or height conversion
profile. The published
[10.11](https://github.com/MariaDB/server/blob/10.11/sql/spatial.h) and
[11.4](https://github.com/MariaDB/server/blob/11.4/sql/spatial.h) storage definitions
describe two-coordinate native points. Other MariaDB versions remain unsupported
for native feature queries pending separate evidence. Native
exports use WKB, with explicit longitude/latitude axis order when MySQL requires
it. NULL and decoded empty geometry become absent geometry; malformed geometry
fails the request. Native storage rejection of corrupt input is separate from
raw decoder/query evidence.

Temporal fields use signed integral Unix seconds, milliseconds, microseconds or
nanoseconds. Instant mappings use `temporal_field`; interval mappings require
both start and end fields. NULL interval endpoints are open; reversed intervals
fail decoding. Exact request fractions and supported leap-second constraints are
handled before paging, without rounding stored values into a different interval.

### Feature test evidence

Package tests include driver-level lifecycle and schema guards. These do not
establish live database parity. Real fixtures are opt-in:

```powershell
$env:RUN_MYSQL_TESTS = 'yes'
$env:MYSQL_FEATURE_TEST_DSN = '<explicit TCP DSN>'
# Or RUN_MARIADB_TESTS=yes and MARIADB_FEATURE_TEST_DSN.
go test -mod=vendor -count=1 -run TestFeatureLive ./provider/mysql
```

Fixtures create and remove isolated tables and require catalog visibility. When
a gate is disabled, the live suite reports a skip. When enabled, unavailable
servers, incorrect flavor, missing permissions or unexpected native error codes
fail the fixture. Official feature conformance and live backend parity require
their separate acceptance evidence.

## Tile configuration

The provider keeps at most `max_connections` open connections, reuses the same
number of idle connections, and retires idle connections after five minutes or
any connection after thirty minutes. Tile queries use `QueryContext` and are
retried up to two times when the driver reports a broken connection. Features
are held until the complete result set has been read, so a retry after a
mid-stream connection failure cannot emit duplicates.

The connection DSN is built with the go-sql-driver's `Config.FormatDSN`, so
user/password/database values with special characters are escaped correctly.
The config starts from the driver's `NewConfig()` defaults, so the standard
authentication plugins (`mysql_native_password`, `caching_sha2_password`) stay
allowed exactly as the upstream driver permits them.
`multiStatements` is deliberately **not** enabled: layer SQL must be a single
statement.

```toml
[[providers]]
name = "mysql_provider"
type = "mysql"
host = "localhost"
port = 3306                 # optional, default 3306
database = "gis"
user = "user"
password = "password"
srid = 3857                 # optional, default 3857
geometry_format = "auto"    # optional: auto (default) | mysql | mariadb | wkb | wkt | mos
mos_precision = 2           # optional, MOS format only: decimal digits of quantized int coords; default depends on mos_units (mm→0, cm→1, dm→1, m→2, km→5)
mos_units = "m"             # optional, MOS format only: mm | cm | dm | m | km, default m
max_connections = 100       # optional, default 100
# tls = "preferred"         # optional, go-sql-driver TLS config name: true | false | preferred | skip-verify | <registered tls.Config name>
# timeout = "10s"           # optional, dial timeout; Go duration string ("500ms", "10s") or integer seconds; default: driver default (no timeout)

[[providers.layers]]
name = "buildings"
tablename = "buildings"
# id_fieldname = "fid"      # optional, default "fid"
# geometry_fieldname = "geom" # optional, default "geom"
# fields = ["height", "name"]
# srid = 4326               # optional, layer-level override (see "SRID handling")
# crs_defn = "+proj=longlat +ellps=WGS84 +datum=WGS84 +no_defs" # optional, wins over srid
# geometry_format = "wkb"   # optional, layer-level override: auto | mysql | mariadb | wkb | wkt | mos (overrides the provider-level value for this layer)
# mos_precision = 2         # optional, layer-level override, MOS format only
# mos_units = "mm"          # optional, layer-level override: mm | cm | dm | m | km

[[providers.layers]]
name = "landmarks"
sql = "SELECT fid, geom, name FROM landmarks WHERE geom && !BBOX! AND zoom_level >= !ZOOM!"
```

## Common geometry / CRS options

The MySQL/MariaDB provider implements the common geometry contract documented
in [docs/provider-contract.md](../../docs/provider-contract.md). Alongside the
format keys above, the common `geometry_type` layer key is supported: an
explicit value (`Point`, `LineString`, `Polygon`, `MultiPoint`,
`MultiLineString`, `MultiPolygon`, `GeometryCollection`) fixes the layer
geometry type before any data is read and skips geometry-class inference (empty data is allowed); structural validation
of custom SQL still runs.
It is orthogonal to MapplGIS table detection: a `tablename` layer is still
checked for the MapplGIS signature and still receives system-info
configuration when detected. Mixed content is permitted with a one-time
warning.

```toml
[[providers.layers]]
name = "lines"
tablename = "lines"
geometry_type = "LineString"
```

## Geometry formats

MySQL and MariaDB store geometry columns in different *native internal layouts*, which this provider handles explicitly:

| Server | Layout |
|---|---|
| MySQL | `[4B SRID little-endian][1B byte-order][WKB type + body]` |
| MariaDB ≤ 10.6 | `[1B byte-order][4B SRID in that byte order][WKB type + body]` — note the SRID comes **after** the byte-order marker |
| MariaDB 10.7+ | same as above, but the top 3 bits of the SRID carry axis-order flags for reference geometries (masked off automatically) |

The `geometry_format` config option controls decoding. It can be set at the
provider level (a default for all layers) and overridden per layer with the
same value set:

- `auto` (default) — the provider runs `SELECT VERSION()` at startup to detect whether the server is MySQL or MariaDB and uses the matching native layout. If a blob does not fit the native layout, plain WKB decoding is attempted as a fallback (covers values already converted with `ST_AsBinary()` in custom SQL).
- `mysql` — force the MySQL native layout.
- `mariadb` — force the MariaDB native layout (handles 10.7+ axis-order flag bits).
- `wkb` — expect plain WKB with no header (e.g. when the layer selects `ST_AsBinary(geom) AS geom`).
- `wkt` — expect WKT text (e.g. a `LINESTRING(...)` stored in a TEXT column). No SRID is decoded; the configured layer/provider SRID applies.
- `mos` — expect the packed binary geometry format written by MapplGIS, typically a `LONGBLOB LINE` column. Coordinates are quantized int32 pairs; `mos_precision` (optional) is the number of decimal digits they carry and `mos_units` (optional) their packed linear units (`mm`, `cm`, `dm`, `m`, or `km`, default `m`; the default `mos_precision` is paired with the units: `mm`→`0`, `cm`→`1`, `dm`→`1`, `m`→`2`, `km`→`5` via `DefaultMOSPrecisionForUnits`). After dequantization, coordinates are converted to metres using the corresponding factor (`mm` → `0.001`, `cm` → `0.01`, `dm` → `0.1`, `m` → `1`, `km` → `1000`) before SRID reprojection. MOS carries no CRS — the configured layer/provider SRID applies (or the layer's own system info blob, see below). Because the blob is opaque, the provider uses indexed `MINX`/`MAXX`/`MINY`/`MAXY` columns as a coarse bounding-box `!BBOX!` filter in the raw MOS units, then applies the decoded geometry's bounding-box intersection check in Go. Automatic-format tile decoding can warn and skip malformed MOS rows; explicit-format decoding and raw feature queries keep their strict error policy.

### Geographic SRIDs and MySQL axis order

MySQL 8 interprets geometry values for geographic SRIDs (e.g. `4326`)
latitude-first according to its SRS metadata, while tegola writes WKT and
`!BBOX!` polygons longitude-first. For `wkt`/`wkb` layers on MySQL with a
geographic SRID the provider therefore builds
`ST_GeomFromText(value, srid, 'axis-order=long-lat')` /
`ST_GeomFromWKB(value, srid, 'axis-order=long-lat')` (both for the geometry
column and the tile bbox polygon) so the filter and the stored data agree.
MariaDB does not support the options argument and projected SRIDs have no
axis order — both keep the plain constructor form. (MySQL 5.7 does not
accept the options argument either; use a projected SRID or MariaDB there.)

Geometry *reads* are decoded in Go from the raw column values and are
unaffected. Custom SQL that returns `ST_AsBinary(geom)` for a geographic
SRID should pass the same option — `ST_AsBinary(geom, 'axis-order=long-lat')`
— so the emitted WKB is longitude-first like everything tegola consumes.

## MOS geometry format

The native MOS blob layout (little-endian) starts with a 10-byte geometry
prefix (object type, subobject count and total point count), then one `uint32`
point count per subobject, followed by all points as contiguous `(int32 x,
int32 y)` pairs. Tegola also accepts the older 12-byte fixture form with an
optional `uint16` flags word before the subobject counts. Real coordinates are
`int / 10^mos_precision`. Object types map to MVT geometries as:

- polygon → `Polygon`/`MultiPolygon` (rings are classified into exteriors and holes by containment, so an island inside a lake hole becomes a second polygon)
- polyline → `LineString`/`MultiLineString`
- point → `Point`/`MultiPoint`
- text/image → anchor `Point`

### MapplGIS table detection (auto-configuration)

MapplGIS tables are detected once, at registration, by their **structure**
(table-canonical MapplGIS detection). This path never scans sample rows and
applies only to `tablename` layers; custom `sql` layers instead use the
separate SQL-sample storage detection (see below). A `tablename` layer is
recognized as a MapplGIS table only when all of the following hold (matched
case-insensitively):

- the table has all nine required columns: `OKEY`, `MUID`, `MINX`, `MAXX`,
  `MINY`, `MAXY`, `ObjectStyle`, `ObjectType`, `LINE`;
- `OKEY` is the table's primary key;
- all six required indexes exist over `MUID`, `MINX`, `MAXX`, `MINY`, `MAXY`,
  `ObjectType`;
- a single point probe (`OKEY = 1 AND LINE IS NOT NULL`) returns a decodable
  `MapplGIS LayerInfo` system info blob.

When detection succeeds the layer gets `IsMapplGIS=true` and the decoded
system info configures the layer:

- `Precision` — the quantization precision (`kPrecision = 10^Precision`). Applied as the layer's `mos_precision` when neither the provider- nor layer-level `mos_precision` key is set.
- `Projection` — the layer's full PROJ.4 definition. Registered as a synthetic SRID (≥ 340000001, same mechanism as `crs_defn`) and used as the layer SRID when no `srid`/`crs_defn` is configured at provider or layer level. Explicit config values always win.
- `MapUnits` / `flMapUnitsDefined` — when `mos_units` is not configured, the declared unit is converted to a metres factor and applied after dequantization. Supported units are millimetres (`muMm`), centimetres (`muSm`), decimetres (`muDm`), metres (`muM`) and kilometres (`muKm`). Unsupported angular/undefined units are rejected.

In practice this means a MapplGIS table needs no `mos_precision`/`mos_units`/`srid` configuration at all — the layer configures itself from its own system info row, and explicit config keys remain available as overrides. The effective geometry format of a detected table is authoritative `mos`: an explicit non-MOS `geometry_format` on the same layer is a startup conflict error. A detected table also replaces the default `id_fieldname = "fid"` with the contract primary key `OKEY`; an explicitly configured `id_fieldname` is honored.

Table-canonical MapplGIS detection never applies to custom `sql` layers, and
`LayerSystemInfo` is never applied from result rows. Custom `sql` layers are
still subject to SQL-sample storage detection: an `auto` SQL layer
whose registration probe reports the MapplGIS signature is tagged `MapplGIS`
without system info and without applying any projection from the sample. A
MOS custom SQL layer must configure `srid` or `crs_defn` explicitly (a
missing CRS is a startup error); `mos_precision` and `mos_units` are
optional with the normative paired defaults.

## SRID handling

SRID resolution order (highest priority first):

1. Layer-level `crs_defn` config value (full PROJ.4 definition, see below).
2. Layer-level `srid` config value.
3. Provider-level `crs_defn` config value.
4. Provider-level `srid` config value (if explicitly configured).
5. `MapplGIS LayerInfo projection` decoded from the system info blob of a detected MapplGIS table layer (registration-time detection only — never from custom SQL; applied when no `srid`/`crs_defn` is configured at provider or layer level; registered as a synthetic SRID, see [docs/crs.md](../../docs/crs.md)).
6. SRID decoded from the sampled geometry header at registration (native formats only).
7. Default `3857` (Web Mercator).

When the layer SRID differs from Web Mercator, tile bounding boxes are reprojected into the layer SRID before the `!BBOX!` filter is applied, and geometries are delivered to the MVT encoder with their true SRID. The full CRS contract (config keys, precedence, synthetic SRIDs, `!BBOX!` semantics) is documented in [docs/crs.md](../../docs/crs.md) and is shared by all standard providers.

### Deferred custom SQL and mixed SRIDs

Tile-dependent custom SQL (queries containing `!X!`/`!Y!`/`!Z!` and friends) defers geometry inspection to the first query. Without an explicitly configured CRS, the first non-zero native geometry header SRID seen in the results establishes the canonical layer CRS. Rows carrying a *different* header SRID are skipped with a per-row warning instead of being silently interpreted as if they carried the canonical SRID; rows without a header SRID (0) are always processed. Mixed-SRID results are therefore not supported on deferred custom SQL layers — configure `srid`/`crs_defn` explicitly if the source mixes SRIDs.

### Reprojection

Reprojection between the layer SRID and Web Mercator uses the vendored `alexeydott/proj` library. Any SRID can be made available in two ways:

1. **Built-in table** — registered automatically at startup (`basic.RegisterBuiltinProj4SRIDs`), covering ~220 widely used systems:

   | Codes | System |
   |---|---|
   | 32601–32660 | WGS 84 / UTM northern zones |
   | 32701–32760 | WGS 84 / UTM southern zones |
   | 28401–28432 | Pulkovo 1942 / Gauss-Kruger zones (8-digit eastings) |
   | 2463–2491 | Pulkovo 1995 / Gauss-Kruger zones |
   | 2492–2522 | Pulkovo 1942 / Gauss-Kruger zones (500000 false easting) |
   | 3785, 900913 | Web Mercator aliases |
   | 53004 | Sphere Mercator (ESRI) |

2. **`crs_defn` config value** — a full PROJ.4 definition at provider or layer level, registered under a synthetic internal SRID. See the next section and [docs/crs.md](../../docs/crs.md).

Note that only a subset of PROJ.4 operations is implemented by the vendored proj library (`merc`, `utm`, `etmerc`, `aea`, `leac`, `eqc`); transverse Mercator-based systems use `+proj=etmerc`, not `+proj=tmerc`.

### `crs_defn` — textual CRS definition instead of SRID

When a system has no EPSG code (or you don't want to use one), a full PROJ.4
definition can be given as `crs_defn` instead of a numeric `srid`, at either
provider or layer level. The definition is passed to proj directly and
validated with a forward+inverse round trip at startup; it is registered under
a synthetic internal SRID (≥ 340000001) that flows through the regular
reprojection path. A `crs_defn` always wins over a numeric `srid` configured at
the same level; layer-level settings win over provider-level ones.

```toml
[[providers]]
name = "mysql_provider"
type = "mysql"
# ...
# provider-level default for all layers:
crs_defn = "+proj=merc +lon_0=0 +k_0=1 +x_0=0 +y_0=0 +ellps=WGS84 +datum=WGS84 +units=m +no_defs"

  [[providers.layers]]
  name = "tram_sections"
  # layer-level override wins over the provider-level crs_defn:
  crs_defn = "+proj=etmerc +lat_0=0 +lon_0=61 +k_0=1 +x_0=500000 +y_0=0 +ellps=krass +units=m +no_defs"
```

Use `+proj=etmerc` instead of `+proj=tmerc` for transverse Mercator
definitions (vendored proj limitation).

Synthetic SRIDs (the IDs assigned to `crs_defn`) exist only inside Tegola and
are not registered in MySQL or MariaDB. For native geometry columns, Tegola
therefore disables the server-side `!BBOX!` spatial predicate for a synthetic
SRID and performs reprojection and tile filtering in Tegola. This is correct
but can be slower; use a real database SRID when the database has a matching
registered SRS. MOS layers continue to use their indexed raw-bounds filter.

## SQL tokens

The following tokens are supported in custom `sql` (case-insensitive) and behave identically to the postgis provider:

- `!BBOX!` — for native spatial geometries, replaced with `ST_Intersects(<geom_field>, ST_GeomFromText('POLYGON(...)', <layer_srid>))` using the tile's buffered extent in the layer's SRID. WKT columns are wrapped with the same SRID. For MOS, replaced with an indexed bounds-columns overlap predicate in the raw packed coordinate units. For MOS custom SQL the `!BBOX!` token is **required** and expands into the bounds-columns predicate (the bounds column names are configurable via `bbox_minx_fieldname` / `bbox_maxx_fieldname` / `bbox_miny_fieldname` / `bbox_maxy_fieldname`, defaults `MINX`/`MAXX`/`MINY`/`MAXY`); the provider verifies the token at registration. The bounds columns do not need to appear in the SELECT list — the predicate resolves them in the query's own scope (e.g. the source table's columns). When they are present in the result their actual names are used; when absent, registration logs a warning and the layer uses the resolved names (layer > provider > `MINX`/`MAXX`/`MINY`/`MAXY`).
- `!ZOOM!`, `!Z!` — the tile's zoom (Z) value.
- `!X!`, `!Y!` — the tile's X/Y values.
- `!SCALE_DENOMINATOR!` — horizontal scale denominator using the OGC 0.28 mm rendering pixel.
- `!PIXEL_WIDTH!`, `!PIXEL_HEIGHT!` — unbuffered pixel size in the resolved layer CRS units.
- `!ID_FIELD!` — the layer's id field name.
- `!GEOM_FIELD!` — the layer's geometry field name.
- `!GEOM_TYPE!` — the layer's geometry type name (POINT, LINESTRING, ...).

The registration probe always executes custom SQL without a spatial filter:
`!BBOX!` is neutralized to `1=1`, position / zoom tokens (`!X!`, `!Y!`,
`!Z!`, `!SCALE_DENOMINATOR!`, `!PIXEL_WIDTH!`, `!PIXEL_HEIGHT!`) are
permissive, and the probe is capped at 16 sample rows. Tegola decodes the
returned geometries and applies the tile intersection check in memory,
avoiding MySQL spatial functions on an unknown format (including MOS); the
layer is registered with its configured CRS. Scale tokens use the shared
[CRS-aware scale contract](../../docs/crs.md). Table-canonical MapplGIS detection never
applies to custom `sql` layers, and no system-info record is applied from
result rows: a MOS custom SQL layer must configure `srid` or `crs_defn`
explicitly (a missing CRS is a startup error), while `mos_precision` /
`mos_units` are optional with the normative paired defaults.

## Empty layers

Layers (table or custom SQL) that currently return 0 rows produce a warning
and remain registered without an inferred geometry type. They are queried
normally when a later request returns data; an empty layer does not prevent the
provider from starting. Until the first decodable geometry is seen, the layer
uses the same safe in-memory filtering path as custom SQL whose geometry type
is not yet resolved so a later
geometry header can establish the source CRS without an incorrect startup
assumption.

## Raw geometry query performance

Raw geometry cannot directly use a native spatial index. MOS layers can instead
use numeric bounds columns to prune candidates in SQL; raw WKB/WKT wrappers may
apply native functions without an index on the stored bytes. Without selective
SQL predicates, a tile can inspect the full selected source. For MOS, configure
`bbox_minx_fieldname`/`bbox_maxx_fieldname`/`bbox_miny_fieldname`/
`bbox_maxy_fieldname` (precomputed column bounds) and use a bounds-backed
MOS custom query carrying `!BBOX!`, which expands to a server-side
comparison over those columns. Alternatively use a native geometry column
(`geometry_format` unset) so MySQL spatial predicates apply.

## Known limitations

- Only 2D geometries are supported by the decoder for MVT encoding (matching MariaDB's capabilities).
- Custom SQL used in derived-table inspection must be aliasable — the inspection query wraps it as `SELECT geom FROM (<your sql>) AS __tegola_inspection LIMIT 1`.

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

SQL tokens use default MySQL lexical rules, including `#` comments and the
whitespace requirement after `--`. Single/double-quoted strings use backslash
escapes; backtick identifiers use doubled backticks. Token scanning does not
support the nondefault `NO_BACKSLASH_ESCAPES` or `ANSI_QUOTES` SQL modes.

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

### Optional scalar filtering

Admitted ordinary and `feature_sql` profiles expose only published direct scalar
columns with catalog-proven semantics: integer widths (including full unsigned
64-bit), exact DECIMAL precision/scale, BIT(1) booleans, and utf8mb4 VARCHAR/TEXT
families. FLOAT/DOUBLE, CHAR, other character sets and date/time declarations
are not advertised initially. Public aliases resolve back to their frozen
physical columns; identity, geometry and private source metadata are excluded.
Optional catalog failure preserves the admitted Core and legacy tile source.

All six comparisons and NULL tests use SQL three-valued logic. Numeric literals
remain exact; directed lattice rounding avoids floating conversion and DECIMAL
rounding. UTF-8 string comparisons use binary source bytes and a bound ASCII hex
parameter through UNHEX, preserving NUL, case, supplementary characters and
trailing spaces independently of connection collation. Raw feature BIT(1)
properties are strict boolean values; tile decoding is unchanged.

The filter is applied inside the protected read snapshot before geometry/time
matching, counts and pagination, AND-combined with a configured `feature_sql`
selection. These capabilities do not themselves declare an OGC conformance
class.

## Troubleshooting MOS custom SQL

### Registration times out despite a larger connection timeout

`timeout = "90s"` limits connection establishment. It does not extend the
30-second registration probe budget. For explicit MOS, Tegola checks result
columns without decoding sample geometries. It then samples geometry classes
unless `geometry_type` is configured or inspection is deferred.

Older MySQL versions can materialize a derived table even when its outer query
has `WHERE 1=0` or `LIMIT 0`. Metadata inspection first checks the full result
projection with zero rows; supported SELECT forms use a direct `LIMIT 0`.
Format/class sampling then transfers only the configured geometry column.
A direct sample rewrite requires simple identifier projections with optional
aliases (with or without `AS`) and a unique geometry output. Expressions, stars,
DISTINCT/ALL,
grouping, ordering, unions, windows, locking, executable comments and ambiguous
projections use a conservative geometry-only derived query. That fallback may
still materialize. Direct sampling retains smaller existing limits and offsets
and caps the row window at 16; no rewrite guarantees the probe deadline.

Registration geometry samples are checked against a 64 MiB cap before decoding.
Custom-SQL format/class queries retrieve at most the cap plus one byte to detect
oversize values; ordinary table sample cells are checked after retrieval. Streamed inspection
retains its first usable geometry rather than buffering all sample blobs.
This fixed registration-only limit is not a runtime feature/tile size limit.
See [startup metadata queries](../../docs/geometry-formats.md#startup-metadata-queries).

### Bounds column is ambiguous in a JOIN

If both joined tables expose `MINX`, `MAXX`, `MINY` or `MAXY`, MySQL can return
`Error 1052 (23000): Column 'MAXX' in where clause is ambiguous` when `!BBOX!`
is expanded. Set `bbox_table` on that custom-SQL layer to its source alias or
schema-qualified table. Keep `bbox_*_fieldname` as simple result-column names.
The full [joined MOS example](../../docs/geometry-formats.md#joined-mos-layer-example)
shows this separation. Merely qualifying the columns in the SELECT list does
not qualify the generated WHERE predicate.

These JOIN and token options apply to tile SQL. They do not broaden the
single-table `feature_sql` grammar or bypass raw feature identity, snapshot,
temporal, dimensional or CRS validation. Successful MOS registration or tile
delivery is not evidence of Feature API publication or OGC conformance.

### Verify actual tile queries after startup

A successful registration or `/capabilities` response does not execute the
per-tile spatial predicate. Start a separate test instance with `--no-cache`
and request both an affected layer tile and the corresponding full-map tile.
Use `:8083` or `127.0.0.1:18083` for the bind address; see
[HTTP bind address](../../docs/configuration.md#http-bind-address).
