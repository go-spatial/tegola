// +build cgo

package mbtiles

import (
	_ "github.com/mattn/go-sqlite3"

	"database/sql"
	"os"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/dict"
)

// New instantiates a Cache. The config expects the following params:
//
// 	basepath (string): a path to where the cache will be written
// 	max_zoom (int): max zoom to use the cache. beyond this zoom cache Set() calls will be ignored
// 	min_zoom (int): min zoom to use the cache. before this zoom cache Set() calls will be ignored
// 	bounds (string): bounds to use the cache. outside this bounds cache Set() calls will be ignored
//
func New(config dict.Dicter) (cache.Interface, error) {
	var err error

	// new filecache
	fc := Cache{}

	defaultMaxZoom := uint(tegola.MaxZ)
	fc.MaxZoom, err = config.Uint(ConfigKeyMaxZoom, &defaultMaxZoom)
	if err != nil {
		return nil, err
	}

	defaultMinZoom := uint(0)
	fc.MinZoom, err = config.Uint(ConfigKeyMinZoom, &defaultMinZoom)
	if err != nil {
		return nil, err
	}

	defaultBounds := "-180.0,-85,180,85"
	fc.Bounds, err = config.String(ConfigKeyBounds, &defaultBounds)
	if err != nil {
		return nil, err
	}
	//TODO clean and format bounds

	fc.Basepath, err = config.String(ConfigKeyBasepath, nil)
	if err != nil {
		return nil, ErrMissingBasepath
	}

	if fc.Basepath == "" {
		return nil, ErrMissingBasepath
	}

	// make our basepath if it does not exist
	if err = os.MkdirAll(fc.Basepath, os.ModePerm); err != nil {
		return nil, err
	}

	fc.dbList = make(map[string]*sql.DB)

	return &fc, nil
}