package filecache_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/terranodo/tegola/cache"
	"github.com/terranodo/tegola/cache/filecache"
)

func TestNew(t *testing.T) {
	maxZoom := uint(9)

	testcases := []struct {
		config   map[string]interface{}
		expected *filecache.Filecache
		err      error
	}{
		{
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
			},
			expected: &filecache.Filecache{
				Basepath: "testfiles/tegola-cache",
			},
			err: nil,
		},
		{
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": 9,
			},
			expected: &filecache.Filecache{
				Basepath: "testfiles/tegola-cache",
				MaxZoom:  &maxZoom,
			},
			err: nil,
		},
		{
			config:   map[string]interface{}{},
			expected: nil,
			err:      filecache.ErrMissingBasepath,
		},
		{
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": "foo",
			},
			expected: nil,
			err:      fmt.Errorf("max_zoom value needs to be of type int. Value is of type string"),
		},
	}

	for i, tc := range testcases {
		output, err := filecache.New(tc.config)
		if err != nil {
			if err.Error() == tc.err.Error() {
				//	correct error returned
				continue
			}
			t.Errorf("testcase (%v) failed. err: %v", i, err)
			continue
		}

		if !reflect.DeepEqual(tc.expected, output) {
			t.Errorf("testcase (%v) failed. expected (%+v) does not match output (%+v)", i, tc.expected, output)
			continue
		}
	}
}

func TestSetGetPurge(t *testing.T) {
	testcases := []struct {
		config   map[string]interface{}
		key      cache.Key
		expected []byte
	}{
		{
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

	for i, tc := range testcases {
		fc, err := filecache.New(tc.config)
		if err != nil {
			t.Errorf("testcase (%v) failed. err: %v", i, err)
			continue
		}

		//	test write
		if err = fc.Set(&tc.key, tc.expected); err != nil {
			t.Errorf("testcase (%v) write failed. err: %v", i, err)
			continue
		}

		output, hit, err := fc.Get(&tc.key)
		if err != nil {
			t.Errorf("testcase (%v) read failed. err: %v", i, err)
			continue
		}
		if !hit {
			t.Errorf("testcase (%v) read failed. should have been a hit but cache reported a miss", i)
			continue
		}

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("testcase (%v) failed. output (%v) does not match expected (%v)", i, output, tc.expected)
			continue
		}

		//	test purge
		if err = fc.Purge(&tc.key); err != nil {
			t.Errorf("testcase (%v) failed. purge failed. err: %v", i, err)
			continue
		}
	}
}

func TestSetOverwrite(t *testing.T) {
	testcases := []struct {
		config   map[string]interface{}
		key      cache.Key
		bytes1   []byte
		bytes2   []byte
		expected []byte
	}{
		{
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

	for i, tc := range testcases {
		fc, err := filecache.New(tc.config)
		if err != nil {
			t.Errorf("testcase (%v) failed. err: %v", i, err)
			continue
		}

		//	test write1
		if err = fc.Set(&tc.key, tc.bytes1); err != nil {
			t.Errorf("testcase (%v) write failed. err: %v", i, err)
			continue
		}

		//	test write2
		if err = fc.Set(&tc.key, tc.bytes2); err != nil {
			t.Errorf("testcase (%v) write failed. err: %v", i, err)
			continue
		}

		//	fetch the cache entry
		output, hit, err := fc.Get(&tc.key)
		if err != nil {
			t.Errorf("testcase (%v) read failed. err: %v", i, err)
			continue
		}
		if !hit {
			t.Errorf("testcase (%v) read failed. should have been a hit but cache reported a miss", i)
			continue
		}

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("testcase (%v) failed. output (%v) does not match expected (%v)", i, output, tc.expected)
			continue
		}

		//	clean up
		if err = fc.Purge(&tc.key); err != nil {
			t.Errorf("testcase (%v) failed. purge failed. err: %v", i, err)
			continue
		}
	}
}
