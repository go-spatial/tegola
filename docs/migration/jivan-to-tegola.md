[Documentation index](../README.md) · [Compatibility matrix →](jivan-compatibility-matrix.md)

# Migrate Jivan feature clients to Tegola

This guide translates Jivan at `c9fba2bb5188ba43c27539740195085e92f0ad7d`
to the implemented Tegola Feature API. It describes migration steps, not a
release announcement or certification. Tegola replaces the feature-serving
responsibility within the supported capability boundaries below. This project
does not manage upstream Jivan: migration does not require changing its README
or archiving its repository.
Use the [15-category matrix](jivan-compatibility-matrix.md) to inventory clients.
The [pinned inventory](jivan-feature-matrix.md) identifies all registered legacy
routes and separates internal, unregistered functions from external behavior.

## 1. Inventory and preserve a rollback

Record the deployed Jivan executable revision, configuration, source tables and
geometry/CRS declarations, public collection names, representative requests,
access controls, reverse-proxy base path and Lambda integration. Preserve the
binary/configuration and a reversible proxy-routing change. Keep credentials in
the deployment's secret mechanism; exclude them from fixtures and reports.

Record expected IDs, properties, geometry, coordinate axes, NULL values and
counts independently of either server. Legacy response equality alone is not
an oracle: Jivan's EmptyTile adapter, property materialization, validator and
negotiation behavior are intentional migration differences.

## 2. Configure explicit publication

Retain or migrate the source into a supported Tegola provider layer using its
backend guide. A map styling layer does not automatically publish features.
For an already configured provider named `gpkg` with layer `roads`, add:

```toml
[features]
enabled = true
basepath = "/features"
default_limit = 100
max_limit = 10000
query_timeout_ms = 30000
max_response_bytes = 16777216
title = "Road features"

[[features.collections]]
id = "roads"
provider_layer = "gpkg.roads"
title = "Roads"
```

