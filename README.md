# Tegola

[![Build Status](https://travis-ci.org/terranodo/tegola.svg?branch=master)](https://travis-ci.org/terranodo/tegola)
[![Report Card](https://goreportcard.com/badge/github.com/terranodo/tegola)](https://goreportcard.com/badge/github.com/terranodo/tegola)
[![Godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/terranodo/tegola)
[![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](https://github.com/terranodo/tegola/blob/master/LICENSE.md)

Tegola is a high performance vector tile server delivering [Mapbox Vector Tiles](https://github.com/mapbox/vector-tile-spec) leveraging PostGIS as the data provider.

## Near term goals
- [X] Support for transcoding WKB to MVT.
- [x] Support for `/z/x/y` web mapping URL scheme.
- [x] Support for PostGIS data provider.

## Running Tegola
1. Download the appropriate binary of tegola for your platoform via the [release page](https://github.com/terranodo/tegola/releases).
2. Setup your config file and run. Tegola expects a `config.toml` to be in the same directory as the binary. You can set a different location for the `config.toml` using a command flag:

```
./tegola -config=/path/to/config.toml
```

## URL Scheme
Tegola uses the following URL scheme:

```
/maps/:map_name/:z/:x/:y
```

- `:map_name` is the name of the map as defined in the `config.toml` file.
- `:z` is the zoom level of the map.
- `:x` is the row of the tile at the zoom level.
- `:y` is the column of the tile at the zoom level.

### Additional endpoints

```
/capabilities
```
Will return a JSON encoded list of the server's configured maps and layers with various attributes. An example response:

```json
{
	"maps": [{
		"name": "zoning",
		"uri": "/maps/zoning",
		"layers": [{
			"name": "landuse",
			"minZoom": 12,
			"maxZoom": 16
		}]
	}]
}

```

## Configuration
The tegola config file uses the [TOML](https://github.com/toml-lang/toml) format. The following example shows how to configure a PostGIS data provider with two layers. The first layer includes a `tablename`, `geometry_field` and an `id_field`. The second layer uses a custom `sql` statement instead of the `tablename` property.

Under the `maps` section, map layers are associated with dataprovider layers and their `min_zoom` and `max_zoom` values are defined. Optionally, `custom_tags` can be setup which will be encoded into the layer. If the same tags are returned from a data provider, the dataprovider's values will take precidence.

```toml

[webserver]
port = ":9090"

# register data providers
[[providers]]
name = "test_postgis"	# provider name is referenced from map layers
type = "postgis"		# the type of data provider. currently only supports postgis
host = "localhost"		# postgis database host
port = 5432				# postgis database port
database = "tegola" 	# postgis database name
user = "tegola"			# postgis database user
password = ""			# postgis database password
srid = 3857             # The default srid for this provider. If not provided it will be WebMercator (3857)

	[[providers.layers]]
	name = "landuse" 					# will be encoded as the layer name in the tile
	tablename = "gis.zoning_base_3857" 	# sql or table_name are required
	geometry_fieldname = "geom"			# geom field. default is geom
	id_fieldname = "gid"				# geom id field. default is gid
	srid = 4326                         # the srid of table's geo data.


	[[providers.layers]]
	name = "roads" 					# will be encoded as the layer name in the tile
	tablename = "gis.zoning_base_3857" 	# sql or table_name are required
	geometry_fieldname = "geom"			# geom field. default is geom
	id_fieldname = "gid"				# geom id field. default is gid
	fields = [ "class", "name" ]        # Additional fields to include in the select statement.
	srid = 3857                         # the srid of table's geo data. Don't need to specify this as it will inherit this from the provider.

	[[providers.layers]]
	name = "rivers" 					# will be encoded as the layer name in the tile
	geometry_fieldname = "geom"			# geom field. default is geom
	id_fieldname = "gid"				# geom id field. default is gid
	# Custom sql to be used for this layer. Note: that the geometery field is wraped
	# in a ST_AsBinary, as tegola only understand wkb.
	sql = """
		SELECT
			gid,
			ST_AsBinary(geom) AS geom
		FROM
			gis.rivers
		WHERE
			geom && !BBOX!
	"""

# maps are made up of layers
[[maps]]
name = "zoning"							# used in the URL to reference this map (/maps/:map_name)

	[[maps.layers]]
	provider_layer = "test_postgis.landuse"	# must match a data provider layer
	min_zoom = 12						# minimum zoom level to include this layer
	max_zoom = 16						# maximum zoom level to include this layer

		[maps.layers.default_tags]		# table of default tags to encode in the tile. SQL statements will override
		class = "park"

	[[maps.layers]]
	provider_layer = "test_postgis.rivers"	# must match a data provider layer
	min_zoom = 10						# minimum zoom level to include this layer
	max_zoom = 18						# maximum zoom level to include this layer


```

## Command flags
Tegola currently supports the following command flags:

- `config` - Location of config file in TOML format. Can be absolute, relative or remote over http(s).
- `port` - Port for the webserver to bind to. i.e. :8080
- `log-file` - Path to write webserver access logs
- `log-format` - The format that the logger will log with. Available fields:
  - `{{.Time}}` : The current Date Time in RFC 2822 format.
  - `{{.RequestIP}}` : The IP address of the the requester.
  - `{{.Z}}` : The Zoom level.
  - `{{.X}}` : The X Coordinate.
  - `{{.Y}}` : The Y Coordinate.

## Debugging

### Environment Variables
The following environment variables can be used for debugging the tegola server:

`SQL_DEBUG` specify the type of SQL debug information to output. Currently support two values:

- `LAYER_SQL`: print layer SQL as they are parsed from the config file.
- `EXECUTE_SQL`: print SQL that is executed for each tile request and the number of items it returns or an error.


`TEGOLA_CLIPPING='mvt'` to enable experimental clipping support. This will clip the geometries to the boundaries of the tile.

#### Usage

```bash
$ SQL_DEBUG=LAYER_SQL tegola -config=/path/to/conf.toml
```

### Client side
When debugging client side, it's often helpful to to see an outline of a tile along with it's Z/X/Y values. To encode a debug layer into every tile add the query string variable `debug=true` to the URL template being used to request tiles. For example:

```
http://localhost:8080/maps/mymap/{z}/{x}/{y}.vector.pbf?debug=true
```

The requested tile will be encode a layer with the `name` value set to `debug` and include two features:

 - `debug_outline`: a line feature that traces the border of the tile
 - `debug_text`: a point feature in the middle of the tile with the following tags:
   - `zxy`: a string with the `Z`, `X` and `Y` values formatted as: `Z:0, X:0, Y:0`

## Building from source

Tegola is written in [Go](https://golang.org/) and requires Go 1.8+ to compile from source. To build tegola from source, make sure you have Go installed and have cloned the repository to your `$GOPATH` or `$GOROOT`. Navigate to the repository then run the following commands:

```bash
cd cmd/tegola/
go build -o tegola *.go
```

You will now have a binary named `tegola` in the current directory which is [ready for running](#running-tegola).


## Specifications
- [Well Known Binary (WKB)](http://edndoc.esri.com/arcsde/9.1/general_topics/wkb_representation.htm)
- [Mapbox Vector Tile (MVT) 2.1](https://github.com/mapbox/vector-tile-spec/tree/master/2.1)

## License
See [license](LICENSE.md) file in repo.
