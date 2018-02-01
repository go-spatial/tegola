# FileCache

File cache uses a file system for cachine tiles. To use use it, add the following minimum config ot your tegola config file:

```toml
[cache]
type="file"
basepath="/tmp/tegola-cache"
```

## Properties
The s3cache config supports the following properties:

- `basepath` (string): [Required] a location on the file system to write the cached tiles to.
- `max_zoom` (int): [Optional] the max zoom the cache should cache to. After this zoom, Set() calls will return before doing work.