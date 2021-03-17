package observability

import (
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/observer"
)

var NullObserver observer.Null

func noneInit(dict.Dicter) (Interface, error) { return NullObserver, nil }
