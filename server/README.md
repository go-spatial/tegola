# Server

The server package is responsible for handling webserver requests for map tiles and various JSON endpoints describing the configured server. Example config:

```toml
[webserver]
tile_http_max_age = 300     # optional browser/proxy cache lifetime in seconds
port = ":9090"              # set something different than default ":8080"
ssl_cert = "fullchain.pem"  # ssl cert for serving by https
ssl_key = "privkey.pem"     # ssl key for serving by https

[webserver.tile_operations]
enabled = true                 # default false
token = "change-me-secret"     # required when enabled
rate_per_minute = 60           # default 60
max_concurrent = 4             # default 4

[webserver.headers]
Access-Control-Allow-Origin = "*"
```

### Config properties

- `port` (string): [Optional] Port and bind string. For example ":9090" or "127.0.0.1:9090". Defaults to ":8080"
- `hostname` (string): [Optional] The hostname to use in the various JSON endpoints. This is useful if tegola is behind a proxy and can't read the API consumer's request host directly.
- `uri_prefix` (string): [Optional] A prefix to add to all API routes. This is useful when tegola is behind a proxy (i.e. example.com/tegola). The prexfix will be added to all URLs included in the capabilities endpoint responses.
- `ssl_cert` (string): [Optional, unless ssl_key provided] Path to a certificate file for serving through HTTPS
- `ssl_key` (string): [Optional, unless ssl_cert provided] Path to a private key file for serving through HTTPS

### Tile HTTP cache

`webserver.tile_http_max_age` is a non-negative integer in seconds (environment
substitution is supported). Omitted or `0` preserves existing header behavior;
negative values are rejected. For example:

```toml
[webserver]
port = ":8082"
tile_http_max_age = 300
```

When positive, anonymous GET and HEAD requests without a query string receive
`Cache-Control: public, max-age=300` only for HTTP 200 vector-tile responses.
The policy is identical for rendered tiles, shared renders, cache hits, and layer
tiles extracted from seeded map tiles, including when no server cache is configured.
`Vary: Accept-Encoding` is preserved for gzip and uncompressed representations.

All other responses on tile routes receive `Cache-Control: no-store`: errors,
non-tile responses, query-dependent requests, all `?tile=...` / `?dirty...` requests,
and requests carrying Authorization or Cookie headers. Existing maintenance
`no-store` is retained. This route-specific policy overrides generic
`webserver.headers` Cache-Control and removes Expires on tile routes; capabilities,
styles, and other routes are unchanged. With the option disabled, generic headers
retain their previous behavior.

This lifetime is independent of memory/file cache TTL. Regenerating or deleting a
server-side tile does not invalidate an already fresh browser/proxy copy: clients
may continue to display it until its HTTP lifetime expires. Use a short lifetime
for frequently edited data; the server does not add ETag/Last-Modified validators
or implement conditional 304 responses as part of this option. Reloading configuration
requires restarting the server. Public caching is intended for tiles shared by all
clients; deployments with identity-aware proxies must account for that proxy's
cache policy separately.

### Tile operations (`[webserver.tile_operations]`)

The tile endpoints accept maintenance parameters: `?tile=update` and `?tile=getupdated` (cache regeneration), `?tile=status` (cache state) and `?dirty=true` (force regeneration, bypassing the cache). Both update operations schedule metatile regeneration (up to 64 tile renders and cache writes) on a bounded background worker pool. `?tile=update` returns `202 Accepted` after enqueueing. `?tile=getupdated` waits for a render of the requested tile and returns that tile while the metatile continues in the background. Maintenance operations are **disabled by default** and, when enabled, require an authentication token.

