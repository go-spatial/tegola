package rediscache_test

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/terranodo/tegola/cache"
	"github.com/terranodo/tegola/cache/rediscache"
	"strings"
)

// TestNew will run tests against a local redis instance
// on 127.0.0.1:6379
func TestNew(t *testing.T) {
	if os.Getenv("RUN_REDIS_TESTS") != "yes" {
		fmt.Println("RUN_REDIS_TESTS not set to 'yes' skipping tests")
		return
	}

	type tc struct {
		config map[string]interface{}
		errMatch    string
	}

	testcases := map[string]tc{
		"redis explicit config": {
			config: map[string]interface{}{
				"network":  "tcp",
				"address":  "127.0.0.1:6379",
				"password": "",
				"db":       0,
				"max_zoom": 0,
			},
			errMatch: "",
		},
		"redis implicit config": {
			config: map[string]interface{}{},
			errMatch:    "",
		},
		"redis bad config":{
			config: map[string]interface{}{
				"address": "127.0.0.1:6000",
			},
			errMatch: "connection refused",
		},
	}

	for i, tc := range testcases {
		_, err := rediscache.New(tc.config)
		if err != nil {
			if tc.errMatch != "" && strings.Contains(err.Error(), tc.errMatch) {
				//	correct error returned
				continue
			}
			t.Errorf("[%v] unexpected err, expected to find %v in %v", i, tc.errMatch, err)
			continue
		}
	}
}

func TestSetGetPurge(t *testing.T) {
	if os.Getenv("RUN_REDIS_TESTS") != "yes" {
		return
	}

	type tc struct {
		config       map[string]interface{}
		key          cache.Key
		expectedData []byte
		expectedHit  bool
	}

	testcases := map[string]tc {
		"redis cache hit": {
			config: map[string]interface{}{},
			key: cache.Key{
				Z: 0,
				X: 1,
				Y: 2,
			},
			expectedData: []byte("\x53\x69\x6c\x61\x73"),
			expectedHit:  true,
		},
		"redis cache miss": {
			config: map[string]interface{}{},
			key: cache.Key{
				Z: 0,
				X: 0,
				Y: 0,
			},
			expectedData: []byte(nil),
			expectedHit:  false,
		},
	}

	for k, tc := range testcases {
		rc, err := rediscache.New(tc.config)
		if err != nil {
			t.Errorf("[%v] unexpected err, expected %v got %v", k, nil, err)
			continue
		}

		//	test write
		if tc.expectedHit {
			err = rc.Set(&tc.key, tc.expectedData)
			if err != nil {
				t.Errorf("[%v] unexpected err, expected %v got %v", k, nil, err)
			}
			continue
		}

		// test read
		output, hit, err := rc.Get(&tc.key)
		if err != nil {
			t.Errorf("[%v] read failed with error, expected %v got %v", k, nil, err)
			continue
		}
		if tc.expectedHit != hit {
			t.Errorf("[%v] read failed, wrong 'hit' value expected %t got %t", k, tc.expectedHit, hit)
			continue
		}

		if !reflect.DeepEqual(output, tc.expectedData) {
			t.Errorf("[%v] read failed, expected %v got %v", k, output, tc.expectedData)
			continue
		}

		//	test purge
		if tc.expectedHit {
			err = rc.Purge(&tc.key)
			if err != nil {
				t.Errorf("[%v] purge failed with err, expected %v got %v", k, nil, err)
				continue
			}
		}
	}
}

func TestSetOverwrite(t *testing.T) {
	type tc struct {
		config   map[string]interface{}
		key      cache.Key
		bytes1   []byte
		bytes2   []byte
		expected []byte
	}

	testcases := map[string]tc{
		"0": {
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

	for k, tc := range testcases {
		rc, err := rediscache.New(tc.config)
		if err != nil {
			t.Errorf("[%v] unexpected err, expected %v got %v", k, nil, err)
			continue
		}

		//	test write1
		if err = rc.Set(&tc.key, tc.bytes1); err != nil {
			t.Errorf("[%v] write failed with err, expected %v got %v", k, nil, err)
			continue
		}

		//	test write2
		if err = rc.Set(&tc.key, tc.bytes2); err != nil {
			t.Errorf("[%v] write failed with err, expected %v got %v", k, nil, err)
			continue
		}

		//	fetch the cache entry
		output, hit, err := rc.Get(&tc.key)
		if err != nil {
			t.Errorf("[%v] read failed with err, expected %v got %v", k, nil, err)
			continue
		}
		if !hit {
			t.Errorf("[%v] read failed, expected hit %t got %t", k, true, hit)
			continue
		}

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("[%v] read failed, expected %v got %v)", k, output, tc.expected)
			continue
		}

		//	clean up
		if err = rc.Purge(&tc.key); err != nil {
			t.Errorf("[%v] purge failed with err, expected %v got %v", k, nil, err)
			continue
		}
	}
}
