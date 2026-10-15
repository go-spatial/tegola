# MultilevelCache

The `multilevel` cache combines two backends into a two-level cache:

- **L1** — the [memory](../memory) backend, hot in-process storage.
- **L2** — the [file](../file) backend, persistent on-disk storage.

## Configuration

```toml
[cache]
type = "multilevel"

[cache.memory]
max_zoom = 18
ttl = 60

[cache.file]
basepath = "./cache/maps"
max_zoom = 22
ttl = 86400
```

Both nested sections are required; each backend receives its own independent
configuration, including its own `max_zoom` and `ttl` values.

## Behaviour

- **Reads** check memory first. On a memory miss (or a recoverable memory
  error) the file backend is consulted; a file hit is promoted to memory.
  Promotion is best-effort: a failed promotion is logged while the file hit
  is still returned to the caller.
- **Writes and purges** are sent to both levels; an error from either level
  is returned to the caller.
- A non-cancellation memory read error is logged and read falls back to file;
  if both reads fail, both errors are returned.

This gives you process-local read latency with restart-safe persistence.
For distributed deployments pair the same on-disk layout with
[s3](../s3)/[gcs](../gcs) instead of the file backend by mounting them, or
use a shared cache backend ([redis](../redis), [s3](../s3),
[azblob](../azblob)) directly.
