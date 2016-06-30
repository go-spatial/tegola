package server

/*
providers:										# Map of providers, the key is the provider name.
    provider1:								# Provider name
         connection_string: "localhost:8080…"	# This is the connection string
         type: "postgis"						# The provider type

maps:											# Map of map ids, the key is the map id name.
     topo:									# Map name
		"layer1":	[0,18]						# The value is a map of layers, where the key is the layer name, and value is left open for the zoom level. (TODO)

layers:											# Map of the layers in the system, the key is the layer name.
	layer1:									# Layer name
		data_provider: "provider1"				# Which data_provider the layers uses.
		config: "…"								# The config for the layer.
*/

type Config struct {
	Providers map[string]Provider
	Maps      map[string]Map
	Layers    map[string]Layer
}

type Provider struct {
	Type string
	//	TODO: this is specific to Postgres. Need to consider polymorphic association
	ConnStr string
}

type Map map[string]MapLayer

type MapLayer struct {
	Zoom []int
}

type Layer struct {
	Provider string
	Config   string
}

var Conf Config
