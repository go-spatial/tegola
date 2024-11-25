package memory_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/cache/memory"
	"github.com/go-spatial/tegola/dict"
)

func TestSetGetPurge(t *testing.T) {
	ctx := context.Background()

	type tcase struct {
		config       dict.Dict
		key          cache.Key
		expectedData []byte
		expectedHit  bool
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			mc, err := memory.New(tc.config)
			if err != nil {
				t.Errorf("unexpected err, expected %v got %v", nil, err)
				return
			}

			// test write
			if tc.expectedHit {
				err = mc.Set(ctx, &tc.key, tc.expectedData)
				if err != nil {
					t.Errorf("unexpected err, expected %v got %v", nil, err)
				}
				return
			}

			// test read
			output, hit, err := mc.Get(ctx, &tc.key)
			if err != nil {
				t.Errorf("read failed with error, expected %v got %v", nil, err)
				return
			}
			if tc.expectedHit != hit {
				t.Errorf("read failed, wrong 'hit' value expected %t got %t", tc.expectedHit, hit)
				return
			}

			if !reflect.DeepEqual(output, tc.expectedData) {
				t.Errorf("read failed, expected %v got %v", output, tc.expectedData)
				return
			}

			// test purge
			if tc.expectedHit {
				err = mc.Purge(ctx, &tc.key)
				if err != nil {
					t.Errorf("purge failed with err, expected %v got %v", nil, err)
					return
				}
			}
		}
	}

	testcases := map[string]tcase{
		"memory cache hit": {
			config: map[string]any{},
			key: cache.Key{
				Z: 0,
				X: 1,
				Y: 2,
			},
			expectedData: []byte("\x53\x69\x6c\x61\x73"),
			expectedHit:  true,
		},
		"memory cache miss": {
			config: map[string]any{},
			key: cache.Key{
				Z: 0,
				X: 0,
				Y: 0,
			},
			expectedData: []byte(nil),
			expectedHit:  false,
		},
	}

	for name, tc := range testcases {
		t.Run(name, fn(tc))
	}
}

func TestSetOverwrite(t *testing.T) {
	ctx := context.Background()

	type tcase struct {
		config   dict.Dict
		key      cache.Key
		bytes1   []byte
		bytes2   []byte
		expected []byte
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			mc, err := memory.New(tc.config)
			if err != nil {
				t.Errorf("unexpected err, expected %v got %v", nil, err)
				return
			}

			// test write1
			if err = mc.Set(ctx, &tc.key, tc.bytes1); err != nil {
				t.Errorf("write failed with err, expected %v got %v", nil, err)
				return
			}

			// test write2
			if err = mc.Set(ctx, &tc.key, tc.bytes2); err != nil {
				t.Errorf("write failed with err, expected %v got %v", nil, err)
				return
			}

			// fetch the cache entry
			output, hit, err := mc.Get(ctx, &tc.key)
			if err != nil {
				t.Errorf("read failed with err, expected %v got %v", nil, err)
				return
			}
			if !hit {
				t.Errorf("read failed, expected hit %t got %t", true, hit)
				return
			}

			if !reflect.DeepEqual(output, tc.expected) {
				t.Errorf("read failed, expected %v got %v)", output, tc.expected)
				return
			}

			// clean up
			if err = mc.Purge(ctx, &tc.key); err != nil {
				t.Errorf("purge failed with err, expected %v got %v", nil, err)
				return
			}
		}
	}

	testcases := map[string]tcase{
		"memory overwrite": {
			config: map[string]any{},
			key: cache.Key{
				Z: 0,
				X: 1,
				Y: 1,
			},
			bytes1:   []byte("\x66\x6f\x6f"),
			bytes2:   []byte("\x53\x69\x6c\x61\x73"),
			expected: []byte("\x53\x69\x6c\x61\x73"),
		},
	}

	for name, tc := range testcases {
		t.Run(name, fn(tc))
	}
}
