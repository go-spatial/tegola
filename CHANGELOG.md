## 0.5.0 (2017-12-XX)
- Added: Command line `cache seed` and `cache purge` commands
- Added: More robust command line interface

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