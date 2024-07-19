# Tegola

![On push](https://github.com/go-spatial/tegola/workflows/On%20push/badge.svg)
[![Report Card](https://goreportcard.com/badge/github.com/go-spatial/tegola)](https://goreportcard.com/badge/github.com/go-spatial/tegola)
[![Coverage Status](https://coveralls.io/repos/github/go-spatial/tegola/badge.svg?branch=master)](https://coveralls.io/github/go-spatial/tegola?branch=master)
[![Godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/go-spatial/tegola)
[![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](https://github.com/go-spatial/tegola/blob/master/LICENSE.md)

Tegola is a vector tile server delivering [Mapbox Vector Tiles](https://github.com/mapbox/vector-tile-spec) with support for [PostGIS](https://postgis.net/), [GeoPackage](https://www.geopackage.org/) and [SAP HANA Spatial](https://www.sap.com/products/technology-platform/hana/what-is-sap-hana.html) data providers. User documentation can be found at [tegola.io](https://tegola.io)

## Features

- Native geometry processing (simplification, clipping, make valid, intersection, contains, scaling, translation)
- [Mapbox Vector Tile v2 specification](https://github.com/mapbox/vector-tile-spec) compliant.
- An embedded viewer with an automatically generated style for quick data visualization and inspection.
- Support for [PostGIS](provider/postgis) and [GeoPackage](provider/gpkg) data providers. Extensible design to support additional data providers.
- Support for several cache backends: [file](cache/file), [s3](cache/s3), [redis](cache/redis), [azure blob store](cache/azblob).
- Cache seeding and invalidation via individual tiles (ZXY), lat / lon bounds and ZXY tile list.
- Parallelized tile serving and geometry processing.
- Support for Web Mercator (3857) and WGS84 (4326) projections.
- Support for [AWS Lambda](cmd/tegola_lambda).
- Support for serving HTTPS.
- Support for [PostGIS ST_AsMVT](mvtprovider/postgis).
- Support for [Prometheus](observability/prometheus/README.md) observability.

## Usage

```
tegola is a vector tile server
Version: v0.17.0

Usage:
  tegola [command]

Available Commands:
  cache       Manipulate the tile cache
  help        Help about any command
  serve       Use tegola as a tile server
  version     Print the version number of tegola

Flags:
      --config string   path to config file (default "config.toml")
  -h, --help            help for tegola

Use "tegola [command] --help" for more information about a command.
```

## Running tegola as a vector tile server

1. Download the appropriate binary of tegola for your platform via the [release page](https://github.com/go-spatial/tegola/releases).
2. Set up your config file and run. By default, Tegola looks for a `config.toml` in the same directory as the binary. You can set a different location for the `config.toml` using a command flag:

```
./tegola serve --config=/path/to/config.toml
```

## Server Endpoints

```
/
```

The server root will display the built-in viewer with an automatically generated style. For example:

![tegola built in viewer](https://raw.githubusercontent.com/go-spatial/tegola/v0.4.0/docs/screenshots/built-in-viewer.png "tegola built in viewer")

```
/maps/:map_name/:z/:x/:y
```

Return vector tiles for a map. The URI supports the following variables:

- `:map_name` is the name of the map as defined in the `config.toml` file.
- `:z` is the zoom level of the map.
- `:x` is the row of the tile at the zoom level.
- `:y` is the column of the tile at the zoom level.

```
/maps/:map_name/:layer_name/:z/:x/:y
```

Return vector tiles for a map layer. The URI supports the same variables as the map URI with the additional variable:

- `:layer_name` is the name of the map layer as defined in the `config.toml` file.

```
/capabilities
```

Return a JSON encoded list of the server's configured maps and layers with various attributes.

```
/capabilities/:map_name
```

Return [TileJSON](https://github.com/mapbox/tilejson-spec) details about the map.

```
/maps/:map_name/style.json
```

Return an auto generated [Mapbox GL Style](https://www.mapbox.com/mapbox-gl-js/style-spec/) for the configured map.

## Configuration

The tegola config file uses the [TOML](https://github.com/toml-lang/toml) format. The following example shows how to configure a `mvt_postgis` data provider. The `mvt_postgis` provider will leverage PostGIS's `ST_AsMVT()` function for the encoding of the vector tile.

Under the `maps` section, map layers are associated with data provider layers and their `min_zoom` and `max_zoom` values are defined.

### Example config using Postgres 12+ / PostGIS 3.0 ST_AsMVT():

```toml
# register a MVT data provider. MVT data providers have the prefix "mvt_" in their type
# note mvt data providers can not be conflated with any other providers of any type in a map
# thus a map may only contain a single mvt provider.
[[providers]]
name = "my_postgis"         # provider name is referenced from map layers (required).
type = "mvt_postgis"        # the type of data provider must be "mvt_postgis" for this data provider (required)
uri = "postgresql://tegola:<password>@localhost:5432/tegola?ssl_mode=prefer" # database connection string

  [[providers.layers]]
  name = "landuse"
  # MVT data provider must use SQL statements
  # this table uses "geom" for the geometry_fieldname and "gid" for the id_fieldname so they don't need to be configured
  # Wrapping the geom with ST_AsMVTGeom is required.
  sql = "SELECT ST_AsMVTGeom(geom,!BBOX!) AS geom, gid FROM gis.landuse WHERE geom && !BBOX!"
  # If you want to use the configurable parameters defined in maps.params make sure to include the token in the SQL statement
  sql = "SELECT ST_AsMVTGeom(geom,!BBOX!) AS geom, gid FROM gis.landuse WHERE geom && !BBOX! !PARAM!"

# maps are made up of layers
[[maps]]
name = "zoning"                           # used in the URL to reference this map (/maps/zoning)

  [[maps.layers]]
  name = "landuse"                        # name is optional. If it's not defined the name of the ProviderLayer will be used.
  provider_layer = "my_postgis.landuse"   # must match a data provider layer
  min_zoom = 10                           # minimum zoom level to include this layer
  max_zoom = 16                           # maximum zoom level to include this layer

  # configure addition URL parameters: /maps/:map_name/:layer_name/:z/:x/:y?param=value
  # which will be passed to the database queries
  [[maps.params]]
  name          = "param"         # name used in the URL
  token         = "!PARAM!"       # token to replace in providers.layers.sql query
  type          = "string"        # one of: int, float, string, bool
  sql           = "AND param = ?" # SQL to replace the token in the query. ? will be replaced with a parameter value. If omitted, defaults to "?"
  # if neither default_value nor default_sql is specified, the URL parameter is required to be present in all queries
  # either
  default_value = "value"         # if parameter is not specified, this value will be passed to .sql parameter
  # or
  default_sql   = " "             # if parameter is not specified, this value will replace the .sql parameter. Useful for omitting query entirely
```

- More information on PostgreSQL SSL modes can be found [here](https://www.postgresql.org/docs/current/libpq-ssl.html).
- More information on the `mvt_postgis` provider can be found [here](mvtprovider/postgis)

## Environment Variables

#### Config TOML

Environment variables can be injected into the configuration file. One caveat is that the injection has to be within a string, though the value it represents does not have to be a string.

The above config example could be written as:

```toml
# register data providers
[[providers]]
name = "test_postgis"
type = "mvt_postgis"
uri = "${POSTGIS_CONN_STR}"  # database connection string
srid = 3857
max_connections = "${POSTGIS_MAX_CONN}"
```

## SQL Debugging

The following environment variables can be used for debugging:

`TEGOLA_SQL_DEBUG` specify the type of SQL debug information to output. Currently, supporting two values:

- `LAYER_SQL` will print layer SQL as they are parsed from the config file.
- `EXECUTE_SQL` will print SQL that is executed for each tile request, and the number of items it returns or an error.

#### Usage

```bash
$ TEGOLA_SQL_DEBUG=LAYER_SQL tegola serve --config=/path/to/conf.toml
```

The following environment variables can be used to control various runtime options on dataproviders that are **NOT** `mvt_postgis`:

`TEGOLA_OPTIONS` specify a set of options comma or space delimited. Supports the following options

- `DontSimplifyGeo` to turn off simplification for all layers.
- `SimplifyMaxZoom={{int}}` to set the max zoom that simplification will apply to. (14 is default)

## Client side debugging

When debugging client side, it's often helpful to see an outline of a tile along with it's Z/X/Y values. To encode a debug layer into every tile add the query string variable `debug=true` to the URL template being used to request tiles. For example:

```
http://localhost:8080/maps/mymap/{z}/{x}/{y}.vector.pbf?debug=true
```

The requested tile will be encoded with a layer that has the `name` value set to `debug` and includes the three following features.

- `debug_outline` is a line feature that traces the border of the tile
- `debug_text` is a point feature in the middle of the tile with the following tags:
- `zxy` is a string with the `Z`, `X` and `Y` values formatted as: `Z:0, X:0, Y:0`

## Building from source

Tegola is written in [Go](https://golang.org/) and requires [Go 1.22](https://go.dev/dl/) or higher to compile from the source.
(We support the two newest versions of Go.)
To build tegola from the source, make sure you have Go installed and have cloned the repository.
Navigate to the repository then run the following command:

```bash
go generate ... && cd cmd/tegola/ && go build -mod vendor
```

You will now have a binary named `tegola` in the current directory which is [ready to run](#running-tegola-as-a-vector-tile-server).

**Build Flags**
The following build flags can be used to turn off certain features of tegola:

- `noAzblobCache` - turn off the Azure Blob cache back end.
- `noS3Cache` - turn off the AWS S3 cache back end.
- `noRedisCache` - turn off the Redis cache back end.
- `noPostgisProvider` - turn off the PostGIS data provider.
- `noGpkgProvider` - turn off the GeoPackage data provider. Note, GeoPackage uses CGO and will be turned off if the environment variable `CGO_ENABLED=0` is set prior to building.
- `noViewer` - turn off the built-in viewer.
- `pprof` - enable [Go profiler](https://golang.org/pkg/net/http/pprof/). Start profile server by setting the environment `TEGOLA_HTTP_PPROF_BIND` environment (e.g. `TEGOLA_HTTP_PPROF_BIND=localhost:6060`).
- `noPrometheusObserver` - turn off support for the Prometheus metric end point.

Example of using the build flags to turn of the Redis cache back end, the GeoPackage provider and the built-in viewer.

```bash
go build -tags 'noRedisCache noGpkgProvider noViewer'
```

**Setting Version Information** The following flags can be used to set version information:

```bash
# first set some env to make it easier to read:
BUILD_PKG=github.com/go-spatial/tegola/internal/build
VERSION=1.16.x
GIT_BRANCH=$(git branch --no-color --show-current)
GIT_REVISION=$(git log HEAD --oneline | head -n 1 | cut -d ' ' -f 1)

# build the go binary
go build -ldflags "-w -X ${BUILD_PKG}.Version=${VERSION} -X ${BUILD_PKG}.GitRevision=${GIT_REVISION} -X ${BUILD_PKG}.GitBranch=${GIT_BRANCH}"
```

## License

See [license](LICENSE.md) file in the repo.

## Looking for a vector tile style editor?

After Tegola is running you're likely going to want to work on your map's cartography.
Give [fresco](https://github.com/go-spatial/fresco) a try!
