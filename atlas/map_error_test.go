package atlas

import (
	"context"
	"errors"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/slippy"
	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/provider"
)

var errTileFeatures = errors.New("tile features failed")

type errorTileProvider struct{}

func (errorTileProvider) Layers() ([]provider.LayerInfo, error) { return nil, nil }

func (errorTileProvider) TileFeatures(context.Context, string, provider.Tile, provider.Params, func(*provider.Feature) error) error {
	return errTileFeatures
}

func TestEncodeMVTTileReturnsLayerErrors(t *testing.T) {
	m := Map{
		SRID: tegola.WebMercator,
		Layers: []Layer{{
			Name:              "test",
			ProviderLayerName: "test",
			Provider:          errorTileProvider{},
			GeomType:          geom.LineString{},
		}},
	}

	_, err := m.encodeMVTTile(context.Background(), slippy.Tile{Z: 0}, nil)
	if !errors.Is(err, errTileFeatures) {
		t.Fatalf("expected tile feature error, got %v", err)
	}
}
