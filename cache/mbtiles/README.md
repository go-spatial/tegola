# MBTilesCache

MBTilesCache uses a .mbtiles file for caching tiles. To use it, add the following minimum config to your tegola config file:

```toml
[cache]
type="mbtiles"
basepath="/tmp/tegola-cache/"
```

## Properties
The MBTilesCache config supports the following properties:

- `basepath` (string): [Required] a location of the mapname.mbtiles file to write the cached tiles to.
- `max_zoom` (int): [Optional] the max zoom the cache should cache to. After this zoom, Set() calls will return before doing work. This set the metadata maxzoom
- `min_zoom` (int): [Optional] the min zoom the cache should cache to. Before this zoom, Set() calls will return before doing work. This set the metadata minzoom
- `bounds` (int): [Optional] the bounds to register in the cache. This set the metadata bounds. Default to earth bounds: -180.0,-85,180,85.
