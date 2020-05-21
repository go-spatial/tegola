# Tegola

![On push](https://github.com/go-spatial/tegola/workflows/On%20push/badge.svg)
[![Report Card](https://goreportcard.com/badge/github.com/go-spatial/tegola)](https://goreportcard.com/badge/github.com/go-spatial/tegola)
[![Coverage Status](https://coveralls.io/repos/github/go-spatial/tegola/badge.svg?branch=master)](https://coveralls.io/github/go-spatial/tegola?branch=master)
[![Godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/go-spatial/tegola)
[![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](https://github.com/go-spatial/tegola/blob/master/LICENSE.md)

Tegola is a vector tile server delivering [Mapbox Vector Tiles](https://github.com/mapbox/vector-tile-spec) with support for [PostGIS](https://postgis.net/) and [GeoPackage](https://www.geopackage.org/) data providers. User documentation can be found at [tegola.io](https://tegola.io)

## Features
- Native geometry processing (simplification, clipping, make valid, intersection, contains, scaling, translation)
- [Mapbox Vector Tile v2 specification](https://github.com/mapbox/vector-tile-spec) compliant.
- Embedded viewer with auto generated style for quick data visualization and inspection.
- Support for PostGIS and GeoPackage data providers. Extensible design to support additional data providers.
- Support for several cache backends: [file](cache/file), [s3](cache/s3), [redis](cache/redis), [azure blob store](cache/azblob).
- Cache seeding and invalidation via individual tiles (ZXY), lat / lon bounds and ZXY tile list.
- Parallelized tile serving and geometry processing.
- Support for Web Mercator (3857) and WGS84 (4326) projections.
- Support for [AWS Lambda](cmd/tegola_lambda).
- Support for serving HTTPS.
- Support for [PostGIS ST_AsMVT](mvtprovider/postgis).

## Usage
```
tegola is a vector tile server
Version: v0.12.0

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
2. Setup your config file and run. Dy default tegola looks for a `config.toml` in the same directory as the binary. You can set a different location for the `config.toml` using a command flag:

```
./tegola serve --config=/path/to/config.toml
```

## Server Endpoints

```
/
```

The server root will display a built in viewer with an auto generated style. For example:

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
The tegola config file uses the [TOML](https://github.com/toml-lang/toml) format. The following example shows how to configure a PostGIS data provider with two layers. The first layer includes a `tablename`, `geometry_field` and an `id_field`. The second layer uses a custom `sql` statement instead of the `tablename` property.

Under the `maps` section, map layers are associated with data provider layers and their `min_zoom` and `max_zoom` values are defined. Optionally, `default_tags` can be setup which will be encoded into the layer. If the same tags are returned from a data provider, the data provider's values will take precedence.

```toml
[webserver]
port = ":9090"              # port to bind the web server to. defaults ":8080"
ssl_cert = "fullchain.pem"  # ssl cert for serving by https
ssl_key = "privkey.pem"     # ssl key for serving by https

	[webserver.headers]
	Access-Control-Allow-Origin = "*"
	Cache-Control = "no-cache, no-store, must-revalidate"

[cache]                     # configure a tile cache
type = "file"               # a file cache will cache to the local file system
basepath = "/tmp/tegola"    # where to write the file cache

# register data providers
[[providers]]
name = "test_postgis"       # provider name is referenced from map layers (required)
type = "postgis"            # the type of data provider. currently only supports postgis (required)
host = "localhost"          # postgis database host (required)
port = 5432                 # postgis database port (required)
database = "tegola"         # postgis database name (required)
user = "tegola"             # postgis database user (required)
password = ""               # postgis database password (required)
srid = 3857                 # The default srid for this provider. Defaults to WebMercator (3857) (optional)
max_connections = 50        # The max connections to maintain in the connection pool. Default is 100. (optional)
ssl_mode = "prefer"        # PostgreSQL SSL mode*. Default is "disable". (optional)

	[[providers.layers]]
	name = "landuse"                    # will be encoded as the layer name in the tile
	tablename = "gis.zoning_base_3857"  # sql or tablename are required
	geometry_fieldname = "geom"         # geom field. default is geom
	id_fieldname = "gid"                # geom id field. default is gid
	srid = 4326                         # the srid of table's geo data. Defaults to WebMercator (3857)

	[[providers.layers]]
	name = "roads"                      # will be encoded as the layer name in the tile
	tablename = "gis.zoning_base_3857"  # sql or tablename are required
	geometry_fieldname = "geom"         # geom field. default is geom
	geometry_type = "linestring"        # geometry type. if not set, tables are inspected at startup to try and infer the gemetry type
	id_fieldname = "gid"                # geom id field. default is gid
	fields = [ "class", "name" ]        # Additional fields to include in the select statement.

	[[providers.layers]]
	name = "rivers"                     # will be encoded as the layer name in the tile
	geometry_fieldname = "geom"         # geom field. default is geom
	id_fieldname = "gid"                # geom id field. default is gid
	# Custom sql to be used for this layer. Note: that the geometery field is wraped
	# in a ST_AsBinary() and the use of the !BBOX! token
	sql = "SELECT gid, ST_AsBinary(geom) AS geom FROM gis.rivers WHERE geom && !BBOX!"

	[[providers.layers]]
	name = "buildings"                  # will be encoded as the layer name in the tile
	geometry_fieldname = "geom"         # geom field. default is geom
	id_fieldname = "gid"                # geom id field. default is gid
	# Custom sql to be used for this layer as a sub query. ST_AsBinary and
	# !BBOX! filter are applied automatically.
	sql = "(SELECT gid, geom, type FROM buildings WHERE scalerank = !ZOOM! LIMIT 1000) AS sub"

# register mvt data providers
# note mvt data providers can not be conflated with any other providers of any type in a map.
# thus a map may only contain a single mvt_provider.
[[mvt_providers]]
name = "test_postgis"       # provider name is referenced from map layers (required)
type = "postgis"            # the type of data provider must be "postgis" for this data provider (required)
host = "localhost"          # PostGIS database host (required)
port = 5432                 # PostGIS database port (required)
database = "tegola"         # PostGIS database name (required)
user = "tegola"             # PostGIS database user (required)
password = ""               # PostGIS database password (required

[[mvt_providers.layers]]
name = "landuse"
# MVT data provider must use SQL statements
# this table uses "geom" for the geometry_fieldname and "gid" for the id_fieldname so they don't need to be configured
sql = "SELECT ST_AsMVTGeom(geom,!BBOX!) AS geom, gid FROM gis.landuse WHERE geom && !BBOX!"

# maps are made up of layers
[[maps]]
name = "zoning"                              # used in the URL to reference this map (/maps/:map_name)

	[[maps.layers]]
	name = "landuse"                         # name is optional. If it's not defined the name of the ProviderLayer will be used.
	                                         # It can also be used to group multiple ProviderLayers under the same namespace.
	provider_layer = "test_postgis.landuse"  # must match a data provider layer
	min_zoom = 12                            # minimum zoom level to include this layer
	max_zoom = 16                            # maximum zoom level to include this layer

		[maps.layers.default_tags]           # table of default tags to encode in the tile. SQL statements will override
		class = "park"

	[[maps.layers]]
	name = "rivers"                          # name is optional. If it's not defined the name of the ProviderLayer will be used.
	                                         # It can also be used to group multiple ProviderLayers under the same namespace.
	provider_layer = "test_postgis.rivers"   # must match a data provider layer
	dont_simplify = true                     # optionally, turn off simplification for this layer. Default is false.
	dont_clip = true                         # optionally, turn off clipping for this layer. Default is false.
	min_zoom = 10                            # minimum zoom level to include this layer
	max_zoom = 18                            # maximum zoom level to include this layer


# note that this map is only using mvt_test_postgis provider. 
# it can not conflate any other providers
[[maps]]
name = "landuse_mvt"

	 [[maps.layers]]
	 name = "landuse"
	 provider_layer = "mvt_test_postgis.landuse" # note the mvt data provider name is prefixed with `mvt_`
	 min_zoom = 12                            # minimum zoom level to include this layer
	 max_zoom = 16                            # maximum zoom level to include this layer


```

\* more on PostgreSQL SSL mode [here](https://www.postgresql.org/docs/9.2/static/libpq-ssl.html). The `postgis` config also supports "ssl_cert" and "ssl_key" options are required, corresponding semantically with "PGSSLKEY" and "PGSSLCERT". These options do not check for environment variables automatically. See the section [below](#environment-variables) on injecting environment variables into the config.

## Environment Variables

#### Config TOML
Environment variables can be injected into the configuration file. One caveat is that the injection has to be within a string, though the value it represents does not have to be a string.

The above config example could be written as:
```toml
# register data providers
[[providers]]
name = "test_postgis"
type = "postgis"
host = "${POSTGIS_HOST}"    # postgis database host (required)
port = "${POSTGIS_PORT}"    # recall this value must be an int
database = "${POSTGIS_DB}"
user = "tegola"
password = ""
srid = 3857
max_connections = "${POSTGIS_MAX_CONN}"
```

#### SQL Debugging
The following environment variables can be used for debugging:

`TEGOLA_SQL_DEBUG` specify the type of SQL debug information to output. Currently support two values:

- `LAYER_SQL`: print layer SQL as they are parsed from the config file.
- `EXECUTE_SQL`: print SQL that is executed for each tile request and the number of items it returns or an error.

#### Usage

```bash
$ TEGOLA_SQL_DEBUG=LAYER_SQL tegola serve --config=/path/to/conf.toml
```

The following environment variables can be used to control various runtime options:

`TEGOLA_OPTIONS` specify a set of options comma or space delimited. Supports the following options

- `DontSimplifyGeo` to turn off simplification for all layers.
- `SimplifyMaxZoom={{int}}` to set the max zoom that simplification will apply to. (14 is default)


## Client side debugging

When debugging client side, it's often helpful to to see an outline of a tile along with it's Z/X/Y values. To encode a debug layer into every tile add the query string variable `debug=true` to the URL template being used to request tiles. For example:

```
http://localhost:8080/maps/mymap/{z}/{x}/{y}.vector.pbf?debug=true
```

The requested tile will be encode a layer with the `name` value set to `debug` and include two features:

 - `debug_outline`: a line feature that traces the border of the tile
 - `debug_text`: a point feature in the middle of the tile with the following tags:
   - `zxy`: a string with the `Z`, `X` and `Y` values formatted as: `Z:0, X:0, Y:0`

## Building from source

Tegola is written in [Go](https://golang.org/) and requires Go 1.x to compile from source. (We support the three newest versions of Go.) To build tegola from source, make sure you have Go installed and have cloned the repository to your `$GOPATH`. Navigate to the repository then run the following commands:


```bash
cd cmd/tegola/
go build
```

You will now have a binary named `tegola` in the current directory which is [ready for running](#running-tegola).

**Build Flags**
The following build flags can be used to turn off certain features of tegola:

- `noAzblobCache` - turn off the Azure Blob cache back end.
- `noS3Cache` - turn off the AWS S3 cache back end.
- `noRedisCache` - turn off the Redis cache back end.
- `noPostgisProvider` - turn off the PostGIS data provider.
- `noGpkgProvider` - turn off the GeoPackage data provider. Note, GeoPackage uses CGO and will be turned off if the environment variable `CGO_ENABLED=0` is set prior to building.
- `noViewer` - turn off the built in viewer.
- `pprof` - enable [Go profiler](https://golang.org/pkg/net/http/pprof/). Start profile server by setting the environment `TEGOLA_HTTP_PPROF_BIND` environment (e.g. `TEGOLA_HTTP_PPROF_BIND=localhost:6060`).

Example of using the build flags to turn of the Redis cache back end, the GeoPackage provider and the built in viewer.

```bash
go build -tags 'noRedisCache noGpkgProvider noViewer'
```

## License
See [license](LICENSE.md) file in repo.

## Looking for a vector tile style editor?
Once you have tegola running you're likely going to want to work on your map's cartography. Give [fresco](https://github.com/go-spatial/fresco) a try!