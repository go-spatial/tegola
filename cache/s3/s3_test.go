package s3_test

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/cache/s3"
	"github.com/go-spatial/tegola/dict"
)

// gzip encoded test data
var testData = []byte{0x1f, 0x8b, 0x8, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xff, 0x2a, 0xce, 0xcc, 0x49, 0x2c, 0x6, 0x4, 0x0, 0x0, 0xff, 0xff, 0xaf, 0x9d, 0x59, 0xca, 0x5, 0x0, 0x0, 0x0}

// skipS3Test will check the environment to see if the test should be skipped
func skipS3Tests(t *testing.T) {
	if strings.TrimSpace(strings.ToLower(os.Getenv("RUN_S3_TESTS"))) != "yes" {
		t.Skipf("skipping %v, RUN_S3_TESTS not set to 'yes'", t.Name())
	}
	// Test for static Credentials as well. If there are not there we skip the tests

	var config dict.Dict = map[string]interface{}{
		"bucket":                os.Getenv("AWS_TEST_BUCKET"),
		"region":                os.Getenv("AWS_REGION"),
		"aws_access_key_id":     os.Getenv("AWS_ACCESS_KEY_ID"),
		"aws_secret_access_key": os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}
	_, err := s3.New(config)
	if err != nil {
		t.Skipf("skipping %v, static ENV's not set or correct", t.Name())
	}

}

func TestNew(t *testing.T) {
	skipS3Tests(t)

	type tcase struct {
		config dict.Dict
		err    error
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		_, err := s3.New(tc.config)
		if err != nil {
			if tc.err == nil {
				t.Errorf("received unexpected err: %v", err)
				return
			}
			if err.Error() == tc.err.Error() {
				// correct error returned
				return
			}
			t.Errorf("%v", err)
			return
		}
	}

	tests := map[string]tcase{
		"static creds": {
			config: map[string]interface{}{
				"bucket":                os.Getenv("AWS_TEST_BUCKET"),
				"region":                os.Getenv("AWS_REGION"),
				"aws_access_key_id":     os.Getenv("AWS_ACCESS_KEY_ID"),
				"aws_secret_access_key": os.Getenv("AWS_SECRET_ACCESS_KEY"),
			},
			err: nil,
		},
		"env var creds": {
			config: map[string]interface{}{
				"bucket":   os.Getenv("AWS_TEST_BUCKET"),
				"max_zoom": uint(9),
				"region":   os.Getenv("AWS_REGION"),
			},
			err: nil,
		},
		"missing bucket": {
			config: map[string]interface{}{},
			err:    s3.ErrMissingBucket,
		},
		"invalid value for max_zoom": {
			config: map[string]interface{}{
				"bucket":   os.Getenv("AWS_TEST_BUCKET"),
				"max_zoom": "foo",
			},
			err: fmt.Errorf("config: value mapped to \"max_zoom\" is string not uint"),
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
	skipS3Tests(t)

	type tcase struct {
		config   dict.Dict
		key      cache.Key
		expected []byte
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		fc, err := s3.New(tc.config)
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
	}

	tests := map[string]tcase{
		"get set purge": {
			config: map[string]interface{}{
				"bucket":   os.Getenv("AWS_TEST_BUCKET"),
				"basepath": "cache",
			},
			key: cache.Key{
				MapName: "test-map",
				Z:       0,
				X:       1,
				Y:       2,
			},
			expected: testData,
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
	skipS3Tests(t)

	type tcase struct {
		config   dict.Dict
		key      cache.Key
		bytes1   []byte
		bytes2   []byte
		expected []byte
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		fc, err := s3.New(tc.config)
		if err != nil {
			t.Errorf("%v", err)
			return
		}

		// test write1
		if err = fc.Set(&tc.key, tc.bytes1); err != nil {
			t.Errorf("write 1 failed. err: %v", err)
			return
		}

		// test write2
		if err = fc.Set(&tc.key, tc.bytes2); err != nil {
			t.Errorf("write 2 failed. err: %v", err)
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
		"overwrite": {
			config: map[string]interface{}{
				"bucket": os.Getenv("AWS_TEST_BUCKET"),
				"region": os.Getenv("AWS_REGION"),
			},
			key: cache.Key{
				Z: 0,
				X: 1,
				Y: 1,
			},
			bytes1:   []byte{0x66, 0x6f, 0x6f},
			bytes2:   testData,
			expected: testData,
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
	skipS3Tests(t)

	type tcase struct {
		config      dict.Dict
		key         cache.Key
		bytes       []byte
		expectedHit bool
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		fc, err := s3.New(tc.config)
		if err != nil {
			t.Errorf("%v", err)
			return
		}

		// test set
		if err = fc.Set(&tc.key, tc.bytes); err != nil {
			t.Errorf("write failed. err: %v", err)
			return
		}

		// fetch the cache entry
		_, hit, err := fc.Get(&tc.key)
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
			if err != fc.Purge(&tc.key) {
				t.Errorf("%v", err)
				return
			}
		}
	}

	tests := map[string]tcase{
		"over max zoom": {
			config: map[string]interface{}{
				"bucket":   os.Getenv("AWS_TEST_BUCKET"),
				"region":   os.Getenv("AWS_REGION"),
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
				"bucket":   os.Getenv("AWS_TEST_BUCKET"),
				"region":   os.Getenv("AWS_REGION"),
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
				"bucket":   os.Getenv("AWS_TEST_BUCKET"),
				"region":   os.Getenv("AWS_REGION"),
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
		tc := tc
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
