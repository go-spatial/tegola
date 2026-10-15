[← Documentation Index](README.md) · [Back to README](../README.md) · [Configuration →](configuration.md)

# API Reference

This page documents the HTTP endpoints exposed by Tegola.

## Server Endpoints

```
/
```

The server root serves the built-in viewer when UI assets are included in the
binary. It uses the configured maps and generated style; it is separate from
private application clients and their geometry editor plugins.

```
/maps/:map_name/:z/:x/:y
```

Return vector tiles for a map. The URI supports the following variables:

- `:map_name` is the name of the map as defined in the `config.toml` file.
- `:z` is the zoom level of the map.
- `:x` is the column of the tile at the zoom level.
- `:y` is the row of the tile at the zoom level.

```
/maps/:map_name/:layer_name/:z/:x/:y
```

Return vector tiles for a map layer. The URI supports the same variables as the map URI with the additional variable:

- `:layer_name` is the name of the map layer as defined in the `config.toml` file.

## Tile cache policy

Ordinary query-free tiles use the configured server-side cache. A layer request
can also be satisfied by extracting its layer from a cached whole-map tile.
`Tegola-Cache: HIT`, `MISS` or `SHARED` identifies the ordinary cached request's
result (`SHARED` joins an in-flight render); bypassed requests can omit it.
`cached` in a status response describes current cache availability, not whether
an editor UI happens to be open. A first request or expired/evicted entry can
legitimately miss even when no edits occurred.

A write-enabled router keeps ordinary caching and returns HTTP `no-store` to
prevent browsers from concealing source changes. For ordinary tile requests,
send `X-Tegola-Editor-Active: true` or `1` to bypass server-cache reads and writes
while editing. This header has an effect only in a write-enabled router and
confers no mutation or maintenance authorization. Refresh the client's loaded
tile sources after a save; a server invalidation cannot remove OpenLayers tiles
already held in memory.

Successful and unknown commit outcomes advance the router's cache generation for
all maps. Renders started earlier cannot populate the new generation. Writable
routers start with a new cache namespace after restart, so their first requests
are cold even with persisted files. External SQL changes and multiple processes
need separate invalidation; see [operating limits](wfs-scope-limitations.md).

## Explicit tile cache maintenance

Tile cache operations can be requested on the map-layer endpoint with the `tile`
query parameter. The query parameter is never included in the tile cache key:

```
/maps/:map_name/:layer_name/:z/:x/:y?tile=status
/maps/:map_name/:layer_name/:z/:x/:y?tile=update
/maps/:map_name/:layer_name/:z/:x/:y?tile=getupdated
```

- `?tile=status` returns JSON describing whether the requested tile is cached
  and whether its aligned 8x8 metatile is currently being regenerated:

  ```json
  {
    "map": "roads",
    "layer": "primary",
    "z": 12,
    "x": 1234,
    "y": 567,
    "cached": true,
    "updating": false,
    "metatile": [1224, 560, 8, 8]
  }
  ```

  For a layer, `cached` also checks the cached whole-map fallback. It does not
  imply that the whole metatile is present or that a request has completed.

  The `metatile` array contains the metatile origin (`x`, `y`) and its
  effective width and height. At low zoom levels the dimensions can be
  smaller than 8x8 because they are clipped to the valid tile range.
- `?tile=update` schedules regeneration of every tile in the aligned 8x8
  metatile in the background and returns `202 Accepted` immediately. The
  response body is empty. Regeneration is single-flight per metatile: requests
  that arrive while a regeneration is running join it instead of starting
  another one. `?tile=status` reports `"updating": true` while a regeneration
  is running. If the background regeneration fails it is logged at WARN and
  the next request retries it.
- `?tile=getupdated` renders and returns the requested tile immediately as an
  MVT response, and schedules the same background metatile regeneration as
  `?tile=update`.

