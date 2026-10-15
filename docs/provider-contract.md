[← Development and Builds](development.md) · [Back to README](../README.md) · [CRS Contract →](crs.md)

# Provider contract

This document describes the configuration keys and runtime semantics that are
common to all standard (raw-feature) data providers: `mysql`, `gpkg`,
`postgis` and `hana`. Provider-specific READMEs only describe backend
differences; when a provider deviates from this contract it is called out
explicitly in the matrix below.

MVT providers (`mvt_*`, e.g. `mvt_postgis`) encode the tile inside the
database (e.g. `ST_AsMVT`) and intentionally do **not** participate in the
raw geometry format / MOS parts of this contract.

## Common provider-level keys

| Key | Type | Meaning |
|---|---|---|
| `name` | string | Provider name referenced from map layers. Required. |
| `srid` | int | Default SRID for all layers of the provider. |
| `crs_defn` | string | Full PROJ.4 definition used instead of a numeric `srid`. Wins over `srid` at the same level. See [crs.md](crs.md). |
| `geometry_format` | string | Default geometry format for layers that do not override it: `wkb`, `wkt` or `mos`. Empty/unset means the provider's native geometry handling. Provider-specific values exist: `mysql` accepts `auto`, `mysql` and `mariadb` in addition to the shared formats; `gpkg` accepts `gpkg` (its native GeoPackage binary). |
| `mos_precision` | int | Decimal digits carried by MOS coordinates (see below). Only applies when `geometry_format = "mos"`. |
| `mos_units` | string | Packed linear unit of MOS coordinates: `mm`, `cm`, `dm`, `m` or `km`. Only applies when `geometry_format = "mos"`. |
| `bbox_minx_fieldname` | string | Bounds column holding the feature minimum X, used by bounds-backed MOS SQL. Default `MINX`. |
| `bbox_maxx_fieldname` | string | Bounds column holding the feature maximum X. Default `MAXX`. |
| `bbox_miny_fieldname` | string | Bounds column holding the feature minimum Y. Default `MINY`. |
| `bbox_maxy_fieldname` | string | Bounds column holding the feature maximum Y. Default `MAXY`. |

## Common layer-level keys

| Key | Type | Meaning |
|---|---|---|
| `name` | string | Layer name used by map layers. Required. |
| `tablename` | string | Table to query. Mutually exclusive with `sql`. |
| `sql` | string | Custom SQL. Mutually exclusive with `tablename`. Supports `!BBOX!`, `!ZOOM!`, `!X!`, `!Y!`, `!Z!`, `!SCALE_DENOMINATOR!`, `!PIXEL_WIDTH!`, `!PIXEL_HEIGHT!`, `!ID_FIELD!`, `!GEOM_FIELD!`, `!GEOM_TYPE!` (token support varies slightly per provider; unknown tokens are rejected). |
| `geometry_fieldname` | string | Geometry column. Defaults to `geom` for generated table SQL. For custom SQL the column must be present in the result set; an empty `geometry_fieldname` means the geometry column is the last column of the result set. |
| `id_fieldname` | string | Feature id column. Defaults: `fid` for `mysql`/`gpkg`, empty for `postgis`/`hana`. |
| `geometry_type` | string | Explicit layer geometry type, valid for every standard provider: `Point`, `LineString`, `Polygon`, `MultiPoint`, `MultiLineString`, `MultiPolygon`, `GeometryCollection`. An explicit value fixes the layer geometry type and skips geometry-class inference (empty data is allowed); automatic format detection needs three valid MOS samples, or one in a completed short result; structural validation of custom SQL still runs. It is orthogonal to MapplGIS table detection: a `tablename` layer is still checked for the MapplGIS signature and still receives system-info configuration when detected. Mixed content is permitted: features whose decoded type differs from the declared value are rendered, and the mismatch is logged once per layer. |
| `srid` / `crs_defn` | int / string | Layer CRS override; see [crs.md](crs.md). |
| `geometry_format` | string | Layer-level geometry format override. |
| `mos_precision` / `mos_units` | int / string | Layer-level MOS overrides (only with `mos`). |
| `bbox_*_fieldname` | string | Layer-level bounds column overrides (`bbox_minx_fieldname`, `bbox_maxx_fieldname`, `bbox_miny_fieldname`, `bbox_maxy_fieldname`). Values must be simple identifiers: trimmed, qualified names like `t.MINX` rejected, duplicates rejected case-insensitively. Resolution is per field: layer > provider > defaults (`MINX`/`MAXX`/`MINY`/`MAXY`); for joins set `bbox_table` or use a CTE/derived table with unambiguous bounds column names. Resolved columns are excluded from feature tags. Only used by bounds-backed MOS SQL (see [geometry-formats.md](geometry-formats.md)). |
| `bbox_table` | string | Optional MOS custom-SQL layer qualifier for `!BBOX!` bounds fields, e.g. `roads` (table alias) or `gis.roads_axis`. One or two unquoted simple identifiers; components are quoted separately by the provider. Result column names and tag exclusion remain unchanged. |
| `fields` | []string | Additional fields to include for generated table SQL. Semantics of an absent or empty `fields` differ per provider and cannot be mixed freely (see matrix below). |

