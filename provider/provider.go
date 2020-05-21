package provider

import (
	"context"
	"fmt"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/log"
)

// TODO(@ear7h) remove this atrocity from the code base
// tile_t is an implementation of the Tile interface, it is
// named as such as to not confuse from the 4 other possible meanings
// of the symbol "tile" in this code base. It should be removed after
// the geom port is mostly done as part of issue #499 (removing the
// Tile interface in this package)
type tile_t struct {
	slippy.Tile
	buffer uint
}

func NewTile(z, x, y, buf, srid uint) Tile {
	return &tile_t{
		Tile: slippy.Tile{
			Z: z,
			X: x,
			Y: y,
		},
		buffer: buf,
	}
}

func (tile *tile_t) Extent() (ext *geom.Extent, srid uint64) {
	return tile.Extent3857(), 3857
}

func (tile *tile_t) BufferedExtent() (ext *geom.Extent, srid uint64) {
	return tile.Extent3857().ExpandBy(slippy.Pixels2Webs(tile.Z, tile.buffer)), 3857
}

// Tile is an interface used by Tiler, it is an unecessary abstraction and is
// due to be removed. The tiler interface will, instead take a, *geom.Extent.
type Tile interface {
	// ZXY returns the z, x and y values of the tile
	ZXY() (uint, uint, uint)
	// Extent returns the extent of the tile excluding any buffer
	Extent() (extent *geom.Extent, srid uint64)
	// BufferedExtent returns the extent of the tile including any buffer
	BufferedExtent() (extent *geom.Extent, srid uint64)
}

type Tiler interface {
	Layerer

	// TileFeature will stream decoded features to the callback function fn
	// if fn returns ErrCanceled, the TileFeatures method should stop processing
	TileFeatures(ctx context.Context, layer string, t Tile, fn func(f *Feature) error) error
}

type Layerer interface {
	// Layers returns information about the various layers the provider supports
	Layers() ([]LayerInfo, error)
}

type LayerInfo interface {
	Name() string
	GeomType() geom.Geometry
	SRID() uint64
}

// InitFunc initilize a provider given a config map. The init function should validate the config map, and report any errors. This is called by the For function.
type InitFunc func(dicter dict.Dicter) (Tiler, error)

// CleanupFunc is called to when the system is shuting down, this allows the provider to cleanup.
type CleanupFunc func()

type pfns struct {
	init    InitFunc
	cleanup CleanupFunc
}

var providers map[string]pfns

// Register the provider with the system. This call is generally made in the init functions of the provider.
// 	the clean up function will be called during shutdown of the provider to allow the provider to do any cleanup.
func Register(name string, init InitFunc, cleanup CleanupFunc) error {
	if providers == nil {
		providers = make(map[string]pfns)
	}

	if _, ok := providers[name]; ok {
		return fmt.Errorf("provider %v already exists", name)
	}

	providers[name] = pfns{
		init:    init,
		cleanup: cleanup,
	}

	return nil
}

// Drivers returns a list of registered drivers.
func Drivers() (l []string) {
	if providers == nil {
		return l
	}

	for k := range providers {
		l = append(l, k)
	}

	return l
}

// For function returns a configured provider of the given type, provided the correct config map.
func For(name string, config dict.Dicter) (Tiler, error) {
	err := ErrUnknownProvider{KnownProviders: Drivers()}
	if providers == nil {
		return nil, err
	}

	p, ok := providers[name]
	if !ok {
		err.Name = name
		return nil, err
	}

	return p.init(config)
}

func Cleanup() {
	log.Info("cleaning up providers")
	for _, p := range providers {
		if p.cleanup != nil {
			p.cleanup()
		}
	}
}
