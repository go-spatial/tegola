package hana

import (
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
)

func init() {
	_ = provider.Register(provider.TypeStd.Prefix()+Name, NewTileProvider, Cleanup)
	_ = provider.MVTRegister(provider.TypeMvt.Prefix()+Name, NewMVTTileProvider, Cleanup)
}

const (
	MVTProviderType = "mvt_hana"
	ProviderType    = "hana"
)

// NewTileProvider instantiates and returns a new HANA provider or an error.
// The function will validate that the config object looks good before
// trying to create a driver. This Provider supports the following fields
// in the provided map[string]interface{} map:
//
//	uri (string): [Required] HANA connection string
//	name (string): [Required] Provider name is referenced from map layers
//	srid (int): [Optional] The default SRID for the provider. When omitted the SRID is auto-detected from the geometry column. Any numeric SRID known to the database is supported, as well as a synthetic SRID registered via crs_defn.
//	crs_defn (string): [Optional] A PROJ.4 definition of the layer's coordinate reference system, registered internally under a synthetic SRID. Wins over srid on the same level. Requires a raw geometry_format (wkb/wkt/mos) and is not supported for MVT providers.
//	type (string): [Required] The type of data provider. must be "hana" to use this data provider
//	layers (map[string]struct{})  — This is map of layers keyed by the layer name. Supports the following properties
//
//		name (string): [Required] the name of the layer. This is used to reference this layer from map layers.
//		tablename (string): [*Required] the name of the database table to query against. Required if sql is not defined.
//		geometry_fieldname (string): [Optional] the name of the filed which contains the geometry for the feature. defaults to geom
//		id_fieldname (string): [Optional] the name of the feature id field. defaults to empty (no id attribute unless configured)
//		fields ([]string): [Optional] a list of fields to include alongside the feature. Can be used if sql is not defined.
//		srid (int): [Optional] the SRID of the layer. Any numeric SRID known to the database is supported, as well as a synthetic SRID registered via crs_defn.
//		crs_defn (string): [Optional] A PROJ.4 definition of the layer's coordinate reference system, registered internally under a synthetic SRID. Wins over srid on the same level. Requires a raw geometry_format (wkb/wkt/mos) and is not supported for MVT providers.
//		sql (string): [*Required] custom SQL to use use. Required if tablename is not defined. Supports the following tokens:
//
//			!BBOX! - [Required] will be replaced with the bounding box of the tile before the query is sent to the database.
//			!ZOOM! - [Optional] will be replaced with the "Z" (zoom) value of the requested tile.
func NewTileProvider(config dict.Dicter, maps []provider.Map) (provider.Tiler, error) {
	return CreateProvider(config, maps, ProviderType)
}
func NewMVTTileProvider(config dict.Dicter, maps []provider.Map) (provider.MVTTiler, error) {
	return CreateProvider(config, maps, MVTProviderType)
}