## CRS

The CRS resolution order is identical for every standard provider:

1. Layer `crs_defn`
2. Layer `srid`
3. Provider `crs_defn`
4. Provider `srid` (if explicitly configured)
5. Source-derived SRID (database metadata, geometry header or — for MOS
   layers of any standard provider — the `MapplGIS LayerInfo projection`
   blob)
6. Provider default (usually 3857)

Provider-native metadata is consulted only in the provider's native geometry
paths; raw-format layers always fall back to the system-info projection (see
[crs.md](crs.md) for the per-provider auto-detect details).

Built-in projected SRIDs and custom PROJ.4 definitions are documented in
[crs.md](crs.md).

## `tablename` vs `sql`

Both keys are mutually exclusive in every standard provider: specifying both
is a startup error, as is specifying neither. This is enforced by key
presence, so an explicit `tablename` that happens to equal the layer name is
still detected.

## Startup inspection and deferred layers

At registration every layer is structurally validated and inspected to
resolve its geometry type and, where possible, its SRID. Structural
validation for custom SQL always runs at registration: a missing
`!BBOX!`/`!BOX!` token or a missing configured geometry column is a startup
error; `mos` requires the geometry column and the `!BBOX!` token, custom
`geometry_format = "gpkg"` carries the same contract with the
bounds-source-CRS predicate, and `wkb` / `wkt` forbid `!BBOX!` and
bounds columns. The four bounds columns are validated only when they
appear in the SQL result: their actual result-column names are persisted
for the bounds predicate, and when the SELECT list omits them the layer is
registered with a warning under the resolved names (layer > provider >
`MINX`/`MAXX`/`MINY`/`MAXY`) — the `!BBOX!` predicate resolves the bounds
columns in the query's own scope (e.g. the source table's columns), so they
do not have to be selected. This rule is identical in every SQL provider
(`mysql`, `postgis`, `hana`, `gpkg`). An explicit `geometry_type` never
skips structural validation — it skips geometry-class inference (explicitly typed layers may have empty data).
Automatic MOS format detection requires three valid samples, or one in a completed short result.

- **Table layers** are inspected via database metadata / a sample query.
- **Custom SQL** in automatic format mode is sampled within a window of 16 rows,
  stopping after three successfully decoded MOS geometries.
  Bounds-predicate `!BBOX!` tokens are neutralized to `1=1`; native PostGIS
  geometry operands instead receive the zoom-zero envelope transformed into
  the layer CRS, including tokens used as spatial-function arguments.
  Automatic PostGIS MOS detection can retry the predicate form if the native
  query fails, but still requires positive decoded MOS evidence. Position /
  zoom tokens are permissive. All token handling follows the backend's SQL
  dialect and preserves literals and comments. If the query currently returns no rows, the layer is still
  registered (without an inferred geometry type) so the server can start;
  it begins serving once the query returns data.

