[← CRS Contract](crs.md) · [Back to README](../README.md) · [Maintenance Status →](maintenance-status.md)

# Geometry formats

All standard providers (`mysql`, `gpkg`, `postgis`, `hana`) decode geometries
into an internal in-memory representation before projecting them to Web
Mercator and encoding them as MVT features. This document describes the
supported input formats, their bounding-box behaviour, and how
`GeometryCollection` values are processed.

Read/tile contracts and the MOS write profile below have separate admission: a format that can
be decoded for a tile is not automatically safe to modify. The
[write evidence matrix](provider-matrix.md) identifies tested provider/format
combinations and the [write scope](wfs-scope-limitations.md) lists admission limits.

## Supported formats

`geometry_format` is a provider-level default and can be overridden per
layer.

| Format | Value | Description |
|---|---|---|
| Native | *(unset / empty)* | The provider's own binary geometry handling: WKB-backed blobs for `postgis`/`hana`/`mysql` (`ST_AsBinary`), GeoPackage binary blobs for `gpkg`. |
| WKB | `wkb` | Plain OGC WKB, e.g. selected via `ST_AsBinary(...)` in custom SQL. |
| WKT | `wkt` | OGC WKT text, e.g. selected via `ST_AsText(...)`. |
| MOS | `mos` | Opaque MapplGIS MOS binary blob with integer-quantized coordinates. |

`geometry_fieldname` selects the column that carries the geometry (default
`geom`); `id_fieldname` selects the feature id column (default `fid` except
`postgis`/`hana`, see [provider-contract.md](provider-contract.md)).

## MOS quantization

