package register

import (
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/p"
	"github.com/go-spatial/tegola/observability"
)

func Observer(config dict.Dicter) (observability.Interface, error) {
	var oType = "none"
	if config != nil {
		oType, _ = config.String("type", p.String("none"))
	}
	return observability.For(oType, config)
}
