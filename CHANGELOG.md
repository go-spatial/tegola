## 0.12.1 (2020-09-04)
**Bug Fixes**
* fixed the internal viewer not using the most recent version (@ARolek)

## 0.12.0 (2020-08-26)
**Features**
* proj: implemented the go-spatial/proj package (@meilinger)
* mvt_providers: brought in mvt_postgis enabling support for ST_AsMVT from postgis. (#556 @gdey)
* viewer: the maps layer list can now be hidden (@mapl)

**Bug Fixes**
* cache seeding: handle map zooms with no layers (#698 @ARolek)  
* provider/postgis: fix dropped errors (@alrs)
* server: fix dropped test error (@alrs)
* providers/postgis: When ignoring UnknownGeometryType log the issue (@gdey).

**Maintenance**
* mvt: Split out Transformation, Simplification, and Clipping from encodeGeometry (#224 @ARolek).
* mvt: remove local mvt package and vendor from geom (@ear7h)
* basic: remove tegola.Geometry type from basic.ToWebMercator (#622 @ear7h)
* basic: normalized table tests to style of table tests (@gdey)

## 0.11.0 (2020-05-04)
**Features**
* Added SSL Support (#82 @ear7h)
* lambda: global state for database connection caching (#609 @ARolek) 

**Bug Fixes**
* server: return tiles that lie on a Map's boundary (#633 @ear7h)
* postgis: use `id` table field as a tile tag (#383 @ear7h)

**Maintenance**
* atlas: remove usage of tegola.Tile (#636 @ear7h)
* atlas: use geom package simplify function (@ear7h)

## 0.10.2 (2019-08-30)
**Features**
* added tegola_lambda_cgo to the build pipeline. Geopackage can now be used with tegola_lambda (@ARolek)

## 0.10.1 (2019-09-03)
**Bug Fixes**
* server: fixed cache middleware removing incorrect path prefix when a `uri_prefix` was set. (@ARolek)
* server: fixed internal viewer URIs when a `uri_prefix` was set. (@ARolek)
* server: fixed internal viewer file paths when a `uri_prefix` was set. (@ARolek #361)

## 0.10.0 (2019-08-30)
**Features**
* cache/redis: add configurable key expiration time. #600 (@tierpod)
* server: configurable webserver URI Prefix #136 (@ARolek)

**Bug Fixes**
* server: Content-Length for non-gzipped responses (@thomersch)
* cmd: tegola command fails when running version subcommand without a config. #626 (@ear7h)

**Maintenance**
* Upgraded internal viewer Mapbox GL JS to 1.0.0 (@ARolek)
* Skip S3 tests on external pull requests (@gdey)

**Breaking Changes**
* If a `webserver.hostname` is set in the config the port is no longer added to the hostname. When setting the `hostname` it's now assumed the user wants full control of `hostname:port` combo.

## 0.9.0 (2019-04-09)
**Features**
* Add support for --no-cache command line flag override (#517 @tierpod)
* Add support for per map tile buffers (#501 @tierpod)
* Added map config option dont_clip to turn off layer clipping (#562 @paumas)

**Bug Fixes**
* Removed superfluous `sort.Int` to improve geoprocessing performance (#567 @vahid-sohrabloo)
* provider/postgis: no error thrown when geoFieldname is missing (#590 @ARolek)
* server: User defined http response headers are not added to OPTIONS requests (#594 @ARolek)
* atlas: Handle empty geometry collections (#429 @paumas, @ARolek)

**Maintenance**
* Update sqlite driver. Driver understands strings now. (@gdey)
* Updated CI to use Xenil (@gdey)

## 0.8.1 (2018-11-07)
**Bug Fixes**
- fixed double seeding when using tegola cache seed tile-list with the --map flag (#553 @arolek)

## 0.8.0 (2018-11-01)
**Features**
- provider/gpkg: use aliases and quotes in query for all column names (#486 @olt)
- provider/gpkg: improve column name extraction (#486 @olt)
- provider/postgis: added support for `!pixel_width!`, `!pixel_height!` and `!scale_denominator!` SQL tokens (#477 @olt)
- provider/postgis: Support for sub-queries as "tablename" (#467 @olt)
- provider/postgis: Set default_transaction_read_only when connecting to PostgreSQL (#369)
- cache/s3: added default mime-type of `application/vnd.mapbox-vector-tile` and made it configurable (#459 @stvno)
- mvt: don't require IDs for mvt features. (#337, #338 @ARolek)
- server: gzip encoding of tiles (#438 @ARolek)
- server: set proper MIME type for vector tiles (#511 @ARolek)
- server: configurable response headers (#519 @tierpod, @ARolek)
- server: Improve display of tile rendering times (#484 @ARolek)
- docker: Add CA certificates to Docker container. Refactored container and pipeline (#385 @gdey, @stvno, @ARolek)
- cmd: added support for `TEGOLA_PPROF_MUTEX_RATE` and `TEGOLA_PPROF_BLOCK_RATE` env vars when `pprof` is enabled. (@gdey)

**Bug Fixes**
- provider/gpkg: fixed index query for geomFieldname != geom (#486 @olt)
- provider/postgis: fixed error not thrown if the database user does not have permissions to access table (#538 @ARolek)
- cache seeding: invalid value for bounds () with 10e3 notation (#539 @gdey)
- fixed the way cache seed / purge with a tile-list or tile-name works. min and max zooms now must be provided for tegola to include parent and child tiles (@gdey)

**Breaking Changes**
- **IMPORTANT**: if you have a current tile cache in place, you will need to purge it entirely as tegola now expects the cache to persist gzipped tiles.
- `cors_allowed_origin` is no longer supported under the `webserver` config section. The same functionality can be implemented using the `[webserver.headers]` config which allows for configuring almost any response header. Use `Access-Control-Allow-Origin = "yourdomain.com"` in the config moving forward.
- Docker container now uses the `ENTRYPOINT` command. Users of the Docker container will need to update the commands they're passing to the container.

## 0.7.0 (2018-08-10)

- `Documentation`: Typo, grammar, clarity fixes. (#345 @erictheise)
- `inspector` : Sort property names in feature inspector (#367 @erictheise)
- `geom/encoding/wkb/` : Added Fuzzing framework for wkb (#53 @chebizarro)
- `server/`: Fix nil pointer dereference when using implicit zooms. (#387  @ear7h)
- `server/`: For MinZoom and MaxZoom default to appropriate values. (#354 @ear7h) 
- `server/`: For invalid x,y values we should return a non-200 response code. (#334 @ear7h)
- `server/`: Empty layer should return 404 (#375)
- `server/`: Bunch of new build tags for leaving out features. (#397)
- `server/` : Enviromental Var subtitution [breaking change] (#353 @ear7h)
- `server/`: Fixed TileJSON minZoom and maxZoom values when two layers with same zoom exists. (@paumas)
- `maths/makevalid`: Simplify, optimize unique. (#344 @paulmach)
- `cache/s3`: Add configurable ACL and Endpoint support for S3 cache to allow for S3 compliant stores outside of AWS (i.e. [minio](https://www.minio.io/features.html)). (#413 @stvno)
- `cache/s3`: Add configurable Cache-Control headers. (#448 @stvno)
- `cmd/tegola_lambda`: Support for running tegola on AWS Lambda. (#388) Instructions can be found in the README.md in the package.
- `cache/file`: On Windows files are written with invalid names (#422 @TNT0305)
- `providers/postgis`: Support for SSL (#426 @nickelbob)
- `providers/postgis`: Fixed panic when encountering 3D geometries (#89)
- `providers/postgis`: Gracefully handle NULL geometries (#429)
- `providers/postgis`: Add support for !bbox! (Mapnik) and !BOX! (MapServer) tokens (#443 @olt)
- `providers/postgis`: Add `geometry_type` option to avoid table inspection (#466 @olt)
- `providers/postgis`: Added `TEGOLA_` prefix to `SQL_DEBUG` (#489)
- `cache/azblob`: Support for Azure blob store as a cache backend (#425)
- Enable Go pprof profiler with `TEGOLA_HTTP_PPROF` environment (@olt)

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
- tegola now has a public docker image which can be found at https://hub.docker.com/r/gospatial/tegola/. 

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