Maintenance is disabled by default. Enable `[webserver.tile_operations]` and
supply its configured token in `X-Tegola-Tile-Operations-Token` for status,
update, getupdated and dirty regeneration. Disabled operations or a missing/wrong
token return 403; the rate limit returns 429 and the concurrency limit returns
503. The editor-active header
does not bypass this gate. See [cache configuration](configuration.md#tile-cache-maintenance-and-editing).

The `tile` operation cannot be combined with other query parameters.
Operation responses are marked `Cache-Control: no-store`.

A cache backend must be configured for `?tile=update` and `?tile=getupdated`
to work; without a cache these requests fail. `?tile=status` works without a
cache and reports `"cached": false` in that case.

Tile regeneration can also be forced with the `dirty` query parameter:

```
/maps/:map_name/:layer_name/:z/:x/:y?dirty
/maps/:map_name/:layer_name/:z/:x/:y?dirty=1
/maps/:map_name/:layer_name/:z/:x/:y?dirty=true
```

- The parameter is detected by its presence; empty values and the values
  `1` / `true` (case-insensitive) force regeneration, other values are
  ignored.
- The regenerated tile is written back to the cache only when `dirty` is the
  sole query parameter. Combined with any other query parameter the request
  bypasses the cache entirely (read and write).

```
/capabilities
```

Return a JSON encoded list of the server's configured maps and layers with various attributes.

```
/capabilities/:map_name
```

Return [TileJSON](https://github.com/mapbox/tilejson-spec) details about the map.

```
/maps/:map_name/style.json
```

Return an automatically generated [Mapbox GL Style](https://www.mapbox.com/mapbox-gl-js/style-spec/) for the configured map.

## Feature API (opt-in)

Feature publication is disabled by default. Enable `[features]` and explicitly
map public collection IDs to eligible `provider.layer` sources. The default
base path is `/features`; configured server URI prefixes also apply. Map layers
are not automatically published as collections. GeoPackage supports eligible
table-backed layers with a schema-proven unique INTEGER ID; its custom feature
SQL is unsupported. MySQL/MariaDB, PostGIS and HANA have separate source admission
rules, including constrained single-table `feature_sql`. MVT-only providers are
not eligible. See the [provider guides](provider-contract.md#raw-feature-query-contract).

The following paths are relative to the feature base path. Each supports GET
and HEAD; HEAD returns the same status and headers without a response body.

| Path | Resource | Canonical response media type |
| --- | --- | --- |
| `/` (base path itself) | Landing page and discovery links | `application/json` |
| `/api` | OpenAPI description | `application/vnd.oai.openapi+json;version=3.0` |
| `/conformance` | Advertised conformance classes | `application/json` |
| `/collections` | Explicitly published collection catalog | `application/json` |
| `/collections/{collectionId}` | Collection metadata | `application/json` |
| `/collections/{collectionId}/queryables` | Public scalar Queryables schema | `application/schema+json` |
| `/collections/{collectionId}/items` | GeoJSON FeatureCollection | `application/geo+json` |
| `/collections/{collectionId}/items/{featureId}` | One GeoJSON Feature | `application/geo+json` |

For example, the landing resource is `/features`, and collection items are
`/features/collections/roads/items`. Feature IDs in URLs are unsigned decimal
integers; GeoJSON IDs are numeric. All resources accept the representation selector
`f=html|json`. Discovery resources accept no other query parameters; the
single-feature resource additionally accepts `crs`.

An absent `Accept` header selects canonical JSON. Successful resources also offer
`text/html; charset=utf-8`, including the API definition and Queryables. Accept
negotiation honours quality and specificity; an exact tie selects canonical JSON.
When `f` is absent, a malformed or incompatible `Accept` returns 406 before provider I/O. An explicit
matching `q=0` excludes that representation even if a broader wildcard permits it.
Feature payloads do not have an `application/json` alias.

`f=html` selects HTML and `f=json` selects the resource's canonical JSON media
type, overriding Accept. Empty, unknown or repeated `f` values return 400.
HTML displays the complete response data and protocol links using embedded,
escaped templates, without external scripts or CDN assets. Self, alternate and
paging links preserve the representation, validated query parameters and mounted
prefix. For example:

```text
/features?f=html
/features/api?f=html
/features/collections/roads/items?limit=25&f=html
```

The generated `/api` definition describes enabled collection capabilities,
actual page defaults and limits, response schemas and media types. Optional
Queryables, filtering and CRS parameters are described for collections that
support them; unavailable profiles retain their documented error behavior.
Core datetime is described for every collection.

### Item query parameters and paging

In addition to `f`, the following parameters are accepted on the items resource. Unknown or
repeated parameters return 400. This strictness is deliberate: clients must send
only documented parameters; anything else is rejected rather than silently ignored.
Spatial, temporal and property constraints combine by AND.

| Parameter | Behavior |
| --- | --- |
| `limit` | Positive decimal page size; defaults to 100. Values above the configured maximum (default 10000) are rejected with 400. Zero and nondecimal values are invalid. |
| `offset` | Nonnegative decimal offset in stable feature-ID order; defaults to 0. This is a Tegola paging extension. |
| `bbox` | Four or six coordinates in `bbox-crs`, or the Core defaults below when `bbox-crs` is absent. |
| `bbox-crs` | Exact advertised CRS identifier for the bbox; requires `bbox`. |
| `crs` | Exact advertised CRS identifier for output geometry; defaults to the collection's Core CRS. |
| `datetime` | RFC3339 instant or inclusive interval, described below. |
| `filter` | Bounded CQL2 text expression over the collection's public queryables. |
| `filter-lang` | `cql2-text`; requires a nonblank filter. This language is the default when a filter is supplied. |

Exact filtering and deduplication precede offset and limit. Responses include
`numberReturned` and paging links (`self`, `next` when more matches
exist, and `prev` when offset is nonzero). `numberMatched` is present only when
the provider knows an exact total; its absence does not mean zero. Follow the
returned links, which retain bbox, CRS, datetime and filter constraints. Paging is evaluated
against each request's source snapshot; it does not freeze data across requests.

```text
/features/collections/roads/items?limit=25&offset=0&bbox=-10,40,10,55
/features/collections/observations/items?datetime=2020-01-01T00:00:00Z/..
```

### Spatial bounds and source profiles

When `bbox-crs` is absent, four-coordinate bbox order is `west,south,east,north`, in longitude/latitude
degrees (CRS84). Six-coordinate order is
`west,south,minHeight,east,north,maxHeight`; heights are WGS84 ellipsoidal metres
(CRS84h). Coordinates must be finite, longitude must lie within −180..180 and
latitude within −90..90. South cannot exceed north, and minimum height cannot
exceed maximum height. Equal endpoints are allowed; boundaries are inclusive.
West greater than east selects an antimeridian-crossing union, with the same
height interval on both sides.

XY source members intersect the horizontal bounds with an unconstrained
vertical dimension, including under six-coordinate bbox. This is an explicit
application policy; it supplies no invented height. XYZ members retain Z and
use exact segment/box or planar polygon-surface intersection, including holes.
Four-coordinate bbox tests their XY projection while retaining output Z.
Nonplanar XYZ polygon surfaces are unsupported. Nil or valid decoded empty
spatial geometry matches a valid bbox and is returned as null geometry;
an empty child does not make a populated collection absent. Malformed geometry
remains an error.

Published sources declare `xy`, `xyz` or `mixed_xy_xyz` dimensional metadata.
Native GeoPackage derives dimensional eligibility from its schema; raw WKB/WKT
default to XY and need explicit configuration for XYZ/mixed. XYZ/mixed requires
`vertical_crs = "http://www.opengis.net/def/crs/OGC/0/CRS84h"`.
The raw feature profile supports ISO-WKB Z and WKT Z geometry families; MOS
remains XY. Actual M/ZM and EWKB profiles are unsupported. Source dimensional
declarations are checked against encountered bodies.

XYZ/mixed coordinate conversion admits canonical WGS84 EPSG:4326, EPSG:3857
and WGS84 UTM zones 32601–32660/32701–32760. Other/custom source definitions and
other vertical references are unsupported. Projection retains height exactly;
transformed vertices define straight segments and planar surfaces. If projection
makes a polygon nonplanar, the query is unsupported. Default GeoJSON uses
longitude/latitude and preserves the third height ordinate where present.
An explicit `crs` selects the advertised wire axes and projection.

### Referenced CRS requests

Eligible collections publish a deterministic `crs` list and a uniform
`storageCrs` when applicable. Use these exact identifiers: EPSG:4326 uses
latitude/longitude axes, unlike CRS84. Projected coordinates use easting/northing
metres. Bbox ordinates follow the selected identifier's axes; its dimension
determines whether four or six numbers are required. Output and bbox CRS are
independent. Exact intersection is evaluated in the bbox query frame before
paging and counts; a transformed source envelope is only candidate pruning.

The single-item resource also accepts `crs`; its other query parameters remain
invalid. Successful feature GET and HEAD responses include `Content-Crs: <URI>`,
including default and empty-page responses. Unsupported, blank, repeated or
unlisted identifiers return 400 without remote identifier resolution. Geometry
transformation failures never fall back to source coordinates. See the
[CRS contract](crs.md#feature-api-crs-identifiers) for publication and dimensional
limits. Application CRS identifiers and successful requests do not declare OGC
certification.

### Exact datetime constraints

`datetime` accepts an RFC3339 instant or `start/end` interval. One endpoint may
be open, written `..` or empty; both open and reversed ranges are invalid.
Offsets and lowercase `t`/`z` are accepted. Fractional seconds may have more than
nine digits: comparisons preserve the remaining decimal precision rather than
rounding the request to nanoseconds. Encode a positive timezone offset's `+`
as `%2B` in a query URL.

Collections without temporal geometry accept a valid datetime constraint and
match every feature. Invalid datetime syntax still returns 400 before provider I/O.

Second 60 is accepted only for a known positive leap insertion, checked after
timezone normalization. The production date table contains the 27 announced
insertions through 2016-12-31; future unannounced leap seconds are rejected and
the table requires an explicit update when new insertions are announced.

Admitted GeoPackage, MySQL/MariaDB, PostGIS and HANA temporal mappings support integer
POSIX seconds, milliseconds,
microseconds or nanoseconds. An instant finer than a source tick matches only
an exactly representable stored instant. POSIX storage has no timestamp inside
an inserted leap second, so such an instant selects no populated instant value;
intervals spanning it can still overlap. Absent temporal geometry matches valid
temporal constraints. A NULL interval endpoint is open; two NULL endpoints are
absent. Invalid stored types or reversed source intervals fail the request.
Native SQL date/time types are not implicitly
interpreted as integer POSIX time. See the backend guides below for eligibility.

### Feature properties

Selected public properties with SQL NULL values are returned as JSON `null`.
Private and unselected properties are absent. Zero, false and the empty string
retain their values. Property selection narrows the published property set and
does not remove feature identity or geometry.

### Queryables and scalar filters

Collections with an available catalog link to their Queryables resource. The Draft 2020-12 schema
has a canonical query-free `$id` and `additionalProperties:false`. It describes
eligible public scalar properties; identity, geometry and internal source
metadata are not automatically queryable. A valid catalog can be empty.
The text transport catalog omits aliases that cannot be expressed as CQL2
identifiers, without removing those properties from ordinary feature responses.

The implemented CQL2 text profile supports TRUE/FALSE, AND/OR/NOT, six scalar
comparisons and IS NULL/IS NOT NULL. Comparisons use a public property on the
left and a compatible scalar literal on the right. Numeric literals remain
exact, string comparisons preserve Unicode order and trailing spaces, and
source NULL follows three-valued logic. Filtering precedes paging and exact
counts within the provider's protected source snapshot.

Unknown/private properties and invalid or incompatible expressions return 400.
Missing optional Queryables capability returns 501 for Queryables/filtering
operations and preserves ordinary Core requests. CQL2 JSON, advanced operators
and `filter-crs` are unsupported. Native date/time or floating-point properties
require separate admission; parsing their literal types does not establish a
backend capability. See [Queryables and filtering](filtering.md) for syntax,
limits, physical eligibility and examples.

### Errors and conformance status

Invalid request parameters return 400, missing collections/features 404,
unsupported methods 405, unacceptable representations 406, cancellation/deadlines 408, and unsupported
requested operations 501. Source-row corruption and other internal failures
return generic 500 responses without exposing source details.

Feature responses, errors, OPTIONS and redirects use `Cache-Control: no-store`
and `Vary: Accept`, without ETag or Last-Modified validators. Conditional requests
do not produce 304. Responses use identity encoding; configured Content-Encoding
is removed because feature compression is not implemented. GET and HEAD select the same representation and headers;
HEAD emits no body. OPTIONS advertises GET, HEAD and OPTIONS. Feature CORS exposes
Content-Crs while retaining configured origin and credential policy.

Feature publication defaults to a 30-second cooperative deadline and a 16 MiB
encoded response cap. Exceeding the cap returns 400 `ResponseTooLarge`
(`{"code":"ResponseTooLarge","description":"Response exceeds publication limit"}`)
before a partial response is sent, including for HEAD: the request asked for
more than the publication can encode, so narrow it (smaller `limit`, tighter
`bbox`, or a more selective `filter`) and retry. These limits bound the
response, not all service or encoder allocations. A raw query above 64 KiB returns
414; aggregate Accept headers above 16 KiB return 431. Limits are applied before
provider I/O where possible. See [publication configuration](configuration.md#feature-publication).

`/conformance` returns a sorted `conformsTo` list from the admitted implementation
registry and immutable publication catalog: Core, GeoJSON, HTML and OpenAPI 3.0.
Part 2 CRS is included only when every published collection has the accepted CRS
capability. A nil or empty catalog declares no classes. Part 3/CQL2 global classes
remain withheld pending an appropriate approved validator; their implemented
collection-specific endpoints remain described by OpenAPI. See the
[reproducible conformance runner](testing/ogc-conformance.md) for pinned official
checks, report provenance and explicitly classified skips. A declaration is not
an OGC certification or a deployment-specific verification report.

Feature handlers execute behind a bounded response transaction. A recovered
panic returns fixed generic JSON 500, without partially committed data or panic
details; HEAD has the same error headers and no body. Ordinary returned context
cancellation remains 408. The standard `http.ErrAbortHandler` sentinel propagates.

## WFS and feature mutations

When explicitly enabled, `/wfs` serves the configured WFS versions, and
`/features/collections/{collection}/items` accepts POST. Individual items accept
configured PUT/PATCH/DELETE operations. PATCH accepts Merge Patch and the
implemented JSON Patch subset. `/collections/{collection}/schema` under the
Feature API base path describes mutation input; Queryables describes filtering,
not write admission. Consult the live OpenAPI document for installed methods.

Property defaults apply to omitted input, not to explicit null. Non-nullable
attributes and geometry reject null during validation. Date-time properties
require RFC3339 timestamps; their schema format is not merely a UI hint.
Announced positive leap seconds use the same validation as query literals,
including equivalent offsets; unannounced leap dates and second 61 are rejected.
Create input does not require a client feature ID: the source must generate its
primary key. PostGIS create admission requires an identity or sequence-generated
key. The admitted sequence default is a plain `nextval(regclass literal)` with
an actual sequence dependency; arbitrary expressions or `DEFAULT 1` are insufficient. An update-only collection does not acquire create capability merely
because it has an integer key.

Use the ETag from the same source-feature response as the editing draft, and
send it with If-Match. A conflict returns 412; a missing required precondition
returns 428. ETags include revision/incarnation and a representation digest;
treat them as opaque strings. Reuse the same output `crs` and format selection
(`f` or Accept) for the mutation that supplied the draft's ETag. A validator from
a different representation still returns 412 even if the revision is unchanged.
Output CRS does not select mutation input CRS: GeoJSON input remains CRS84;
another Content-Crs is rejected. Attribute-only patches preserve stored geometry.
Invalid or duplicate output CRS selectors fail before mutation.

A confirmed commit whose representation cannot be
read back returns a minimal success with `Tegola-Commit-Status: committed`.
Do not repeat the mutation to obtain its representation. Unknown outcomes also
require source reconciliation before any retry.

The [write scope](wfs-scope-limitations.md) documents unsupported locks,
cache policy, external-writer limitations and the bounded embedded editor.

### WFS names, versions and operation links

Capabilities operation links follow the same public URL policy as Feature API:
the resolved URL root, configured `webserver.uri_prefix`, and WFS `basepath`
are all included. A service mounted at `/v1/wfs` behind a configured HTTPS proxy
must advertise that public path and scheme, not an internal unprefixed URL.

Use the feature type QName advertised by capabilities, for example `app:sites`.
The QName resolves to the local published collection ID `sites`; it is not a new
collection ID. Qualified property references resolve against that collection's
application namespace. Unqualified local aliases remain accepted. An undeclared,
foreign or malformed prefix is rejected rather than stripped arbitrarily.

For XML requests declare prefixes in scope. For KVP requests an alternative
prefix can be bound with `NAMESPACES=xmlns(prefix,http://example.com/tegola/sites)`
for the `sites` collection; the WFS 1.1 `xmlns(prefix=URI)` form is also accepted.
The parameter aliases `NAMESPACE` and `NAMESPACES` are supported. The advertised
`app` prefix fallback applies only to KVP QName fields. An XML FES expression
needs its own declarations or an explicit request namespace binding; there is
no implicit `app` binding inside XML. Do not reuse a different collection's
namespace URI for a filter property.

DescribeFeatureType marks non-nullable geometry as required (`minOccurs="1"`).
Nullable geometry is optional (`minOccurs="0"`, `nillable="true"`), matching
mutation validation for all supported WFS versions.

GetPropertyValue supports the configured WFS 2.0.0 and 2.0.2 versions, not 1.1.0.
Transaction responses retain the negotiated version instead of upgrading 2.0.0
to 2.0.2. The KVP `BBOX` parameter remains supported, but an XML FES/Filter
`BBOX` predicate is outside this parser's subset and is not advertised as a
spatial filter operator. These distinct paths are not interchangeable.

FES comparison literals are bound to the published Queryables schema before
provider execution. String fields preserve exact lexical text, including `123`,
`00123`, `1e3` and whitespace. Numeric fields retain exact numeric lexemes;
boolean fields accept `true`/`false` and `1`/`0`. Dates and timestamps follow the
provider queryable's admitted lexical profile. Invalid typed values and unknown
properties fail before a query. This does not relax CQL2's own strict typing.
Literal binding does not grant provider capability: the PostGIS source JSON
profile currently rejects native DATE, TIMESTAMP, TIMESTAMPTZ and NUMERIC columns
before querying. Date/time/exact-decimal binder tests therefore do not establish
native PostGIS filtering or HTTP support for those types; no implicit cast is
introduced to admit them.

After transaction coordination, WFS responses expose `Tegola-Commit-Status`
(`committed`, `not-committed` or `unknown`) and, when available,
`Tegola-Transaction-Id`. Both are exposed through CORS. Error bodies remain OWS
XML and do not expose private receipt details. Preserve correlation headers;
an unknown outcome must not trigger automatic retry. See [recovery](operational.md).

## See Also

- [Configuration](configuration.md) — server and cache settings
- [Provider contract](provider-contract.md) — provider runtime behavior
- [CRS contract](crs.md) — coordinate reference system behavior
- [GeoPackage provider](../provider/gpkg/README.md) — raw source eligibility, temporal mappings and dimensional configuration
