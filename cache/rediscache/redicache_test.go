package rediscache_test

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/terranodo/tegola/cache"
	"github.com/terranodo/tegola/cache/rediscache"
)

// TestNew will run tests against a local redis instance
// on 127.0.0.1:6379
func TestNew(t *testing.T) {
	if os.Getenv("RUN_REDIS_TESTS") != "yes" {
		fmt.Println("RUN_REDIS_TESTS not set to 'yes' skipping tests")
		return
	}

	testcases := []struct {
		config map[string]interface{}
		err    error
	}{
		{
			config: map[string]interface{}{
				"network":  "tcp",
				"address":  "127.0.0.1:6379",
				"password": "",
				"db":       0,
				"max_zoom": 0,
			},
			err: nil,
		},
		{
			config: map[string]interface{}{},
			err:    nil,
		},
		{
			config: map[string]interface{}{
				"address": "127.0.0.1:6000",
			},
			err: fmt.Errorf("dial tcp 127.0.0.1:6000: getsockopt: connection refused"),
		},
	}

	for i, tc := range testcases {
		_, err := rediscache.New(tc.config)
		if err != nil {
			if tc.err != nil && err.Error() == tc.err.Error() {
				//	correct error returned
				continue
			}
			t.Errorf("testcase (%v) failed. err: %v", i, err)
			continue
		}
	}
}

func TestSetGetPurge(t *testing.T) {
	if os.Getenv("RUN_REDIS_TESTS") != "yes" {
		return
	}

	testcases := []struct {
		config       map[string]interface{}
		key          cache.Key
		expectedData []byte
		expectedHit  bool
	}{
		{
			config: map[string]interface{}{},
			key: cache.Key{
				Z: 0,
				X: 1,
				Y: 2,
			},
			expectedData: []byte("\x53\x69\x6c\x61\x73"),
			expectedHit:  true,
		},
		{
			config: map[string]interface{}{},
			key: cache.Key{
				Z: 0,
				X: 0,
				Y: 0,
			},
			expectedData: nil,
			expectedHit:  true,
		},
	}

	for i, tc := range testcases {
		rc, err := rediscache.New(tc.config)
		if err != nil {
			t.Errorf("testcase (%v) failed. err: %v", i, err)
			continue
		}

		//	test write
		if tc.expectedHit {
			err = rc.Set(&tc.key, tc.expectedData)
			if err != nil {
				t.Errorf("testcase (%v) write failed. err: %v", i, err)
			}
			continue
		}

		// test read
		output, hit, err := rc.Get(&tc.key)
		if err != nil {
			t.Errorf("testcase (%v) read failed. err: %v", i, err)
			continue
		}
		if tc.expectedHit == hit {
			t.Errorf("testcase (%v) read failed. should hit should have been %b but cache reported a %b", i, tc.expectedHit, hit)
			continue
		}

		if !reflect.DeepEqual(output, tc.expectedData) {
			t.Errorf("testcase (%v) failed. output (%v) does not match expected (%v)", i, output, tc.expectedData)
			continue
		}

		//	test purge
		if tc.expectedHit {
			err = rc.Purge(&tc.key)
			if err != nil {
				t.Errorf("testcase (%v) failed. purge failed. err: %v", i, err)
				continue
			}
		} else {
			// test purge non-existent key
			err = rc.Purge(&tc.key)
			if err != nil {
				t.Errorf("testcase (%v) failed. purge failed. err: %v", i, err)
				continue
			}
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
			config: map[string]interface{}{},
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
		rc, err := rediscache.New(tc.config)
		if err != nil {
			t.Errorf("testcase (%v) failed. err: %v", i, err)
			continue
		}

		//	test write1
		if err = rc.Set(&tc.key, tc.bytes1); err != nil {
			t.Errorf("testcase (%v) write failed. err: %v", i, err)
			continue
		}

		//	test write2
		if err = rc.Set(&tc.key, tc.bytes2); err != nil {
			t.Errorf("testcase (%v) write failed. err: %v", i, err)
			continue
		}

		//	fetch the cache entry
		output, hit, err := rc.Get(&tc.key)
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
		if err = rc.Purge(&tc.key); err != nil {
			t.Errorf("testcase (%v) failed. purge failed. err: %v", i, err)
			continue
		}
	}
}
