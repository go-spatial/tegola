package observability

import (
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/observer"
)

var NullObserver observer.Null

func noneInit(dict.Dicter) (Interface, error) { return NullObserver, nil }