## `!BBOX!` semantics

`!BBOX!` is always expanded into the layer SRID: tile bounds are converted
from Web Mercator into the layer CRS before the query is executed, so the
filter, the data and the MVT encoding agree on one CRS (see
[crs.md](crs.md) for the conversion details and synthetic-SRID rules).

| Format | SQL-side filter |
|---|---|
| native geometry | Provider spatial predicate (`&&`, `ST_Intersects`, MBR, …) |
| `wkb` / `wkt` | No native spatial predicate (raw values are not database geometries); the exact in-memory bounding-box check on the decoded geometry is applied in all cases |
| `mos` | Bounds-backed SQL: the `!BBOX!` token expands into the indexed bounds-columns predicate (`bbox_*_fieldname`, defaults `MINX/MAXX/MINY/MAXY`) scaled by the layer's MOS quantization; the exact in-memory bounding-box check remains authoritative. Custom MOS SQL must carry `!BBOX!`/`!BOX!` |

MapplGIS detection distinguishes how a layer was identified: `table-canonical`
(structural DDL + PK + indexes + probe row, guarantees system info) vs
`sql-sample` (custom SQL whose sample carries the four bounds columns and
decodable MOS rows — no system info guarantee). MOS custom SQL requires an
explicit `srid` or `crs_defn` (a missing CRS is a startup error);
`mos_precision` and `mos_units` are optional with the normative paired
defaults (see [geometry-formats.md](geometry-formats.md#mos-quantization)).

## SQL-context-aware token substitution

Token substitution and detection use the provider's SQL dialect through
`internal/sqltoken`. Only executable SQL contains substitution sites; literals,
quoted identifiers and comments recognized by that dialect remain unchanged.
A protected `!BBOX!` mention neither satisfies a custom-SQL token requirement
nor triggers raw-format (`wkb`/`wkt`) token rejection.

PostgreSQL preserves dollar-quoted strings and distinguishes ordinary strings
(`standard_conforming_strings=on`) from backslash-escaped `E'...'` strings.
Its `#>`, `#>>`, `#-` operators and array brackets remain executable SQL.
MySQL recognizes `#` comments and its whitespace-sensitive `--` comments,
using the default backslash-escape mode. SQLite and HANA use their own quoting
rules; SQLite backslashes do not escape string quotes. Nondefault PostgreSQL
and MySQL string modes are outside the documented scanner contract.
The legacy package-level scanner remains for compatibility; providers use
explicit dialects for both runtime substitution and registration probes.

## System info auto-configuration (MapplGIS tables only)

MOS tables written by MapplGIS carry a `MapplGIS LayerInfo` metadata blob.
Every standard provider identifies such tables with the same one-time,
registration-time detector — the detection is **structural**, based on the
table's DDL, its indexes and a single probe row, never on scanning result
rows or sample windows:

1. The registered `tablename` is inspected (case-insensitive) for all nine
   required columns: `OKEY`, `MUID`, `MINX`, `MAXX`, `MINY`, `MAXY`,
   `ObjectStyle`, `ObjectType`, `LINE`.
2. `OKEY` must be the table's primary key.
3. All six required indexes must exist over `MUID`, `MINX`, `MAXX`, `MINY`,
   `MAXY`, `ObjectType`.
4. A single point probe (`OKEY = 1 AND LINE IS NOT NULL`) must return a
   decodable `LayerSystemInfo` blob.

Only when all four conditions hold does the layer get `IsMapplGIS=true`,
applied exactly once during registration. From the decoded system info the
provider reads:

- `Precision` → `mos_precision` (when not explicitly configured),
- `Projection` → layer PROJ.4 definition registered as a synthetic SRID (same
  mechanism as `crs_defn`, only used when no `srid`/`crs_defn` is configured),
- `MapUnits` / `flMapUnitsDefined` → `mos_units` metres factor.

Explicit configuration always wins over system info.

**Table-canonical MapplGIS detection never applies to custom `sql` layers,
and `LayerSystemInfo` is never applied from result rows.** Custom `sql`
layers are still subject to `sql-sample` storage detection (above), which
tags the layer `MapplGIS` without system info. A SQL layer that uses
`geometry_format = "mos"` must configure `srid` or `crs_defn` explicitly in
its config (a missing CRS is a startup error); `mos_precision` and
`mos_units` are optional with the normative paired defaults. A decodable
blob that happens to appear in the result set is ignored (the row is still
rendered as a feature if its geometry decodes, otherwise it is skipped).

Because detection happens once at registration, tile requests do not repeat
it: after `NewTileProvider` returns, the layer's MapplGIS identity and MOS
settings are immutable and the tile path only reads them. `geometry_type` is
orthogonal to detection: an explicit `geometry_type` does not skip MapplGIS
table detection (it only fixes the layer's geometry type), and a detected
MapplGIS table still applies its system info even with `geometry_type` set.

The detector is implemented per provider against the backend's own catalog:
MySQL/MariaDB via `SHOW COLUMNS` / `SHOW INDEX` (version-safe parsing of the
5.7/8.0/MariaDB row shapes), GPKG via `PRAGMA table_info` /
`sqlite_master`, PostGIS via `information_schema.columns` /
`pg_index` / `pg_attribute`, HANA via `SYS.TABLE_COLUMNS` /
`SYS.INDEXES` / `SYS.INDEX_COLUMNS`. Detection runs for every `tablename`
layer of every standard provider; custom `sql` layers are subject only to
`sql-sample` storage detection (above), which never applies system info.

When a table is detected as MapplGIS, the effective geometry format is
authoritative `mos`: an unset or `auto` format resolves to `mos`, and an
explicit non-MOS `geometry_format` on the same layer is a startup conflict
error. A detected table also replaces an unset (default) `id_fieldname` with
the contract primary key `OKEY`; an explicitly configured id field is
honored.

Startup type inspection (as opposed to MapplGIS detection) still samples the
same uniform window of up to `codec.InspectionSampleLimit` (16) rows to
infer the layer geometry type from the first decodable geometry — but this
sampling is type inference only and never applies system info.

## Tile provider support matrix

| Capability | mysql | gpkg | postgis | hana |
|---|---|---|---|---|
| `srid` / `crs_defn` | yes | yes | yes | yes (synthetic CRS requires a raw format, see below) |
| `geometry_format` (`wkb`/`wkt`/`mos`) | yes | yes (`mos` skips the GeoPackage binary header) | yes | yes |
| `mos_precision` / `mos_units` | yes | yes | yes | yes |
| system info auto-config (MapplGIS tables) | yes | yes | yes | yes |
| native spatial filter | yes (MBR/indexed bounds) | yes (RTree index; skipped for all raw formats `wkb`/`wkt`/`mos`, and for raw tables without GeoPackage metadata) | yes (`&&`) | yes (`ST_IntersectsRect*`; skipped for `mos` and synthetic CRS) |
| `id_fieldname` default | `fid` | `fid` | empty | empty |
| absent/empty `fields` | id + geometry only | id + geometry only | all columns | all columns |
| MVT variant | — | — | `mvt_postgis` | `mvt_hana` |

HANA notes:

- HANA distinguishes round-earth and planar SRS; a round-earth SRS is queried
  through its planar equivalent internally (`PLANAR_SRID_OFFSET = 1000000000`).
- A synthetic CRS (from `crs_defn` or system-info `Projection`) cannot be used
  with HANA's native `ST_Geometry` handling: the provider requires
  `geometry_format` = `wkb`, `wkt` or `mos` for such layers. MVT providers do
  not support synthetic CRS at all.
- Planar-equivalent SRIDs (`1000000000 + n`) are internal and must not be
  confused with synthetic Tegola SRIDs (`>= 340000001`).

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

## Raw feature query contract

Raw feature publication uses a separate immutable source profile and does not
reuse tile SQL or tile field defaults. Ordinary table `fields` selects public
properties; absent or empty configuration selects all eligible properties.
Request field selection only narrows that frozen public set. A constrained
`feature_sql` projection defines public aliases independently of tile `fields`;
custom tile SQL alone does not establish a raw feature source.

Direct startup metadata probes and `bbox_table` qualification improve legacy
tile SQL registration and predicates. They do not admit JOINs into `feature_sql`
or weaken raw source identity, protected-snapshot, temporal, dimensional or CRS
proofs. Automatically detected MOS tile rows can use the documented tolerant
skip policy; published raw features retain strict source-data error handling.

Identity, geometry and configured private bounds/metadata are suppressed by
physical column lineage, including renamed projections. Physical `min_zoom` and
`max_zoom` are always private under ASCII case folding. Required temporal columns
are read privately when omitted from the public subset; explicitly selected
temporal properties remain public.

Selected SQL NULL properties remain present with nil values and serialize as JSON
null. Private and unselected properties are absent. Zero, false and empty strings
retain their values. This rule does not change tile property decoding.

Exact spatial and temporal matching precedes logical paging. Source corruption
fails the query; unsupported requested capabilities remain distinct. Actual
MySQL/MariaDB, PostGIS and HANA admitted profile checks have passed independent
source and runtime review. HANA uses a transaction-owned source lock that blocks
writers and DDL for the query duration. Native storage rejection or normalization
is distinct from successful feature-query evidence; unsupported storage and CRS
profiles remain explicit. None of these results declares OGC conformance.

Consult [GeoPackage](../provider/gpkg/README.md),
[MySQL/MariaDB](../provider/mysql/README.md),
[PostGIS](../provider/postgis/features.md), and
[HANA](../provider/hana/FEATURE-QUERY.md) for exact storage, CRS, snapshot and
resource limits. [ADR-0007](architecture/decisions/ADR-0007-safe-provider-feature-sql.md)
defines constrained selection;
[ADR-0008](architecture/decisions/ADR-0008-hana-feature-source-lock.md)
defines HANA's read-write repeatable-read transaction and source lock. Existing
tile behavior in the sections above retains its separate contract.

### Optional referenced CRS contract

`FeatureCRSLayerInfo` supplies a copied effective source definition and spatial
profile. Providers prove identity at registration; a numeric SRID alone cannot
authorize a public EPSG identifier. Optional unsupported metadata preserves Core
and tile queries. Admitted Part 2 query targets carry a bounded
`FeatureQuery.BoundsCRSDefinition`, independently validated before I/O and used
to construct an owned converter. Its definition is authoritative over the numeric
mapping: same-SRID shortcuts and indexed pruning require proven mathematical
identity of the complete frozen source and target definitions. Exact intersection
remains in the original query frame
before paging and counts. See [CRS identifiers](crs.md#feature-api-crs-identifiers)
and [ADR-0010](architecture/decisions/ADR-0010-public-crs.md).

### Optional scalar filtering contract

Optional `FeatureQueryableLayerInfo` metadata supplies a detached, bounded
catalog of proven public scalar fields. `FeatureQuery.Filter` carries a
validated neutral expression, never SQL. Providers resolve every expression
against their own catalog and private physical mappings before compiling bound
predicates. Invalid properties or literal types are typed client errors;
missing optional metadata is an unsupported capability and must preserve
ordinary Core and tile queries. Filtering combines with trusted source selection
and existing query constraints before counts and paging. See
[ADR-0009](architecture/decisions/ADR-0009-typed-feature-filtering.md) for the
exact scalar, integrity and resource boundaries.

## See Also

- [Queryables and filtering](filtering.md) — typed property catalogs, comparison semantics and limits
- [Configuration](configuration.md) — cache, provider, and TOML examples
- [CRS contract](crs.md) — coordinate reference system behavior
- [Geometry formats](geometry-formats.md) — raw feature geometry handling