When feature or WFS writes are enabled, ordinary viewing uses the configured server-side cache while HTTP responses remain `no-store`. Ordinary requests carrying `X-Tegola-Editor-Active: true` (or `1`) bypass the server-side cache. This header grants no authorization. Successful and uncertain commits advance a router-local cache generation for all maps; in-flight old renders cannot repopulate the new generation. Authenticated `tile=status`, `tile=update` and `tile=getupdated` operations use the same generation independently of the editor header. Writable caches start cold after restart; expired generations require backend expiry or cleanup. External SQL changes and other processes are not invalidated; see [operating limits](../docs/wfs-scope-limitations.md).

- `enabled` (bool): [Optional] Enables the tile maintenance parameters above. Defaults to `false`. When disabled, any request carrying `?tile=...` or a regenerating `?dirty` (see below) is rejected with `403 Forbidden` — the parameters are not silently ignored — and no tile is rendered or written to the cache. Ordinary tile serving is unaffected.
- `token` (string): [Required when `enabled` is true] Shared secret clients must send in the `X-Tegola-Tile-Operations-Token` header. A missing or wrong token yields `403 Forbidden`; if `enabled` is true and no token is configured, all tile operations fail closed with `403`.
- `rate_per_minute` (int): [Optional] Maximum number of tile-operation requests accepted per minute. Defaults to `60`. Requests over the limit receive `429 Too Many Requests`.
- `max_concurrent` (int): [Optional] Maximum number of tile operations executing at the same time. Defaults to `4`. Additional mutating requests (`?tile=update`, `?tile=getupdated`, `?dirty=true`) receive `503 Service Unavailable`; `?tile=status` is exempt from the concurrency slot as it never renders. `?tile=update` holds its slot through enqueueing and response; `?tile=getupdated` also holds it while waiting for the requested tile. Background metatile regeneration has its own bounded scheduler.

Note: `max_concurrent` limits tile *operations*; requests over the limit fail fast rather than queue, so a burst of cache-maintenance traffic cannot exhaust the server.

Note: only the regenerating `?dirty` variants (`?dirty`, `?dirty=1`, `?dirty=true`, case-insensitive) are tile operations. Falsy values such as `?dirty=0` or `?dirty=false` request no regeneration: they are treated exactly like a request without the parameter, are served as ordinary cache queries, and never require the token or the `enabled` gate.

### Tile format

Tile output is MVT (`.pbf`). A missing suffix defaults to `pbf`; another
suffix, including `.json`, logs a warning and still returns MVT for backward
compatibility. A different suffix does not select another output format.

Dirty regeneration applies when `dirty` is the only query parameter.
Additional query parameters follow the uncached query path; do not combine
`dirty` with filter parameters expecting the canonical cache entry to change.

## Local development of the embedded viewer

Tegola's viewer lives in `ui/`. Build its assets with Node/npm before compiling
Tegola; `ui/embed.go` embeds `ui/dist` directly. No generated Go source file is
needed. From the repository root:

```
npm --prefix ui ci --ignore-scripts --no-audit --no-fund
npm --prefix ui run build
git restore -- ui/dist/.keep
```

## Disabling the viewer

The viewer can be excluded during building by using the build flag `noViewer`. For example, building tegola from the `cmd/tegola` directory:

```bash
go build -tags "noViewer"
```

### Serving seeded map tiles through layer URLs

`cache seed --map MAP` writes `MAP/z/x/y`, while a layer URL normally reads
`MAP/LAYER/z/x/y`. On a layer-cache miss, serve now checks the whole-map tile
and selects the requested MVT layer without querying a provider or rendering
geometry again. This is reported as `Tegola-Cache: HIT`. A separately cached
layer takes precedence. The extracted response is not persisted, so it cannot
extend the map tile TTL. Unsupported/ambiguous aliases and invalid cached
content fall back to normal rendering; parameterized requests retain their
existing cache bypass. Only layers configured at the requested zoom qualify.

For file and multilevel caches, compare the absolute `file cache: basepath=`
startup messages from seed and serve. Relative paths use the process working
directory. Seeded tiles expire according to the file TTL; `ttl = 0` disables
expiry. Seeding a map does not precompute arbitrary query-parameter variants.
