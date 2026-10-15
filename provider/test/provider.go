package test

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/provider"

	"github.com/alexeydott/tegola/dict"
)

const Name = "test"

var (
	lock     sync.Mutex
	Count    int
	MVTCount int
)

func init() {
	_ = provider.Register(provider.TypeStd.Prefix()+Name, NewTileProvider, Cleanup)
	_ = provider.MVTRegister(provider.TypeMvt.Prefix()+Name, NewMVTTileProvider, Cleanup)
}

// NewTileProvider setups a test provider. there are not currently any config params supported
func NewTileProvider(config dict.Dicter, maps []provider.Map) (provider.Tiler, error) {
	lock.Lock()
	Count++
	lock.Unlock()
	return &TileProvider{}, nil
}

// NewMVTTileProvider setups a test provider for mvt tiles providers. The only supported parameter is
// "test_file", which should point to a mvt tile file to return for MVTForLayers
func NewMVTTileProvider(config dict.Dicter, maps []provider.Map) (provider.MVTTiler, error) {
	lock.Lock()
	MVTCount++
	lock.Unlock()
	var mvtTile []byte
	if config != nil {
		path, err := config.String("test_file", nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get test_file key: %w", err)
		}
		mvtTile, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read test_file: %w", err)
		}
	}
	return &TileProvider{
		MVTTile: mvtTile,
	}, nil
}

// Cleanup cleans up all the test providers.
func Cleanup() {
	lock.Lock()
	Count = 0
	MVTCount = 0
	lock.Unlock()
}

// TileProvider mocks out a tile provider
type TileProvider struct {
	MVTTile []byte
	// MVTForLayersFunc overrides the canned response and receives the request
	// unchanged, allowing tests to assert inputs or inject errors.
	MVTForLayersFunc func(context.Context, provider.Tile, provider.Params, []provider.Layer) ([]byte, error)
}

// Layers returns the configured layers, there is always only one "test-layer"
func (tp *TileProvider) Layers() ([]provider.LayerInfo, error) {
	return []provider.LayerInfo{
		layer{
			name:     "test-layer",
			geomType: geom.Polygon{},
			srid:     tegola.WebMercator,
		},
	}, nil
}

// TileFeatures always returns a feature with a polygon outlining the tile's Extent (not Buffered Extent)
func (tp *TileProvider) TileFeatures(ctx context.Context, layer string, t provider.Tile, queryParams provider.Params, fn func(f *provider.Feature) error) error {
	// get tile bounding box
	ext, srid := t.Extent()

	debugTileOutline := provider.Feature{
		ID:       0,
		Geometry: ext.AsPolygon(),
		SRID:     srid,
		Tags: map[string]interface{}{
			"type": "debug_buffer_outline",
		},
	}

	return fn(&debugTileOutline)
}

// MVTForLayers delegates to MVTForLayersFunc when set. Otherwise it returns
// MVTTile verbatim, without filtering or renaming its encoded layers. The
// canned mode (including a nil receiver) preserves the legacy no-error
// behavior; callbacks own cancellation and error handling.
func (tp *TileProvider) MVTForLayers(ctx context.Context, tile provider.Tile, params provider.Params, layers []provider.Layer) ([]byte, error) {
	if tp == nil {
		return nil, nil
	}
	if tp.MVTForLayersFunc != nil {
		return tp.MVTForLayersFunc(ctx, tile, params, layers)
	}
	return tp.MVTTile, nil
}
