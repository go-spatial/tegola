package slippy

// MvtTileDim is the number of pixels in a tile
const MvtTileDim = 4096.0

// PixelsToProjectedUnits scalar conversion of pixels into projected units
// TODO (@ear7h): this only considers the tile's native width
func PixelsToNative(g Grid, zoom uint, pixels uint) float64 {
	ext, _ := Extent(g, NewTile(zoom, 0, 0))
	return ext.XSpan() / MvtTileDim
}
