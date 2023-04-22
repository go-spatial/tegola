package atlas

import (
	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/internal/env"
	"github.com/go-spatial/tegola/observability"
	"github.com/go-spatial/tegola/provider"
)

type Layer struct {
	// optional. if not set, the ProviderLayerName will be used
	Name              string
	ProviderLayerName string
	MinZoom           uint
	MaxZoom           uint
	// instantiated provider
	Provider provider.Tiler
	// default tags to include when encoding the layer. provider tags take precedence
	DefaultTags env.Dict
	GeomType    geom.Geometry
	// DontSimplify indicates whether feature simplification should be applied.
	// We use a negative in the name so the default is to simplify
	DontSimplify bool
	// DontClip indicates whether feature clipping should be applied.
	// We use a negative in the name so the default is to clip
	DontClip bool
	// DontClean indicates whether feature cleaning (e.g. make valid) should be applied.
	// We use a negative in the name so the default is to clean
	DontClean bool
}

// MVTName will return the value that will be encoded in the Name field when the layer is encoded as MVT
func (l *Layer) MVTName() string {
	if l.Name != "" {
		return l.Name
	}

	return l.ProviderLayerName
}

func (l Layer) Collectors(prefix string, config func(configKey string) map[string]interface{}) ([]observability.Collector, error) {
	collect, ok := l.Provider.(observability.Observer)
	if !ok {
		return nil, nil
	}
	return collect.Collectors(prefix, config)
}
