package s3cache_test

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/terranodo/tegola/cache"
	"github.com/terranodo/tegola/cache/s3cache"
)

func TestNew(t *testing.T) {
	if os.Getenv("RUN_S3_TESTS") != "yes" {
		return
	}

	testcases := []struct {
		config map[string]interface{}
		err    error
	}{
		//	test static creds
		{
			config: map[string]interface{}{
				"bucket":                "tegola-test-data",
				"region":                "us-west-1",
				"aws_access_key_id":     os.Getenv("AWS_ACCESS_KEY_ID"),
				"aws_secret_access_key": os.Getenv("AWS_SECRET_ACCESS_KEY"),
			},
			err: nil,
		},
		//	test env var creds and max zoom
		{
			config: map[string]interface{}{
				"bucket":   "tegola-test-data",
				"max_zoom": 9,
				"region":   "us-west-1",
			},
			err: nil,
		},
		//	missing bucket
		{
			config: map[string]interface{}{},
			err:    s3cache.ErrMissingBucket,
		},
		//	invalid value for max_zoom
		{
			config: map[string]interface{}{
				"bucket":   "tegola-test-data",
				"max_zoom": "foo",
			},
			err: fmt.Errorf("max_zoom value needs to be of type int. Value is of type string"),
		},
	}

	for i, tc := range testcases {
		_, err := s3cache.New(tc.config)
		if err != nil {
			if tc.err == nil {
				t.Errorf("testcase (%v) failed. received unexpected err: %v", i, err)
				continue
			}
			if err.Error() == tc.err.Error() {
				//	correct error returned
				continue
			}
			t.Errorf("testcase (%v) failed. err: %v", i, err)
			continue
		}
	}
}

func TestSetGetPurge(t *testing.T) {
	if os.Getenv("RUN_S3_TESTS") != "yes" {
		return
	}

	testcases := []struct {
		config   map[string]interface{}
		key      cache.Key
		expected []byte
	}{
		{
			config: map[string]interface{}{
				"bucket": "tegola-test-data",
				"region": "us-west-1",
			},
			key: cache.Key{
				Z: 0,
				X: 1,
				Y: 2,
			},
			expected: []byte{0x53, 0x69, 0x6c, 0x61, 0x73},
		},
	}

	for i, tc := range testcases {
		fc, err := s3cache.New(tc.config)
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
	if os.Getenv("RUN_S3_TESTS") != "yes" {
		return
	}

	testcases := []struct {
		config   map[string]interface{}
		key      cache.Key
		bytes1   []byte
		bytes2   []byte
		expected []byte
	}{
		{
			config: map[string]interface{}{
				"bucket": "tegola-test-data",
				"region": "us-west-1",
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

	for i, tc := range testcases {
		fc, err := s3cache.New(tc.config)
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
