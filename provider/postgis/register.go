package postgis

import (
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/mvtprovider"
	"github.com/go-spatial/tegola/provider"
)

func init() {
	provider.Register(Name, NewTileProvider, Cleanup)
	mvtprovider.Register(Name, NewMVTTileProvider, Cleanup)
}

// NewTileProvider instantiates and returns a new postgis provider or an error.
// The function will validate that the config object looks good before
// trying to create a driver. This Provider supports the following fields
// in the provided map[string]interface{} map:
//
// 	host (string): [Required] postgis database host
// 	port (int): [Required] postgis database port (required)
// 	database (string): [Required] postgis database name
// 	user (string): [Required] postgis database user
// 	password (string): [Required] postgis database password
// 	srid (int): [Optional] The default SRID for the provider. Defaults to WebMercator (3857) but also supports WGS84 (4326)
// 	max_connections : [Optional] The max connections to maintain in the connection pool. Default is 100. 0 means no max.
// 	layers (map[string]struct{})  — This is map of layers keyed by the layer name. supports the following properties
//
// 		name (string): [Required] the name of the layer. This is used to reference this layer from map layers.
// 		tablename (string): [*Required] the name of the database table to query against. Required if sql is not defined.
// 		geometry_fieldname (string): [Optional] the name of the filed which contains the geometry for the feature. defaults to geom
// 		id_fieldname (string): [Optional] the name of the feature id field. defaults to gid
// 		fields ([]string): [Optional] a list of fields to include alongside the feature. Can be used if sql is not defined.
// 		srid (int): [Optional] the SRID of the layer. Supports 3857 (WebMercator) or 4326 (WGS84).
// 		sql (string): [*Required] custom SQL to use use. Required if tablename is not defined. Supports the following tokens:
//
// 			!BBOX! - [Required] will be replaced with the bounding box of the tile before the query is sent to the database.
// 			!ZOOM! - [Optional] will be replaced with the "Z" (zoom) value of the requested tile.
//
func NewTileProvider(config dict.Dicter) (provider.Tiler, error)       { return CreateProvider(config) }
func NewMVTTileProvider(config dict.Dicter) (mvtprovider.Tiler, error) { return CreateProvider(config) }
