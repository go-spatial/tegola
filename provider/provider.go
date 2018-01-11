package provider

import (
	"context"
	"errors"

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
	Zoom() uint64
	Extent() [2][2]float64
}

type Tiler interface {
	// TileFeature will stream decoded features to the callback function fn
	// if fn returns ErrCanceled, the TileFeatures method should stop processing
	TileFeatures(ctx context.Context, layer string, t Tile, fn func(f Feature) error) error
}
