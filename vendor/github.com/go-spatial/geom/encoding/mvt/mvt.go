/*
Package mvt is used to encode MVT tiles

In short, a `Tile`s has `Layer`s, which have `Feauture`s. The `Feature` type
is what holds a single `geom.Geometry` and associated metadata.

To encode a geometry into a tile, you need:
	* a geometry
	* a tile's `geom.Extent` in the same projection as the geometry
	* the size of the tile you want to output in pixels

note: the geometry must not go outside the tile extent. If this is unknown,
use the clip package before encoding.
(https://godoc.org/github.com/go-spatial/geom/planar/clip#Geometry)


To encode:
	1. Call `PrepareGeomtry`, it returns a `geom.Geometry` that is "reprojected"
	   into pixel values relative to the tile
	2. Add the returned geometry to a `Feature`, optionally with an ID and
	   tags by calling `NewFeatures`.
	3. Add the feature to a `Layer` with a name for the layer
	   by calling `(*Layer).AddFeatures`
	4. Add the layer to a `Tile` by calling `(*Tile).AddLayers`
	5. Get the `protobuf` tile by calling `(*Tile).VTile`
	6. Encode the `protobuf` into bytes with `proto.Marshal`

For an example, check the use of this package in tegola/atlas/map.go (https://github.com/go-spatial/tegola/blob/master/atlas/map.go)
*/
package mvt

const (
	MimeType      = "application/vnd.mapbox-vector-tile"
)

var (
	Version       uint32 = 2
	DefaultExtent uint32 = 4096
)
