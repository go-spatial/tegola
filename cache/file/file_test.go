package file_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/terranodo/tegola/cache"
	"github.com/terranodo/tegola/cache/file"
)

func TestNew(t *testing.T) {
	maxZoom := uint(9)

	type tcase struct {
		config   map[string]interface{}
		expected *file.Cache
		err      error
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		output, err := file.New(tc.config)
		if err != nil {
			if err.Error() == tc.err.Error() {
				//	correct error returned
				return
			}
			t.Errorf("%v", err)
			return
		}

		if !reflect.DeepEqual(tc.expected, output) {
			t.Errorf("expected %+v got %+v", tc.expected, output)
			return
		}
	}

	tests := map[string]tcase{
		"valid basepath": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
			},
			expected: &file.Cache{
				Basepath: "testfiles/tegola-cache",
			},
			err: nil,
		},
		"valid basepath and max zoom": {
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": 9,
			},
			expected: &file.Cache{
				Basepath: "testfiles/tegola-cache",
				MaxZoom:  &maxZoom,
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
			err:      fmt.Errorf("max_zoom value needs to be of type int. Value is of type string"),
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
		config   map[string]interface{}
		key      cache.Key
		expected []byte
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		fc, err := file.New(tc.config)
		if err != nil {
			t.Errorf("%v", err)
			return
		}

		//	test write
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

		//	test purge
		if err = fc.Purge(&tc.key); err != nil {
			t.Errorf("purge failed. err: %v", err)
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
			expected: []byte("\x53\x69\x6c\x61\x73"),
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
		config   map[string]interface{}
		key      cache.Key
		bytes1   []byte
		bytes2   []byte
		expected []byte
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		fc, err := file.New(tc.config)
		if err != nil {
			t.Errorf("%v", err)
			return
		}

		//	test write1
		if err = fc.Set(&tc.key, tc.bytes1); err != nil {
			t.Errorf("write failed. err: %v", err)
			return
		}

		//	test write2
		if err = fc.Set(&tc.key, tc.bytes2); err != nil {
			t.Errorf("write failed. err: %v", err)
			return
		}

		//	fetch the cache entry
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

		//	clean up
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
			bytes1:   []byte("\x66\x6f\x6f"),
			bytes2:   []byte("\x53\x69\x6c\x61\x73"),
			expected: []byte("\x53\x69\x6c\x61\x73"),
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func TestMaxZoom(t *testing.T) {
	type tcase struct {
		config      map[string]interface{}
		key         cache.Key
		bytes       []byte
		expectedHit bool
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		fc, err := file.New(tc.config)
		if err != nil {
			t.Errorf("err: %v", err)
			return
		}

		//	test set
		if err = fc.Set(&tc.key, tc.bytes); err != nil {
			t.Errorf("write failed. err: %v", err)
			return
		}

		//	fetch the cache entry
		_, hit, err := fc.Get(&tc.key)
		if err != nil {
			t.Errorf("read failed. err: %v", err)
			return
		}
		if hit != tc.expectedHit {
			t.Errorf("expectedHit %v got %v", tc.expectedHit, hit)
			return
		}

		//	clean up
		if tc.expectedHit {
			if err != fc.Purge(&tc.key) {
				t.Errorf("%v", err)
				return
			}
		}
	}

	tests := map[string]tcase{
		"over max zoom": tcase{
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": 10,
			},
			key: cache.Key{
				Z: 11,
				X: 1,
				Y: 1,
			},
			bytes:       []byte("\x66\x6f\x6f"),
			expectedHit: false,
		},
		"under max zoom": tcase{
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": 10,
			},
			key: cache.Key{
				Z: 9,
				X: 1,
				Y: 1,
			},
			bytes:       []byte("\x66\x6f\x6f"),
			expectedHit: true,
		},
		"equals max zoom": tcase{
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": 10,
			},
			key: cache.Key{
				Z: 10,
				X: 1,
				Y: 1,
			},
			bytes:       []byte("\x66\x6f\x6f"),
			expectedHit: true,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
