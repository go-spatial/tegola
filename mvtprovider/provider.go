package mvtprovider

import (
	"context"

	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/provider"
)

const NamePrefix = "mvt_"

type Tiler interface {
	provider.Layerer

	// MVTForLayers will return a MVT byte array or an error for the given layer names.
	MVTForLayers(ctx context.Context, tile provider.Tile, layers []Layer) ([]byte, error)
}

// InitFunc initialize a provider given a config map. The init function should validate the config map, and report any errors. This is called by the For function.
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
		return provider.ErrProviderAlreadyExists{Name: name}
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
		l = append(l, NamePrefix+k)
	}

	return l
}

// For function returns a configured provider of the given type, provided the correct config map.
func For(name string, config dict.Dicter) (Tiler, error) {
	if providers == nil {
		return nil, provider.ErrUnknownProvider{}
	}

	p, ok := providers[name]
	if !ok {
		return nil, provider.ErrUnknownProvider{Name: name}
	}

	return p.init(config)
}

func Cleanup() {
	log.Info("cleaning up mvt providers")
	for _, p := range providers {
		if p.cleanup != nil {
			p.cleanup()
		}
	}
}
