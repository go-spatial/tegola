[← Provider Contract](provider-contract.md) · [Back to README](../README.md) · [Geometry Formats →](geometry-formats.md)

# CRS Contract

This document describes the unified coordinate reference system (CRS) contract
shared by the standard storage providers: `postgis`, `gpkg`, `mysql` and
`hana`.

The implementation lives in [`provider/crsconfig`](../provider/crsconfig/) and
uses the registration helpers from [`basic/epsg.go`](../basic/epsg.go).

## Config keys

Both keys are accepted at the **provider level** (default for all layers) and
at the **layer level** (override for a single layer):

| Key | Type | Description |
|---|---|---|
| `srid` | int | Numeric spatial reference id of the layer's source CRS (e.g. `3857`, `4326`, `32637`). |
| `crs_defn` | string | A full PROJ.4 definition of the source CRS, e.g. `+proj=longlat +datum=WGS84 +no_defs`. |

When `crs_defn` is present on a config level it **wins over a numeric `srid`
on the same level**. The definition is registered under a *synthetic SRID*
(see below) that flows through the regular reprojection path — no provider
treats it as a database-side SRS.

## Precedence

```
layer crs_defn > layer srid > provider crs_defn > provider srid >
source auto-detect > defaults
```

Source auto-detect is provider-specific and shared across all standard
providers (not just MySQL):

- **postgis** — `geometry_columns` / spatial metadata for `tablename` layers;
  MOS layers carry no native spatial metadata, so their CRS comes from the
  `MapplGIS LayerInfo` projection (below). Concretely: for a plain table
  layer (no `sql`), native geometry format, when neither the provider nor
  the layer has an explicit `srid`/`crs_defn`, Tegola runs
  `Find_SRID('<schema>', '<table>', '<geom field>')` (schema defaults to
  `public`) and uses the result as the layer source SRID. If the lookup
  fails — unknown table/column, unregistered or mixed SRID — provider
  creation fails with a controlled error advising an explicit `srid` or
  `crs_defn`; it does **not** silently fall back to 3857. Custom-SQL layers
  and raw geometry formats (`wkb`/`wkt`/`mos`) have no single table column
  to introspect, so they keep the documented defaults.
- **gpkg** — `gpkg_contents.srs_id` for `tablename` layers; for custom-SQL
  layers the srs id of the sampled row's GeoPackage binary header is used as
  a fallback (only when no explicit provider `srid` is configured). Raw
  format tables (`wkb`/`wkt`/`mos`) carry no GeoPackage metadata, so the MOS
  system-info projection applies there too.
- **mysql** — `MapplGIS LayerInfo` projection (MOS layers).
- **hana** — `geom.ST_SRID()` sampled from the geometry column; raw-format
  MOS layers use the system-info projection (see
  [geometry-formats.md](geometry-formats.md)).

