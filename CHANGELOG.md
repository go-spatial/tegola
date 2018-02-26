## 0.6.0 (2018-02-26)

- `provider/postgis`: Added: connection parameterization for tegola unit-test suite (#221)
- `provider/postgis`: Fixed: Using !ZOOM! token can cause nil geom type on style generation (#232)
- `provider/postgis`: Refactor postgis provider to use provider.Tiler interface (#265)
- `provider/gpkg`: Add GeoPackage as a Provider (#161)
- `wkb`: Fixed: WKT for collection doesn't do much (#227, @remster)
- `server`: Fixed: A GET request for a Tile with a negative row value is successful (#229)
- `server`: Fixed: Tile request returns 200 when using invalid map (#250)
- `internal/log`: Added: Logger outputs file:line of log/standard.go along with timestamp in output. (#231)
- `tegola`: Fixed / Added: Configurable tile buffer (#107)
- `config`: Added: Support environment variables in config file (#210)
- `config`: Added: Support for turning off simplification per layer (#165)
- `server`: Added: Configurable CORS header (#28)
- `server`: Fixed: Tile cache middleware not receiving response code 200 (#263)
- `server`: Fixed: /maps/:map/:layer/:z/:x/:y not filtering to correct layer (#252)
- `server`: Fixed: style generator handling of nil geoms (#302)
- `server`: Removed configurable request logger in server package (#255)
- `server`: Added: Configurable layer simplification (#165)
- `mvt/feature`: Fixed: 2 pt lines are being disregarded (#280)
- `maths/clip/`: Fixed: Line clipping panics when linestring has 0 points (#290)
- `cache/file`: Fixed: Caching at higher levels than specified by maxZoom (#311)
- `cache/s3`: Fixed: Caching at higher levels than specified by maxZoom (#311)
- `cache/redis`: Added: Redis cache support (#300 - @ear7h)
- `encoding/geojson`: Added: geojson data types and encoding. (#288)
- Write Dockerfile to build tegola & create minimal deployment images (#244)
- Wire docker image build into CI (#245)
- Fixed: clipping & simplification bugs (#282)
- `Documentation`: Document the layer name property in the example config (#333 @pnorman)

**Additional Notes**
- tegola now has a public docker image which can be found at https://hub.docker.com/r/terranodo/tegola/. 

## 0.5.0 (2017-12-12)

- Added: Command line `cache seed` and `cache purge` commands (#64)
- Added: Support for Amazon S3 as a cache backend (#64)
- Added: More robust command line interface (#64)
- Added: No-Cache headers to `/capabilities`, `/capabilities/:map_name` and `/maps/:map_name/style.json` endpoints. (#176)
- Fixed: Possible Panic if a feature without an ID is added before a feature with an ID; when constructing Layers (#195)

Breaking changes:
- To use tegola as a web server, use the command `tegola serve --config=/path/to/config.toml`

## 0.4.2 (2017-11-28)

- Fixed: Performance affected by unused log statements (#197, @remster)

## 0.4.1 (2017-11-21)

- Fixed: regression in providers/postgis. EXECUTE_SQL environment debug was dropped.
- Fixed: Filecache: concurrent map read and map write on Set() (#188)
- Fixed: Filecache: invalid fileKey on cache init (Windows) (#178)
- Fixed: Clean up context canceled log (#170)

## 0.4.0 (2017-11-11)

- Fixed: configurable max_connections param for PostGIS provider
- Fixed: 504 returned when attempting to retrieve a tile at negative zoom (#163)
- Fixed: Using WGS84 yields squishes tiles along Y-axis (#156)
- Fixed: Capabilities endpoints not returning zoom range for all layers with the same name (#153)
- Fixed: Default config.toml not found in (#157)
- Fixed: Config validation fails when layers are overlapping but in different map configs (#158)
- Fixed: PostGIS: hstore tags should not override column tags (#154)
- Added: Filesystem cache (#96)
- Added: Clipping & Make Valid (whew!) (#56)

## v0.4.0-beta (2017-10-09)

- Fixed: Panic when PostGIS tries to query a layer that does not exist (#78)
- Fixed: Viewer not indicating colors correctly for polygons (#146)
- Fixed: stacked scrollbars showing in the embedded viewer (#148)
- Fixed: Invalid tilejson scheme (#149)
- Added: Support for X-Forwarded-Proto (#135, @mojodna)
- Added: Support for user defined layer names (#94)
- Updated: MVTProvider interface to return LayerInfo (#131)

## v0.4.0-alpha (2017-08-21)

- Added: hstore support for PostGIS driver. (#71)
- Added: experimental clipping support. (#56). To enable set the environment variable TEGOLA_CLIPPING=mvt
- Added: !ZOOM! token support for PostGIS SQL statements. (#88)
- Added: Support for debug=true query string param in /capabilities endpoints. (#99)
- Added: Config validation for layer name collision. (#81)
- Added: "center" property to map config (#84)
- Added: "bounds" property to map config
- Added: "attribution" property to map config
- Added: Support numeric (decimal) types (#113)
- Added: Configurable Webserver->HostName with fallbacks (#118)
- Added: AddFeatures performance improvements (#121)

## v0.3.2 (2017-03-13)

- Changed: MVT version from 2.1 to 1 per issue (#102)

## v0.3.1 (2017-01-22)

- Enhanced the /capabilities endpoint with bounds, center,tiles and capabilities values.
- Added: /capabilities/:map_name endpoint which returns TileJSON about a map.
- Added: configuration values for map -> center and map -> bounds. These values will be included in the /capabilities and /capabilities/:map_name responses.
- Fixed: bug where the HTTP port was not being read correctly from the config file.
- Added: http(s) prefix to tile URLs returned the /capabilities endpoints

## v0.3.0 (2016-09-11)

- Support for fetching individual layers from a map (i.e. /maps/:map_name/:layer_name/:z/:x/:y)
- Added a `/capabilities` endpoint with information about the tegola version, maps and map layers.
- Fixed an issue where the TOML config parser was not reporting config syntax errors.

## v0.2.0 (2016-08-16)

- Fixed: issue with PostGIS driver not handling nil tag values.
- Fixed: issue building PostGIS queries when tablename is used instead of sql in config file.
- Fixed: issue when table field names could be Postgres keywords.
- Added: concurrent layer fetching from data providers.
- Added: remote config loading over http(s).

## v0.1.0 (2016-07-29)
