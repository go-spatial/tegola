package cache

import (
	"math"

	"github.com/go-spatial/geom/slippy"
)

// rangeFamilyAt runs the given callback on every related tile at the given zoom. This could include
// the provided tile itself (if the same zoom is provided), the parent (overlapping tile at a lower zoom
// level), or children (overlapping tiles at a higher zoom level).
//
// Copied from go-spatial/geom (dc1d50720ee77122d0) since the function by this name doesn't do
// the same thing anymore. (In geom, it no longer returns ancestors or self, only descendants.
// It's also buggy because grid.ToNative/FromNative are buggy.)
//
// This function should be removed once the one in geom is updated to work as expected.
func rangeFamilyAt(t *slippy.Tile, zoom uint, f func(*slippy.Tile) error) error {
	// handle ancestors and self
	if zoom <= t.Z {
		mag := t.Z - zoom
		arg := slippy.NewTile(zoom, t.X>>mag, t.Y>>mag)
		return f(arg)
	}

	// handle descendants
	mag := zoom - t.Z
	delta := uint(math.Exp2(float64(mag)))

	leastX := t.X << mag
	leastY := t.Y << mag

	for x := leastX; x < leastX+delta; x++ {
		for y := leastY; y < leastY+delta; y++ {
			err := f(slippy.NewTile(zoom, x, y))
			if err != nil {
				return err
			}
		}
	}

	return nil
}
