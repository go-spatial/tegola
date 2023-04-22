package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/log"
)

// providerType defines the type of providers we have in the system.
// Standard providers allow layers to be co-mingled from different data sources
// because Tegola takes care of the geometry processing and mvt generation.
// MVT providers do not allow layers to be co-mingled and bypasses tegola's geometry
// processing and mvt generation.
type providerType uint8

const (

	// TypeStd declares a provider to be a standard provider
	TypeStd providerType = 1 << iota
	// TypeMvt declares a provider to be an mvt provider.
	TypeMvt

	// TypeAll should be all the types
	TypeAll = TypeStd & TypeMvt
)

func (pt providerType) Prefix() string {
	if pt == TypeMvt {
		return "mvt_"
	}
	return ""
}

func (pt providerType) String() string {
	if pt == TypeMvt {
		return "MVT Provider"
	}
	return "Standard Provider"
}

type providerFilter uint8

func providerFilterInclude(filters ...providerType) providerFilter {
	ret := uint8(0)
	for _, v := range filters {
		ret |= uint8(v)
	}
	return providerFilter(ret)
}

// Is will check to see if the filter is one of the provider types. Is acts as an or, returning
// true if any one of the provided types matches
// false if none of them match
func (pf providerFilter) Is(ps ...providerType) bool {
	t := providerFilterInclude(ps...)
	return (uint8(pf) & uint8(t)) != 0
}

// tile_t is an implementation of the Tile interface, it is
// named as such as to not confuse from the 4 other possible meanings
// of the symbol "tile" in this code base. It should be removed after
// the geom port is mostly done as part of issue #499 (removing the
// Tile interface in this package)
// TODO(@ear7h) remove this atrocity from the code base
type tile_t struct {
	slippy.Tile
	buffer uint
}

// NewTile creates a new slippy tile with a Buffer
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

// Extent returns the extent of the tile
func (tile *tile_t) Extent() (ext *geom.Extent, srid uint64) {
	return tile.Extent3857(), 3857
}

// BufferedExtent returns a the extent of the tile, with the define buffer
func (tile *tile_t) BufferedExtent() (ext *geom.Extent, srid uint64) {
	return tile.Extent3857().ExpandBy(slippy.Pixels2Webs(tile.Z, tile.buffer)), 3857
}

// Tile is an interface used by Tiler, it is an unnecessary abstraction and is
// due to be removed. The tiler interface will, instead take a, *geom.Extent.
type Tile interface {
	// ZXY returns the z, x and y values of the tile
	ZXY() (uint, uint, uint)
	// Extent returns the extent of the tile excluding any buffer
	Extent() (extent *geom.Extent, srid uint64)
	// BufferedExtent returns the extent of the tile including any buffer
	BufferedExtent() (extent *geom.Extent, srid uint64)
}

// ParameterTokenRegexp to validate QueryParameters
var ParameterTokenRegexp = regexp.MustCompile("![a-zA-Z0-9_-]+!")

// Tiler is a Layers that allows one to encode features in that layer
type Tiler interface {
	Layerer

	// TileFeature will stream decoded features to the callback function fn
	// if fn returns ErrCanceled, the TileFeatures method should stop processing
	TileFeatures(ctx context.Context, layer string, t Tile, params Params, fn func(f *Feature) error) error
}

// TilerUnion represents either a Std Tiler or and MVTTiler; only one should be not nil.
type TilerUnion struct {
	Std Tiler
	Mvt MVTTiler
}

// Layers return the layers of the Tiler. It will only return Std layers if
// STD is defined other the MVT layers
func (tu TilerUnion) Layers() ([]LayerInfo, error) {
	if tu.Std != nil {
		return tu.Std.Layers()
	}
	if tu.Mvt != nil {
		return tu.Mvt.Layers()
	}
	return nil, ErrNilInitFunc
}

// InitFunc initialize a provider given a config map. The init function should validate the config map, and report any errors. This is called by the For function.
type InitFunc func(dicter dict.Dicter, maps []Map) (Tiler, error)

// CleanupFunc is called to when the system is shutting down, this allows the provider to cleanup.
type CleanupFunc func()

type pfns struct {
	// init will be filled out if it's a standard provider
	init InitFunc
	// mvtInit will be filled out if it's a mvt provider
	mvtInit MVTInitFunc

	cleanup CleanupFunc
}

var providers map[string]pfns

// Register the provider with the system. This call is generally made in the init functions of the provider.
// 	the clean up function will be called during shutdown of the provider to allow the provider to do any cleanup.
// The init function can not be nil, the cleanup function may be nil
func Register(name string, init InitFunc, cleanup CleanupFunc) error {
	if init == nil {
		return ErrNilInitFunc
	}
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

// MVTRegister the provider with the system. This call is generally made in the init functions of the provider.
// 	the clean up function will be called during shutdown of the provider to allow the provider to do any cleanup.
// The init function can not be nil, the cleanup function may be nil
func MVTRegister(name string, init MVTInitFunc, cleanup CleanupFunc) error {
	if init == nil {
		return ErrNilInitFunc
	}
	if providers == nil {
		providers = make(map[string]pfns)
	}

	if _, ok := providers[name]; ok {
		return fmt.Errorf("provider %v already exists", name)
	}

	providers[name] = pfns{
		mvtInit: init,
		cleanup: cleanup,
	}

	return nil
}

// Drivers returns a list of registered drivers.
func Drivers(types ...providerType) (l []string) {
	if providers == nil {
		return l
	}
	filter := providerFilterInclude(types...)
	// An empty list of types should be all drivers. We do not provider a way
	// to filter out all drivers
	all := filter == 0 || filter == providerFilter(TypeAll)
	mvt := filter.Is(TypeMvt)
	std := filter.Is(TypeStd)

	for k, v := range providers {
		switch {
		case all:
		case mvt:
			if v.mvtInit == nil { // not of type mvt
				continue
			}
		case std:
			if v.init == nil { //not of type std
				continue
			}
		default:
			continue
		}
		l = append(l, k)
	}

	return l
}

// For function returns a configure provider of the given type; The provider may be a mvt provider or
// a std provider. The correct entry in TilerUnion will not be nil. If there is an error both entries
// will be nil.
func For(name string, config dict.Dicter, maps []Map) (val TilerUnion, err error) {
	var (
		driversList = Drivers()
	)
	if providers == nil {
		return val, ErrUnknownProvider{KnownProviders: driversList}
	}
	p, ok := providers[name]
	if !ok {
		return val, ErrUnknownProvider{KnownProviders: driversList, Name: name}
	}
	if p.init != nil {
		val.Std, err = p.init(config, maps)
		return val, err
	}
	if p.mvtInit != nil {
		val.Mvt, err = p.mvtInit(config, maps)
		return val, err
	}
	return val, ErrInvalidRegisteredProvider{Name: name}
}

// Cleanup is called at the end of the run to allow providers to cleanup
func Cleanup() {
	log.Info("cleaning up providers")
	for _, p := range providers {
		if p.cleanup != nil {
			p.cleanup()
		}
	}
}