MOS coordinates are integers in a packed linear unit. `mos_precision` (decimal
digits) and `mos_units` (`mm`, `cm`, `dm`, `m`, `km`; default `m`) define the
scale factor applied when converting to metres. The default `mos_precision` is
paired with the effective units: `mm`→`0`, `cm`→`1`, `dm`→`1`, `m`→`2`,
`km`→`5`. An explicitly set `mos_precision` overrides the units-paired default.
`mos_precision` and `mos_units` are optional with these normative paired
defaults (`DefaultMOSPrecisionForUnits`); the mandatory part of a MOS
configuration is the CRS: MOS custom SQL requires an explicit `srid` or
`crs_defn` and fails at startup without one. Explicitly set
`mos_precision`/`mos_units` always win over values detected from a
`MapplGIS LayerInfo` metadata blob during registration-time MapplGIS table
detection (see
[provider-contract.md](provider-contract.md#system-info-auto-configuration-mapplgis-tables-only)).

When writing an admitted MOS layer, the writer must use the same resolved
precision and units as its reader. Integer quantization can change coordinates;
verify decoded coordinates against the declared precision, not byte equality
with an unrelated native geometry encoding. A raw MOS fixture without derived
bounds does not establish write evidence for MapplGIS tables whose separate
MINX/MAXX/MINY/MAXY columns must remain consistent with their geometry.

### MOS writes with separate bounds columns

MySQL and the GeoPackage provider's raw SQLite MOS profile maintain an admitted
four-column bounds mapping in the same transaction as the geometry. Insert and
geometry replacement write bounds derived from the encoded, quantized geometry,
in packed integer MOS units. Attribute-only updates preserve geometry bytes,
opaque trailing annotations and existing bounds. Deletion removes the row;
setting nullable geometry to NULL requires nullable bounds columns as well.

REST PUT preserves the native geometry and bounds when its supplied CRS84
geometry exactly matches the current canonical read representation. The read
revision is checked again inside the write transaction, including when the
request uses `If-Match: *`; concurrent changes fail the precondition. Properties
still follow complete replacement semantics. Changed geometry is encoded normally;
no coordinate tolerance or approximate equality suppresses an intended edit.

The mapping uses the resolved `bbox_*_fieldname` keys described below. All four
columns must exist, be distinct writable numeric columns, and be separate from
the identity and geometry columns. They are maintained by the provider and are
not client-editable attributes. MySQL additionally checks numeric storage range;
incomplete, generated or incompatible mappings fail write admission. Geometry
that overflows the MOS integer grid or collapses during quantization is rejected.

Use a direct table-backed layer for mutation. Custom SQL, joins, views and
`bbox_table` query aliases do not establish a writable table mapping. MySQL
requires InnoDB and the ordinary generated-key/transaction/revision safeguards.
Raw MOS in SQLite remains distinct from native GeoPackage binary geometry and
its RTree metadata. This extension does not add PostGIS bounds-column writes.
For detected canonical MapplGIS tables, existing feature update/delete is bounded
by additional metadata safeguards: SystemInfo rows are immutable, MUID,
ObjectStyle and ObjectType remain read-only, and replacement preserves the
stored geometry family. Canonical creation remains unavailable until the
MUID/ObjectType/style initialization contract is defined. Ordinary raw MOS
tables retain their separate create admission.
Canonical geometry replacement is further limited to ordinary Point, Polyline
and Polygon records whose existing bytes can be reproduced by the encoder.
Opaque tails, nondefault header flags/modifications, legacy header layouts and
special Text/Image records cannot be rewritten safely and are rejected for
geometry edits. Their admitted attribute-only updates retain the original bytes;
delete remains a separate operation. These canonical metadata protections are
stricter than geometry replacement on an ordinary raw MOS table.
For source projection support see [custom feature CRS](crs.md#custom-feature-crs).
Native runtime results remain separately recorded in the
[provider evidence matrix](provider-matrix.md).

## CRS handling

The layer SRID resolution order and the Web Mercator (EPSG:3857) conversion
are documented in [crs.md](crs.md). In short: tile bounds (`!BBOX!`) are
always expressed in the layer's resolved CRS, decoded geometries are
reprojected from that CRS to Web Mercator, and MVT encoding consumes the
reprojected geometry — one consistent CRS contract end to end.

## BBOX filtering per format

| Format | SQL filter | In-memory filter |
|---|---|---|
| Native | Provider spatial predicate against the indexed geometry (e.g. `&&`, MBR, RTree) | Exact bounding-box check |
| `wkb` | None (raw bytes are not database geometries); `mysql` wraps the WKB column in `ST_GeomFromWKB` as a provider optimization | Exact bounding-box check |
| `wkt` | None; `mysql` wraps the WKT column in `ST_GeomFromText` as a provider optimization | Exact bounding-box check |
| `mos` | Bounds-backed SQL: indexed bounds columns (`MINX/MAXX/MINY/MAXY` by default) filtered with `!BBOX!`; mandatory for custom SQL | Exact bounding-box check |

Because raw formats cannot use the database spatial index (except the
bounds-backed MOS case), spatial selectivity comes from the in-memory
bounding-box check applied to every decoded feature. The MySQL
`ST_GeomFromWKB`/`ST_GeomFromText` wrapping is only a server-side coarse
filter (a provider optimization, not an index use); the exact in-memory
bounding-box check remains mandatory for every raw format.

### Bounds-backed MOS SQL (`!BBOX!` over bounds columns)

The `mos` format stores each feature's raw bounds in four numeric columns
(EGKO MapplGIS convention: `MINX`, `MAXX`, `MINY`, `MAXY` by default). These
are the only server-side filter a raw MOS column can support: for `mos`
layers the `!BBOX!` token (and `!BOX!`) expands into the bounds predicate

```
<maxx> >= tile.minx AND <minx> <= tile.maxx AND <maxy> >= tile.miny AND <miny> <= tile.maxy
```

scaled from the tile metres by the layer's MOS quantization
(floor/ceil x 10^precision / unit-factor), so the comparison runs in the
same integer units the bounds columns store.

For MySQL feature queries, an explicit MOS bounds mapping also supplies a
conservative SQL candidate filter when the request bbox uses the same native
CRS as the layer. The bounds are scaled to the stored integer grid; decoded
geometry still undergoes the normal spatial check. OGC Features clients can
select this path with `bbox-crs` set to the collection's advertised
`storageCrs`, independently of the response CRS. This optimization does not
change the fallback for bboxes in a different CRS, and does not guarantee
index use or identical performance for every schema.

The four columns are configurable with the common config keys (layer level
overrides provider level, per field):

- `bbox_minx_fieldname` (default `MINX`)
- `bbox_maxx_fieldname` (default `MAXX`)
- `bbox_miny_fieldname` (default `MINY`)
- `bbox_maxy_fieldname` (default `MAXY`)

Each value must be a simple identifier: values are trimmed, qualified names
such as `t.MINX` are rejected, and two keys may not name the same column
(duplicates are rejected case-insensitively). For MOS custom SQL with joins,
set the optional layer `bbox_table` to the source table or its SQL alias:

```toml
bbox_table = "gis.roads_axis"
```

This makes `!BBOX!` reference, for example,
`` `gis`.`roads_axis`.`MAXX` `` on MySQL, avoiding ambiguous bounds
columns in joined tables. Use the alias if the source table has one. The value
accepts one or two unquoted simple identifiers (alias/table or schema.table),
quoted separately for MySQL, PostgreSQL, HANA and GeoPackage. Keep
`bbox_*_fieldname` and SELECT result names unqualified. Alternatively, use a
CTE or derived table exposing unambiguous bounds column names; older MySQL
versions may materialize those derived tables.

Columns resolved this way are excluded from the feature tags: they are
implementation details of the bounds contract, not feature attributes.

### Raw formats and custom SQL

Custom SQL (`sql` key) with a raw `geometry_format`. In custom SQL an empty
`geometry_fieldname` means the geometry column is the last column of the
result set. Structural validation always runs at registration: a missing
`!BBOX!`/`!BOX!` token or a missing configured geometry column is a startup
error naming the layer. The bounds columns are validated only when they
appear in the SQL result: their actual result-column names are persisted for
the bounds predicate, and when they are absent from the SELECT list the
layer is registered with a warning under the resolved names (layer >
provider > `MINX`/`MAXX`/`MINY`/`MAXY`), because the `!BBOX!` predicate
resolves the bounds columns in the query's own scope (e.g. the source
table's columns) — they do not have to be selected. This contract is
identical in every SQL provider (`mysql`, `postgis`, `hana`, `gpkg`).

- `mos` **must** use `!BBOX!` (or `!BOX!`): the bounds predicate over the
  configured bounds fields is the only server-side selectivity a raw MOS
  column supports. The `mos` structural contract requires the geometry
  column and the `!BBOX!` token as startup errors; the four bounds columns
  follow the result-column rule above. An
  explicit `geometry_type` never skips this structural validation — it
  skips only geometry-class inference and the >=3-sample-row requirement
  (explicitly typed layers may have empty data). With
  `geometry_type = "auto"`, the effective format switches to `mos` only on
  a positive MOS signature, at least three decodable MOS rows with
  coordinates and a structurally valid `mos` contract (the `MapplGISSource` =
  `sql-sample` probe); native / WKB / WKT rows never count as MOS. SQL-sample
  storage detection tags the layer `MapplGIS` without ever applying
  `SystemInfo` or a projection from sample rows.
- `wkb`/`wkt` must **not** use `!BBOX!` (nor its `!BOX!` alias): its
  expansion assumes a native spatial column (e.g. `geom && ST_MakeEnvelope(...)`
  or a `minx`/`maxx` column predicate), which a raw BLOB/TEXT column does not
  provide. Providers reject such custom SQL at startup with an error naming the
  layer and the offending token — remove `!BBOX!` from the custom SQL; the exact
  in-memory bounding-box filter is always applied for raw formats. Custom SQL
  without `!BBOX!` works for raw formats in every provider, so the same
  configuration behaves identically across `postgis`, `hana`, `gpkg`, and
  `mysql`. No bounds columns apply to `wkb`/`wkt`. With
  `geometry_format = "auto"` (`mysql`), structural validation runs against
  the effective format resolved at registration: a layer that resolves to
  `mos` must satisfy the `mos` contract above, `wkb`/`wkt` resolutions must
  satisfy the raw contract, and a resolved native geometry column may use
  `!BBOX!`.

### Joined MOS layer example

The following layer belongs inside an existing standard provider configured
with the correct source CRS and MOS quantization settings:

```toml
[[providers.layers]]
name = "roads_axis"
geometry_format = "mos"
geometry_fieldname = "LINE"
id_fieldname = "MUID"
bbox_table = "axis"
sql = """
SELECT axis.MUID, axis.LINE,
       axis.MINX, axis.MAXX, axis.MINY, axis.MAXY,
       road.name AS road_name
FROM roads_axis AS axis
LEFT JOIN roads AS road ON road.axis_muid = axis.MUID
WHERE !BBOX!
"""
```

Even if `roads` also has bounds columns, the filter references only
`axis.MAXX`, `axis.MINX`, `axis.MAXY` and `axis.MINY` (with provider-specific
quoting). The result names remain `MINX`/`MAXX`/`MINY`/`MAXY` and are excluded
from feature attributes. Without a source alias, `bbox_table = "gis.roads_axis"`
is also valid. This is a layer-only option for custom MOS SQL; it does not
change native spatial filters or generated table/RTree queries. It qualifies
every bounds token in the query, so the chosen alias must be visible at each
`!BBOX!` or `!BOX!` location.

This example is tile SQL. Its JOIN is not an admitted `feature_sql` selection.
`bbox_table` resolves tile-predicate ambiguity; it does not prove a raw feature
source's unique identity, immutable catalog or CRS eligibility. Opt-in Feature
API publication keeps its separate strict source contract.

### Startup metadata queries

Explicit `geometry_format = "mos"` checks result columns without wrapping a
16-row sample. MySQL uses a full-projection, zero-row metadata query first.
Supported ordinary SELECT statements receive a direct `LIMIT 0`, including
supported numeric LIMIT clauses; complex forms retain a metadata wrapper.
PostgreSQL, GeoPackage and HANA use a metadata wrapper over the prepared SQL.

MySQL format/class inference fetches only the configured geometry column.
A direct rewrite is admitted only when every SELECT expression is a simple
identifier path (with an optional alias, with or without `AS`), with one
unambiguous geometry output.
Expressions, `*`, DISTINCT/ALL, grouping, ordering, unions, windows, locking and
executable comments use a conservative geometry-only derived projection.
The direct path preserves FROM/JOIN/WHERE, smaller numeric limits and offsets,
and caps its search window at 16 rows. The fallback can still materialize on
older MySQL servers; neither path guarantees completion within the deadline.

Registration samples have a fixed 64 MiB geometry-cell cap, independent of the
row window. Custom-SQL format/class queries fetch at most the cap plus one byte to detect
oversize values; ordinary table sample cells are checked after retrieval. An
oversized cell fails inspection before decoding, rather than publishing a
truncated geometry. Samples are decoded as rows arrive, retaining the first
usable geometry instead of buffering sixteen geometry blobs. This limit has no
configuration knob and does not change runtime tile or Feature API limits.
Automatic MOS format detection stops after three successfully decoded, nonempty
MOS geometries; the 16-row window allows NULL, malformed, empty and SystemInfo
values to be skipped. Explicit MOS format skips format sampling. Geometry-class
inference is a separate probe and retains only its first usable geometry.

The MySQL provider's `timeout` controls connection establishment, not the
30-second registration probe deadline. An explicit `geometry_type` skips
geometry-class sampling but does not skip this result-column validation.

## GeometryCollection behaviour

`GeometryCollection` values (including nested collections) are handled in
three cooperating stages:

1. **CRS conversion.** The reprojection helper applies the CRS transform to
   every point of every member geometry, recursively. Collections are not
   flattened at this stage.
2. **MVT encoding.** `GeometryCollection` features are recursively flattened
   into separate MVT features, one per leaf geometry, so MVT never emits
   nested collections. Leaf multiparts are kept whole.
3. **Auto-styling** (`?style` generated styles). A layer whose collection
   mixes polygon and line leaf geometries gets two sibling style layers:
   a line layer rendered before the fill layer, with `"filter": {"$type":
   "Polygon"/"LineString"}` so leaf geometry classes are styled correctly.

## See Also
- [provider-contract.md](provider-contract.md) — configuration keys shared by all providers
- [crs.md](crs.md) — SRID resolution, built-in and synthetic CRS definitions
- [Configuration](configuration.md) — provider and environment settings
