package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/terranodo/tegola/geom"
)

type Feature struct {
	ID       uint64
	Geometry geom.Geometry
	SRID     int
	Tags     map[string]interface{}
}

var ErrCanceled = errors.New("provider: canceled")

type Tile interface {
	Z() uint64
	X() uint64
	Y() uint64
	//	Extent returns the extent of the tile excluding any buffer
	Extent() (extent [2][2]float64, srid uint64)
	//	BufferedExtent returns the extent of the tile including any buffer
	BufferedExtent() (extent [2][2]float64, srid uint64)
}

type Tiler interface {
	// TileFeature will stream decoded features to the callback function fn
	// if fn returns ErrCanceled, the TileFeatures method should stop processing
	TileFeatures(ctx context.Context, layer string, t Tile, fn func(f *Feature) error) error
	// Layers returns information about the various layers the provider supports
	Layers() ([]LayerInfo, error)
}

type LayerInfo interface {
	Name() string
	GeomType() geom.Geometry
	SRID() int
}

// InitFunc initilize a provider given a config map. The init function should validate the config map, and report any errors. This is called by the For function.
type InitFunc func(map[string]interface{}) (Tiler, error)

var providers map[string]InitFunc

// Register is called by the init functions of the provider.
func Register(name string, init InitFunc) error {
	if providers == nil {
		providers = make(map[string]InitFunc)
	}

	if _, ok := providers[name]; ok {
		return fmt.Errorf("Provider %v already exists", name)
	}

	providers[name] = init

	return nil
}

// Drivers returns a list of drivers that have registered.
func Drivers() (l []string) {
	if providers == nil {
		return l
	}

	for k, _ := range providers {
		l = append(l, k)
	}

	return l
}

// For function returns a configured provider of the given type, provided the correct config map.
func For(name string, config map[string]interface{}) (Tiler, error) {
	if providers == nil {
		return nil, fmt.Errorf("No providers registered.")
	}

	p, ok := providers[name]
	if !ok {
		return nil, fmt.Errorf("No providers registered by the name: %v, known providers(%v)", name, strings.Join(Drivers(), ","))
	}

	return p(config)
}