Bounds-backed MOS custom SQL builds its `!BBOX!` predicate in the layer's
source CRS and then converts the tile extent into the quantized raw MOS
units (floor/ceil scaling by `10^precision / unit factor`). Unlike a
MapplGIS table, `sql-sample` storage detection never decodes a
`MapplGIS LayerInfo projection`, so a `mos` custom-SQL layer requires an
explicit `srid` or `crs_defn` and fails at startup without one.
`mos_precision` / `mos_units` are optional and fall back to the normative
paired defaults (see [geometry-formats.md](geometry-formats.md#mos-quantization)).

The `MapplGIS LayerInfo projection` blob is the shared source-derived CRS for
layers of **every** standard provider whose table was identified as a
MapplGIS table at registration (table-canonical structural detection: DDL +
primary key + required indexes + an `OKEY = 1` probe row; custom `sql`
layers are subject only to `sql-sample` storage detection, which never
applies system info from result rows). When no explicit
`srid`/`crs_defn` is configured, the projection is registered
as a synthetic SRID through the same mechanism as `crs_defn` (see
[`provider/crsconfig.ApplySystemInfoCRS`](../provider/crsconfig/crsconfig.go)).
Provider-native metadata is consulted only in the provider's native geometry
paths; raw-format layers always fall back to the system-info projection.

An explicit `srid` or `crs_defn` (at either level) always suppresses the
source-inferred value.

## Synthetic SRIDs

SRIDs at or above `basic.SyntheticSRIDMin` (340000001) are reserved for
Tegola-internal synthetic codes allocated by `basic.RegisterProj4Defn` for
`crs_defn` values. They exist only inside the Tegola process:

- never send them to a database spatial API (`ST_SetSRID`, `NEW ST_POINT(.., srid)`,
  round-earth checks, and so on);
- never configure them as a plain numeric `srid`;
- reprojection of features out of a synthetic CRS happens on the client side
  via the registered PROJ.4 definition.

The trimmed definition determines a stable hash-based ID. If another definition
already owns that ID, registration fails without changing existing layers or
cached transformations. Re-registering the same definition is idempotent.
`RegisterProj4SRID` also rejects replacing a synthetic ID owned by another
definition; callers using explicit registrations must choose a different ID.

## Built-in CRS registry

The fork ships a built-in registry of commonly used projected CRSs
(`basic.RegisterBuiltinProj4SRIDs`, `basic/epsg.go`), so `srid: 32637` works
even when the backend itself has no such database SRS registered. The registry
is backend-independent and shared by all providers listed above.

## `!BBOX!` semantics

The `!BBOX!` token is always evaluated **in the layer's source CRS**:

- providers that can push spatial predicates to the database convert the tile
  extent from Web Mercator into the source CRS first (e.g. PostGIS
  `ST_SetSRID(geom, 0) && !BBOX!`, HANA `ST_IntersectsRectPlanar`);
- providers/geometry formats that cannot (MOS blobs, raw WKB/WKT columns,
  synthetic CRSs on HANA) replace `!BBOX!` with a no-op and apply an exact
  in-memory bbox filter after decoding.

## Scale tokens

**HANA, PostGIS, GeoPackage and MySQL** compute `!PIXEL_WIDTH!` and `!PIXEL_HEIGHT!` from the unbuffered
tile extent transformed into the resolved layer CRS, divided by the tile's
pixel dimensions. `!SCALE_DENOMINATOR!` converts the horizontal pixel width
to meters and divides by the OGC standard pixel size of 0.00028 m. Projected
CRSs use their PROJ.4 linear units; EPSG:4326 and supported `longlat`
definitions use the source ellipsoid's local parallel-arc length per longitude
degree at the transformed tile center:
`N(phi) * cos(phi) * pi / 180`, where
`N(phi) = a / sqrt(1 - e^2 * sin^2(phi))`, `a` is the semi-major axis and `e^2`
is the squared eccentricity. Both ellipsoid parameters and latitude belong to
the resolved layer CRS, including geographic `crs_defn` definitions with
`+towgs84`. This is a local horizontal scale; a large tile is represented by
its center rather than a finite geodesic or diagonal distance. Pixel dimensions
remain in source degrees. Singular pole latitudes and unknown CRS/unit definitions
cause a query error for executable
scale tokens. Default EPSG:3857 SQL values are unchanged. See the
[HANA scale token contract](../provider/hana/README.md#scale-tokens) for tile
sizes, projection-scale interpretation, and geographic limitations.

All four providers share `provider.TileScale`. The default pixel dimensions
are 256 by 256; custom tile implementations can expose `PixelSize()`.
Queries without executable scale tokens do not require scale-unit resolution.
MySQL custom SQL whose CRS inspection is deferred must set `srid` or
`crs_defn` when using scale tokens: row-header discovery happens after SQL
execution and cannot establish the units needed to construct that query.
For PostGIS, GeoPackage and MySQL, non-WebMercator scale thresholds may need
adjustment when upgrading from the earlier WebMercator-only behavior.

## Geographic PROJ definitions

`crs_defn = "+proj=longlat +datum=WGS84 +no_defs"` is supported, including
the `latlong`, `lonlat` and `latlon` aliases. Coordinates remain longitude,
latitude in degrees; registration allocates a synthetic SRID as for projected
definitions. Scale tokens recognize these synthetic geographic SRIDs.

The Go `proj` fork applies its existing `datumToWGS84` / `datumFromWGS84`
transformations to geographic coordinates as well as projected coordinates.
Source longitude/latitude is converted through geocentric coordinates using
the source ellipsoid and a three- or seven-parameter `+towgs84` transformation;
the reverse path converts WGS84 back to the source datum. For example:

```toml
crs_defn = "+proj=longlat +ellps=bessel +towgs84=41,-107.6,-93,0,0,0,0 +no_defs"
```

Supported definitions can name a datum in the Go fork's datum table with a
built-in three- or seven-parameter transformation, or specify an ellipsoid
and `+towgs84` explicitly. Names accepted by the full PROJ library are not
necessarily present in this table. Custom ellipsoids may use `+a` with the
supported `+b`, `+rf`, `+f`, `+es` or `+e` parameters. An explicit WGS84 ellipsoid needs no datum
shift. A named datum cannot be combined with a conflicting `+ellps` or
numeric ellipsoid overrides; define the ellipsoid and `+towgs84` without
`+datum` for such custom transformations. Unknown datums and other ellipsoids
without a defined transformation are rejected rather than treated as WGS84.

The coordinate interface is two-dimensional: each direction assumes zero
input ellipsoidal height and discards the transformed height. Consequently,
forward/inverse round trips with a datum shift can have small differences.
Coordinates must remain longitude/latitude in degrees, with Greenwich as
the prime meridian and east/north axes (`enu`). Grid transformations, other
prime meridians, angular-unit conversions and axis changes remain unsupported
by this Go fork and are rejected at registration. These are implementation
limits of the fork, not limitations of `crs_defn` or the PROJ library generally.

## Provider support matrix

| Provider | `srid` | `crs_defn` | Notes |
|---|---|---|---|
| postgis | yes | yes | Synthetic CRS via `ST_SetSRID(geom, 0) && !BBOX!` (no database SRS needed). |
| gpkg | yes | yes | Explicit config wins over `gpkg_contents.srs_id` (table layers) and the sampled header srs id (custom-SQL layers). |
| mysql | yes | yes | `MapplGIS LayerInfo projection` applies only when no explicit CRS is configured. |
| hana | yes | yes | `crs_defn` requires a raw `geometry_format` (`wkb`/`wkt`/`mos`); native `ST_Geometry` columns use a database-side SRS. Not supported for MVT providers. Round-earth SRSs are used through their planar-equivalent SRIDs (`PLANAR_SRID_OFFSET = 1000000000`). |

## Feature API CRS identifiers

The feature API uses exact CRS identifiers from each collection's `crs` list.
Internal numeric SRIDs, including synthetic IDs, are never public EPSG claims.
Eligible collections expose a uniform `storageCrs` that belongs to the same list.
Mixed XY/XYZ storage has no single dimensional storage CRS. Lists are detached,
deterministic and collection specific; there is no inherited global CRS list.

| Identifier/profile | Wire coordinates |
| --- | --- |
| `http://www.opengis.net/def/crs/OGC/1.3/CRS84` | Longitude, latitude in degrees |
| `http://www.opengis.net/def/crs/EPSG/0/4326` | Latitude, longitude in degrees |
| `http://www.opengis.net/def/crs/EPSG/0/3857` | Easting, northing in metres |
| Advertised WGS84 UTM EPSG identifier | Easting, northing in metres |
| `http://www.opengis.net/def/crs/OGC/0/CRS84h` | Longitude, latitude, WGS84 ellipsoidal height in metres |
| `http://www.opengis.net/def/crs/EPSG/0/4979` | Latitude, longitude, WGS84 ellipsoidal height in metres |
| Advertised application UUID identifier | The locally documented axes, units and height reference |

XY collections publish their CRS84 default and proven two-dimensional targets.
XYZ/mixed collections publish CRS84h and proven height-bearing targets. The
supported target set contains the applicable geographic default, its EPSG axis
variant, Web Mercator and the proven source UTM/storage profile. Knowing another
UTM identifier does not automatically advertise it for every collection.
Explicit unlisted identifiers return 400. With `bbox-crs` absent, the existing
four-coordinate CRS84 and six-coordinate CRS84h interpretation remains available
independently of the explicit output catalog.

Use `crs` for response geometry on items or a single item, and `bbox-crs` for
the bbox query frame. Successful feature GET/HEAD responses identify their
output with `Content-Crs: <URI>`, including defaults and empty pages. Bbox axes
follow the requested CRS. Exact geometry intersection in that frame precedes
paging and counts; source-envelope pruning is not a replacement for this check.
Cross-CRS queries may inspect the source through a bounded cursor when safe
indexed pruning is unavailable.

Application identifiers use the registered `urn:uuid:` namespace with UUIDv8,
derived from a versioned SHA-256 descriptor of the complete effective horizontal
definition, datum, wire axes, units, dimension and vertical reference. The full
digest guards collisions. They contain no process-local SRID. Detached service
descriptor metadata explains each locally resolved identifier; requests never
fetch, register or reinterpret a URI. Equivalent definition spellings need not
have the same application identifier.

Part 2 feature transformations use owned projection instances for admitted canonical
WGS84 profiles and the bounded custom horizontal profile below. Numeric EPSG identity alone cannot prove a source definition,
because the legacy registry permits overrides. Unsupported custom/datum profiles
retain Core and tile behavior while optional Part 2 metadata and explicit CRS
requests remain unavailable. Projection-domain failures never return source
coordinates silently. Heights are neither invented nor discarded: mixed output
retains XY children with missing height and XYZ children with their actual Z.
Nonplanar transformed XYZ polygon surfaces remain unsupported.

Native storage also needs an identity proof. GeoPackage native sources with an
explicit admitted CRS configuration use that frozen coordinate declaration;
automatic native SRID detection alone does not publish Part 2 metadata. MySQL
native SRS IDs and axis options do not prove the full SRS definition, so its
native profile retains Core without Part 2 publication. Raw admitted
sources have a separate explicit coordinate declaration. Provider-specific native
proofs and unsupported profiles must be evaluated separately; a successful raw
query does not establish native Part 2 support.

Native PostGIS admission compares the complete authority, WKT and PROJ tuple
against the pinned PostGIS 3.4.1 definitions for the admitted WGS84 profiles,
then rechecks the tuple in the query snapshot. Native HANA admission uses its
existing approved full SRS tuple. A changed or unknown native definition does
not acquire canonical identity merely because its numeric code is familiar.

See [ADR-0010](architecture/decisions/ADR-0010-public-crs.md) for the
identifier and immutable transform boundaries, and [API parameters](api.md#referenced-crs-requests)
for request behavior. Implemented behavior does not establish OGC certification.

### Custom feature CRS

An explicit `+proj=etmerc` definition can be used for horizontal feature reads
and MOS writes in MySQL and the GeoPackage provider. Use `+datum=WGS84`, or an
explicit `+ellps` together with three or seven `+towgs84` parameters. Seven-parameter
shifts use the position-vector convention: translations in metres, rotations in
arcseconds, and scale in parts per million. `lat_0`, `lon_0`, `k` or `k_0`, `x_0`,
`y_0`, `units` or `to_meter`, `pm`, and inert `no_defs` are admitted. Conflicting
scale/unit declarations, unknown parameters, axis overrides, grid shifts,
vertical modifiers and other custom projection operations are rejected.

The converter owns its definition and mathematical state; changing the legacy
global SRID registry cannot redirect an admitted feature transformation. Custom
definitions have no canonical EPSG identity (`CanonicalSRID` is zero). Equivalence
compares the complete admitted parameter set, ignoring only ordering, whitespace
and `no_defs`; matching synthetic numeric IDs do not establish equivalence.

Datum conversion is horizontal and assumes zero ellipsoidal height. It does not
admit custom XYZ writes or prove a vertical datum. Public CRS publication still
requires the separate source declaration and dimensional admission rules above.
Because each direction starts at zero height in its own datum, a horizontal
forward/inverse round trip can retain a small residual even without MOS
quantization. Validate against an independent transform using the same 2D
datum convention and quantization grid; a unit-dependent angular comparison to
the original input is not a sufficient accuracy test.
An unchanged-geometry REST PUT avoids accumulating that round-trip residual by
retaining native geometry under a checked revision; see the
[MOS write contract](geometry-formats.md#mos-writes-with-separate-bounds-columns).
Synthetic Bessel fixtures check forward/inverse results against PROJ 9.5.1,
including three/seven-parameter shifts, kilometre units and a Paris prime
meridian; they do not certify arbitrary local coordinate systems.

## See Also

- [Provider contract](provider-contract.md) — provider-level and layer-level CRS keys
- [Geometry formats](geometry-formats.md) — geometry decoding and projection
- [Configuration](configuration.md) — TOML configuration examples
