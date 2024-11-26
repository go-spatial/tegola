package file_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/cache/file"
	"github.com/go-spatial/tegola/dict"
)

func TestNew(t *testing.T) {
	type tcase struct {
		config   dict.Dict
		expected *file.Cache
		err      error
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			output, err := file.New(tc.config)
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
		}
	}

	tests := map[string]tcase{
		"valid basepath": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
			},
			expected: &file.Cache{
				Basepath: "testfiles/tegola-cache",
				MaxZoom:  tegola.MaxZ,
			},
			err: nil,
		},
		"valid basepath and max zoom": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": uint(9),
			},
			expected: &file.Cache{
				Basepath: "testfiles/tegola-cache",
				MaxZoom:  9,
			},
			err: nil,
		},
		"missing basepath": {
			config:   map[string]interface{}{},
			expected: nil,
			err:      file.ErrMissingBasepath,
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
		t.Run(name, fn(tc))
	}
}

func TestSetGetPurge(t *testing.T) {
	type tcase struct {
		config   dict.Dict
		key      cache.Key
		expected []byte
	}

	ctx := context.Background()
	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			fc, err := file.New(tc.config)
			if err != nil {
				t.Errorf("%v", err)
				return
			}

			// test write
			if err = fc.Set(ctx, &tc.key, tc.expected); err != nil {
				t.Errorf("write failed. err: %v", err)
				return
			}

			output, hit, err := fc.Get(ctx, &tc.key)
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
			if err = fc.Purge(ctx, &tc.key); err != nil {
				t.Errorf("purge failed. err: %v", err)
				return
			}
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
		t.Run(name, fn(tc))
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

	ctx := context.Background()
	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			fc, err := file.New(tc.config)
			if err != nil {
				t.Errorf("%v", err)
				return
			}

			// test write1
			if err = fc.Set(ctx, &tc.key, tc.bytes1); err != nil {
				t.Errorf("write failed. err: %v", err)
				return
			}

			// test write2
			if err = fc.Set(ctx, &tc.key, tc.bytes2); err != nil {
				t.Errorf("write failed. err: %v", err)
				return
			}

			// fetch the cache entry
			output, hit, err := fc.Get(ctx, &tc.key)
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
			if err = fc.Purge(ctx, &tc.key); err != nil {
				t.Errorf("purge failed. err: %v", err)
				return
			}
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
		t.Run(name, fn(tc))
	}
}

func TestMaxZoom(t *testing.T) {
	type tcase struct {
		config      dict.Dict
		key         cache.Key
		bytes       []byte
		expectedHit bool
	}

	ctx := context.Background()
	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			fc, err := file.New(tc.config)
			if err != nil {
				t.Errorf("err: %v", err)
				return
			}

			// test set
			if err = fc.Set(ctx, &tc.key, tc.bytes); err != nil {
				t.Errorf("write failed. err: %v", err)
				return
			}

			// fetch the cache entry
			_, hit, err := fc.Get(ctx, &tc.key)
			if err != nil {
				t.Errorf("read failed. err: %v", err)
				return
			}
			if hit != tc.expectedHit {
				t.Errorf("expectedHit %v got %v", tc.expectedHit, hit)
				return
			}

			// clean up
			if tc.expectedHit {
				if err != fc.Purge(ctx, &tc.key) {
					t.Errorf("%v", err)
					return
				}
			}
		}
	}

	tests := map[string]tcase{
		"over max zoom": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": uint(10),
			},
			key: cache.Key{
				Z: 11,
				X: 1,
				Y: 1,
			},
			bytes:       []byte{0x66, 0x6f, 0x6f},
			expectedHit: false,
		},
		"under max zoom": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": uint(10),
			},
			key: cache.Key{
				Z: 9,
				X: 1,
				Y: 1,
			},
			bytes:       []byte{0x66, 0x6f, 0x6f},
			expectedHit: true,
		},
		"equals max zoom": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": uint(10),
			},
			key: cache.Key{
				Z: 10,
				X: 1,
				Y: 1,
			},
			bytes:       []byte{0x66, 0x6f, 0x6f},
			expectedHit: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
