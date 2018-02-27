package atlas

import (
	"github.com/go-spatial/tegola/geom"
	"github.com/go-spatial/tegola/provider"
)

type Layer struct {
	//	optional. if not set, the ProviderLayerName will be used
	Name              string
	ProviderLayerName string
	MinZoom           int
	MaxZoom           int
	//	instantiated provider
	Provider provider.Tiler
	//	default tags to include when encoding the layer. provider tags take precedence
	DefaultTags map[string]interface{}
	GeomType    geom.Geometry
	//	DontSimplify indicates wheather feature simplification should be applied.
	//	We use a negative in the name so the default is to simplify
	DontSimplify bool
}

//	MVTName will return the value that will be encoded in the Name field when the layer is encoded as MVT
func (l *Layer) MVTName() string {
	if l.Name != "" {
		return l.Name
	}

	return l.ProviderLayerName
}
