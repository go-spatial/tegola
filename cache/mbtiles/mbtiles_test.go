// +build cgo

package mbtiles_test

import (
	"database/sql"
	"fmt"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/cache/mbtiles"
	"github.com/go-spatial/tegola/dict"
)

func TestNew(t *testing.T) {
	type tcase struct {
		config            dict.Dict
		expected          *mbtiles.Cache
		expectedBoundsStr string
		err               error
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		output, err := mbtiles.New(tc.config)
		if err != nil {

			if tc.err != nil && err.Error() == tc.err.Error() {
				// correct error returned
				return
			}
			t.Errorf("unexpected error %v", err)
			return
		}

		if !reflect.DeepEqual(tc.expected, output) {
			t.Errorf("expected %+v got %+v", tc.expected, output)
			return
		}

		if tc.expectedBoundsStr != output.(*mbtiles.Cache).Bounds.String() {
			t.Errorf("expected %+v got %+v", tc.expectedBoundsStr, output.(*mbtiles.Cache).Bounds.String())
			return
		}
	}

	tests := map[string]tcase{
		"valid basepath": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
			},
			expected: &mbtiles.Cache{
				Basepath: "testfiles/tegola-cache",
				Bounds:   [4]float64{-180.0, -85.0511, 180, 85.0511},
				MinZoom:  0,
				MaxZoom:  tegola.MaxZ,
				DBList:   make(map[string]*sql.DB),
			},
			expectedBoundsStr: "-180.000000,-85.051100,180.000000,85.051100",
			err:               nil,
		},
		"valid basepath and max zoom": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": uint(9),
			},
			expected: &mbtiles.Cache{
				Basepath: "testfiles/tegola-cache",
				Bounds:   [4]float64{-180.0, -85.0511, 180, 85.0511},
				MinZoom:  0,
				MaxZoom:  9,
				DBList:   make(map[string]*sql.DB),
			},
			expectedBoundsStr: "-180.000000,-85.051100,180.000000,85.051100",
			err:               nil,
		},
		"valid basepath and min zoom": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": uint(2),
			},
			expected: &mbtiles.Cache{
				Basepath: "testfiles/tegola-cache",
				Bounds:   [4]float64{-180.0, -85.0511, 180, 85.0511},
				MinZoom:  0,
				MaxZoom:  2,
				DBList:   make(map[string]*sql.DB),
			},
			expectedBoundsStr: "-180.000000,-85.051100,180.000000,85.051100",
			err:               nil,
		},
		"missing basepath": {
			config:   map[string]interface{}{},
			expected: nil,
			err:      mbtiles.ErrMissingBasepath,
		},
		"invalid zoom": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": "foo",
			},
			expected: nil,
			err:      fmt.Errorf(`config: value mapped to "max_zoom" is string not uint`),
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func TestSetGetPurge(t *testing.T) {
	type tcase struct {
		config   dict.Dict
		key      cache.Key
		expected []byte
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		fc, err := mbtiles.New(tc.config)
		if err != nil {
			t.Errorf("%v", err)
			return
		}

		// test write
		if err = fc.Set(&tc.key, tc.expected); err != nil {
			t.Errorf("write failed. err: %v", err)
			return
		}

		output, hit, err := fc.Get(&tc.key)
		if err != nil {
			t.Errorf("read failed. err: %v", err)
			return
		}
		if !hit {
			t.Errorf("read failed. should have been a hit but cache reported a miss")
			return
		}

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("expected %v got %v", tc.expected, output)
			return
		}

		// test purge
		if err = fc.Purge(&tc.key); err != nil {
			t.Errorf("purge failed. err: %v", err)
			return
		}

		output, hit, err = fc.Get(&tc.key)
		if err != nil {
			t.Errorf("read failed. err: %v", err)
			return
		}
		if hit {
			t.Errorf("purge failed. should have been a miss but cache reported a hit")
			return
		}
	}

	tests := map[string]tcase{
		"get set purge": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
			},
			key: cache.Key{
				Z: 0,
				X: 1,
				Y: 2,
			},
			expected: []byte{0x53, 0x69, 0x6c, 0x61, 0x73},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func TestSetOverwrite(t *testing.T) {
	type tcase struct {
		config   dict.Dict
		key      cache.Key
		bytes1   []byte
		bytes2   []byte
		expected []byte
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		fc, err := mbtiles.New(tc.config)
		if err != nil {
			t.Errorf("%v", err)
			return
		}

		// test write1
		if err = fc.Set(&tc.key, tc.bytes1); err != nil {
			t.Errorf("write failed. err: %v", err)
			return
		}

		// test write2
		if err = fc.Set(&tc.key, tc.bytes2); err != nil {
			t.Errorf("write failed. err: %v", err)
			return
		}

		// fetch the cache entry
		output, hit, err := fc.Get(&tc.key)
		if err != nil {
			t.Errorf("read failed. err: %v", err)
			return
		}
		if !hit {
			t.Errorf("read failed. should have been a hit but cache reported a miss")
			return
		}

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("expected %v got %v", tc.expected, output)
			return
		}

		// clean up
		if err = fc.Purge(&tc.key); err != nil {
			t.Errorf("purge failed. err: %v", err)
			return
		}
	}

	tests := map[string]tcase{
		"set overwrite": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
			},
			key: cache.Key{
				Z: 0,
				X: 1,
				Y: 1,
			},
			bytes1:   []byte{0x66, 0x6f, 0x6f},
			bytes2:   []byte{0x53, 0x69, 0x6c, 0x61, 0x73},
			expected: []byte{0x53, 0x69, 0x6c, 0x61, 0x73},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
