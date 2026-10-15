# MemoryCache

The `memory` cache keeps tiles in the running Tegola process. It is the
simplest backend: no external services, no persistence — the cache lives and
dies with the process and is shared by all maps and layers.

## Configuration

```toml
[cache]
type = "memory"
# Optional: do not cache tiles above this zoom (defaults to the maximum Tegola zoom).
max_zoom = 18
# Optional: lazy expiration in seconds; 0 (default) keeps entries until restart.
ttl = 3600
```

- `max_zoom` (uint): tiles with `z > max_zoom` are never inserted; reads for
  those zooms are misses.
- `ttl` (int): seconds after which an entry lazily expires on read.
  `0` disables expiration.

Total memory usage is limited only by available process memory. For caches
that must survive restarts or be shared between instances use a persistent
backend ([file](../file), [s3](../s3), [gcs](../gcs), [redis](../redis),
[azblob](../azblob)) or combine memory with [file](../file) in the
[multilevel](../multilevel) cache.
