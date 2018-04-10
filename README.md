# Tegola

[![Build Status](https://travis-ci.org/go-spatial/tegola.svg?branch=master)](https://travis-ci.org/go-spatial/tegola)
[![Report Card](https://goreportcard.com/badge/github.com/go-spatial/tegola)](https://goreportcard.com/badge/github.com/go-spatial/tegola)
[![Coverage Status](https://coveralls.io/repos/github/go-spatial/tegola/badge.svg?branch=master)](https://coveralls.io/github/go-spatial/tegola?branch=master)
[![Godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/go-spatial/tegola)
[![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](https://github.com/go-spatial/tegola/blob/master/LICENSE.md)

Tegola is a vector tile server delivering [Mapbox Vector Tiles](https://github.com/mapbox/vector-tile-spec) leveraging PostGIS as the data provider.

## Features
- Native geometry processing (simplification, clipping, make valid, intersection, contains, scaling, translation)
- [Mapbox Vector Tile v2 specification](https://github.com/mapbox/vector-tile-spec) compliant.
- Embedded viewer with auto generated style for quick data visualization and inspection.
- Support for PostGIS as a data provider. Extensible to support additional data providers.
- Local filesystem caching. Extensible design to support additional cache backends.
- Cache seeding to fill the cache prior to web requests.
- Parallelized tile serving and geometry processing.
- Support for Web Mercator (3857) and WGS84 (4326) projections.

## Usage
```
tegola is a vector tile server
Version: v0.6.0 

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
2. After the download you will need to make the binary executable. The binary, will be name `tegola_$OS_$ARCH`, to follow along with the instructions make sure to rename it to `tegola`.
2. Setup your config file and run. Tegola expects a `config.toml` to be in the same directory as the binary. You can set a different location for the `config.toml` using a command flag:

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
/capabilities/:map_name/style.json
```

Return an auto generated [Mapbox GL Style](https://www.mapbox.com/mapbox-gl-js/style-spec/) for the configured map.

## Configuration
The tegola config file uses the [TOML](https://github.com/toml-lang/toml) format. The following example shows how to configure a PostGIS data provider with two layers. The first layer includes a `tablename`, `geometry_field` and an `id_field`. The second layer uses a custom `sql` statement instead of the `tablename` property.

Under the `maps` section, map layers are associated with data provider layers and their `min_zoom` and `max_zoom` values are defined. Optionally, `default_tags` can be setup which will be encoded into the layer. If the same tags are returned from a data provider, the data provider's values will take precedence.

```toml
[webserver]
port = ":9090"              # port to bind the web server to. defaults ":8080"

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

	[[providers.layers]]
	name = "landuse"                    # will be encoded as the layer name in the tile
	tablename = "gis.zoning_base_3857"  # sql or table_name are required
	geometry_fieldname = "geom"         # geom field. default is geom
	id_fieldname = "gid"                # geom id field. default is gid
	srid = 4326                         # the srid of table's geo data. Defaults to WebMercator (3857)

	[[providers.layers]]
	name = "roads"                      # will be encoded as the layer name in the tile
	tablename = "gis.zoning_base_3857"  # sql or table_name are required
	geometry_fieldname = "geom"         # geom field. default is geom
	id_fieldname = "gid"                # geom id field. default is gid
	fields = [ "class", "name" ]        # Additional fields to include in the select statement.

	[[providers.layers]]
	name = "rivers"                     # will be encoded as the layer name in the tile
	geometry_fieldname = "geom"         # geom field. default is geom
	id_fieldname = "gid"                # geom id field. default is gid
	# Custom sql to be used for this layer. Note: that the geometery field is wraped
	# in a ST_AsBinary() and the use of the !BBOX! token
	sql = "SELECT gid, ST_AsBinary(geom) AS geom FROM gis.rivers WHERE geom && !BBOX!"

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
	min_zoom = 10                            # minimum zoom level to include this layer
	max_zoom = 18                            # maximum zoom level to include this layer
```

### Supported PostGIS SQL tokens
The following tokens are supported in custom SQL queries for the PostGIS data provider:

- `!BBOX!` - [required] Will convert the z/x/y values into a bounding box to query the feature table with.
- `!ZOOM!` - [optional] Pass in the zoom value for the request. Useful for filtering feature results by zoom.

## Environment Variables
The following environment variables can be used for debugging:

`SQL_DEBUG` specify the type of SQL debug information to output. Currently support two values:

- `LAYER_SQL`: print layer SQL as they are parsed from the config file.
- `EXECUTE_SQL`: print SQL that is executed for each tile request and the number of items it returns or an error.

#### Usage

```bash
$ SQL_DEBUG=LAYER_SQL tegola -config=/path/to/conf.toml
```

The following environment variables can be use to control various runtime options:

`TEGOLA_OPTIONS` specify a set of options comma (or space) seperated options.

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

Tegola is written in [Go](https://golang.org/) and requires Go 1.8+ to compile from source. To build tegola from source, make sure you have Go installed and have cloned the repository to your `$GOPATH`. Navigate to the repository then run the following commands:

```bash
cd cmd/tegola/
go build
```

You will now have a binary named `tegola` in the current directory which is [ready for running](#running-tegola).

**Build Flags**
The following build flags can be used to turn off certain features of tegola:

- `noS3Cache` - turn off the AWS S3 cache back end
- `noRedisCache` - turn off the Redis cache back end
- `noPostgisProvider` - turn off the PostGIS data provider
- `noGpkgProvider` - turn off the GeoPackage data provider. Note, GeoPackage uses CGO and will be turned off if the environment variable `CGO_ENABLED=0` is set prior to building.

Example of using the build flags to turn of the Redis cache back end and the GeoPackage provider

```bash
go build -tags 'noRedisCache noGpkgProvider'
```


**Turning off CGO**
Tegola uses CGO for certain functionality (i.e. GeoPackge support). To build tegola without CGO use the following command:

```bash
CGO_ENABLED go build
```


## License
See [license](LICENSE.md) file in repo.
