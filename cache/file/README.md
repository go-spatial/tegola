# FileCache

filecache uses a file system for caching tiles. To use it, add the following minimum config to your tegola config file:

```toml
[cache]
type="file"
basepath="/tmp/tegola-cache"
ttl=1
```

## Properties
The filecache config supports the following properties:

- `basepath` (string): [Required] a location on the file system to write the cached tiles to.
- `max_zoom` (int): [Optional] the max zoom the cache should cache to. After this zoom, Set() calls will return before doing work.
- `ttl` (int): [Optional] time to live in seconds for cached tiles. Defaults to 0 (never expires). TTL is evaluated lazily on Get() operations - expired tiles are deleted when accessed but may remain on disk if never requested.