This is a publication fragment, not a complete provider configuration. See
[configuration](../configuration.md#feature-publication) and the
[GeoPackage](../../provider/gpkg/README.md),
[PostGIS](../../provider/postgis/features.md),
[MySQL/MariaDB](../../provider/mysql/README.md) and
[HANA](../../provider/hana/FEATURE-QUERY.md) guides for source eligibility.
Source IDs must have the backend's proven unique integer identity. Public
collection IDs use unreserved ASCII characters; bindings contain exactly one
dot between provider and layer names. MVT-only providers have no raw fallback.

### Translate the pinned configuration

The pinned `go-wfs-config.toml` is a migration input, not Tegola TOML. Translate
its fields deliberately:

| Jivan field | Tegola destination | Operator decision |
|---|---|---|
| `server.bind_host`, `server.bind_port` | `webserver.port`, for example `"127.0.0.1:9000"` | Choose the listener address; the CLI `--port` override takes precedence. |
| `server.url_scheme`, `server.url_hostport` | `webserver.hostname`, an absolute public origin URL | Set the externally reachable scheme/host; validate proxy behavior. This does not configure the listener or TLS by itself. |
| `server.url_basepath` | `webserver.uri_prefix` plus `features.basepath` | Choose one external resource path and avoid duplicating prefixes. A legacy `/jivan/` prefix can become `/jivan` with feature resources at `/jivan/features`. |
| `server.paging_limit`, `server.paging_maxlimit` | `features.default_limit`, `features.max_limit` | Preserve intended limits explicitly; pinned values are 10 and 1000, unlike Tegola defaults 100 and 10000. |
| `metadata.identification.title`, `description` | `features.title`, `features.description`; per-collection title/description | Transfer supported descriptions. Contact, fees, keyword and organization metadata have no automatic equivalent in this publication model. |
| `providers.data` | `[[providers]]` with `type="gpkg"`, `filepath`, explicit `[[providers.layers]]`; then `[[features.collections]]` | Inventory each intended table, geometry and unique ID; publish only named eligible layers. |
| `server.default_mimetype` | Client `f=json|html` or Accept | There is no default-MIME configuration translation; absent Accept uses canonical JSON. |
| `server.encoding`, `language`, `pretty_print`; `logging` | Existing Tegola deployment/logging policy | No automatic field-for-field migration; do not assume legacy formatting or localization. |

The pinned sample spells `providers.data` as
`test-data/athens-osm-20170921.gpkg`, but its source archive contains
`test_data/athens-osm-20170921.gpkg`. Resolve and verify the actual deployed file
path; do not claim the unmodified stock sample starts successfully. Back up data
before any intentional storage conversion. Use owned synthetic fixtures for
reproducible smoke tests rather than publishing a private deployment dataset.

Table-backed GeoPackage is supported; arbitrary legacy tile SQL is not a raw
feature source. Other backends admit only their documented constrained
`feature_sql` profiles. Do not translate a Jivan query into arbitrary SQL.
Selected public SQL NULL properties remain present as JSON `null`; unselected
and reserved private properties are absent. Check public aliases and eligible
Queryables after registration.

## 3. Translate client requests

The default feature origin is `/features`, combined with the configured server
URI prefix. Update clients to follow links from the landing page; preserve the
external scheme/host/base path at the proxy boundary.

| Legacy request | Replacement |
|---|---|
| `/collections/roads/items?time=2020-01-01T00:00:00Z` | `/features/collections/roads/items?datetime=2020-01-01T00:00:00Z` |
| `/collections/roads/items?name=Main` | Discover Queryables, then use `filter=name%20%3D%20%27Main%27&filter-lang=cql2-text` on the new items resource. |
| `/collections/roads/items?page=2&limit=25` | Start with `limit=25` and follow the returned `next` link. |
| `f=application/json` or `f=text/html` | `f=json` or `f=html`. |

Build URLs with an encoder, including `%2B` for a positive datetime timezone
offset. Unknown, repeated and malformed parameters return 400. An absent
`filter-lang` with a filter selects `cql2-text`; a language without a filter is
invalid. Only the advertised typed catalog can be filtered: six scalar
comparisons, Boolean composition and NULL predicates are available in the
initial profile. CQL2 JSON, advanced expressions and `filter-crs` are unsupported.
See [filtering](../filtering.md) for precise grammar and backend type limits.

`page` is unsupported. The vendor `offset` parameter exists, but links preserve
validated bbox, datetime, filter, output CRS and representation selections;
prefer those links over reconstructing offsets. Ordering is deterministic by
ID. Paging across separate requests does not guarantee one snapshot while the
underlying data changes. `numberMatched` can be absent after lookahead; absent
is not zero. `numberReturned` describes the current response.

## 4. Reconcile geometry, CRS and time

Default coordinates are CRS84 longitude/latitude for XY, or CRS84h with retained
height for XYZ/mixed sources. GeometryCollection remains a collection; features
are not tile-clipped or simplified. Valid null/wholly-empty geometry is encoded
as null and matches valid bbox constraints. Malformed geometry fails the request.
M/ZM and EWKB raw bodies are unsupported; native export profiles have separate
eligibility. MOS remains XY. Verify source dimensional metadata and vertical CRS
instead of dropping Z to make a fixture pass.

Core bbox accepts four or six coordinates with finite geographic ranges and
ordered latitude/height. West greater than east expresses an antimeridian union.
XYZ uses exact query-frame intersection, including polygon holes; XY members
have unconstrained height. Nonplanar XYZ polygon surfaces are unsupported.
For eligible collections, select only advertised `crs`/`bbox-crs` URIs.
EPSG:4326 wire axes are latitude/longitude; CRS84 axes are longitude/latitude.
Projected CRS uses easting/northing. Output and query CRS are independent.
Successful item/items GET and HEAD include `Content-Crs`, including defaults.
Unavailable/unlisted explicit CRS returns 400, without source-coordinate fallback.
See [CRS publication](../crs.md#feature-api-crs-identifiers) for exact proof limits.

Replace implicit tag-name temporal discovery with explicit backend mappings:
one instant field, or start/end interval fields, and an admitted integer POSIX
unit (seconds, milliseconds, microseconds or nanoseconds). Do not assume legacy
`start_time`, `stop_time` or `timestamp` strings are automatically interpreted.
Convert and validate stored values deliberately; native SQL timestamps are not
implicitly epoch fields. RFC3339 instants and open-ended intervals preserve
subnanosecond request precision. Known positive leap seconds have explicit POSIX
comparison semantics. Collections without temporal geometry accept valid
datetime and match all features; invalid syntax still fails before source I/O.
See [datetime details](../api.md#exact-datetime-constraints).

## 5. Update transport and deployment assumptions

JSON defaults to each resource's canonical media type: `application/geo+json`
for items/item, `application/json` for discovery, schema JSON for Queryables,
and versioned OpenAPI JSON for `/api`. HTML is embedded, escaped and self-contained;
there is no legacy CDN dependency. Accept quality/specificity is validated;
unsupported media returns 406. Explicit `f=json|html` overrides Accept.

GET and HEAD select the same status/headers; HEAD has no body. Feature responses,
redirects, OPTIONS and errors use `no-store`, `Vary: Accept`, identity encoding
and no ETag/Last-Modified. Do not carry forward Jivan's FNV name/ID validator or
expect conditional 304 responses. Tile caching remains a separate contract.

The default cooperative deadline is 30 seconds and encoded response cap is
16 MiB, including HEAD. Large scans and complex geometry may require operational
budget changes; the cap does not bound all allocations. Returned cancellation
is 408; source failures and recovered feature-handler panics are generic 500.
The standard abort sentinel propagates. Deployment access controls are required
where data is private; CORS is not authorization. Do not expose arbitrary source
layers merely to reproduce a legacy collection list.

Use the normal CLI/shared router for the replacement. Lambda adapter parity is
a separate migration check: verify API Gateway stage paths, forwarded origin,
query encoding, GET/HEAD and errors with the actual deployment event format.
Do not treat a local server smoke as deployed Lambda evidence. Track adapter
validation with the release candidate before routing production traffic.

## 6. Verify before switching traffic

1. Start Tegola with the migrated configuration on an isolated local origin.
2. Read landing, API, conformance, collection list and each collection in JSON
   and HTML; follow every genuine navigation link under the intended prefix.
3. Check known/missing IDs, bbox boundaries/antimeridian, mapped and unmapped
   datetime, NULL properties, typed filters, page links and exact/unknown counts.
4. Compare independently recorded geometry/axes/height; check all advertised CRS
   and invalid URI rejection. Exercise GET/HEAD and no-store/no-validator policy.
5. Test malformed requests, unacceptable media, unavailable capabilities,
   deadline/response cap and recovery; verify no source details reach clients.
6. Run the [conformance procedure](../testing/ogc-conformance.md), preserving
   official failures/skips and separately scoped ATS supplements. Actual
   declarations are not certification or proof of this deployment.
7. Verify Lambda separately if used, rehearse proxy rollback, then obtain the
   deployment owner's cutover approval before disabling the old service. Keep
   upstream Jivan README/repository management outside this migration.

Internal `filteredFeatures` and temporary-collection creation were unregistered
in the pinned source. They are not replacement public endpoints. A newly desired
mutable collection workflow needs its own product design and authorization.

### Reproduce the owned CLI smoke

From the repository root, use Python and a CGO-enabled Tegola executable. Both
output directories must be fresh. The preparation step creates synthetic data
and a migrated configuration; it does not read a private or legacy stock dataset.

```powershell
python scripts/ogc/migration_smoke.py --prepare D:/temp/tegola-jivan-fixture
./tegola.exe serve --config D:/temp/tegola-jivan-fixture/config.toml
```

Keep the server running and execute the check in a second terminal:

```powershell
python scripts/ogc/migration_smoke.py --origin http://127.0.0.1:19100/features --output D:/temp/tegola-jivan-receipt
```

The [owned fixture](../../testdata/migration/jivan/migrated.toml) binds loopback
port 19100; stop this test server after inspecting the generated receipt.
The [smoke tool](../../scripts/ogc/migration_smoke.py) checks literal expected
responses through the ordinary CLI. A successful local receipt establishes only
that fixture's checks; it does not establish deployed AWS parity, production
cutover approval or upstream repository retirement.

## See Also

- [Compatibility matrix](jivan-compatibility-matrix.md)
- [HTTP API](../api.md)
- [Pinned Jivan inventory](jivan-feature-matrix.md)
