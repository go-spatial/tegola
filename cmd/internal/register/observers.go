package register

import (
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/p"
	"github.com/alexeydott/tegola/observability"
)

func Observer(config dict.Dicter) (observability.Interface, error) {
	var oType = "none"
	if config != nil {
		oType, _ = config.String("type", p.String("none"))
	}
	return observability.For(oType, config)
}
