package azblob_test

import (
	"testing"
	"os"
	"github.com/go-spatial/tegola/cache/azblob"
	"fmt"
	"reflect"
	"github.com/go-spatial/tegola/cache"
)

func TestNew(t *testing.T) {
	if os.Getenv("RUN_AZBLOB_TESTS") != "yes" {
		return
	}

	type tcase struct {
		config map[string]interface{}
		expectReadOnly bool
		err    error
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		c, err := azblob.New(tc.config)
		if err != nil {
			if tc.err == nil {
				t.Errorf("unexpected err %v", err)
				return
			}


			if err.Error() == tc.err.Error() {
				// correct error returned
				return
			}
			t.Errorf("unexpected err, got %v expected %v", err, tc.err)
			return
		}

		azb := c.(*azblob.Cache)

		if tc.expectReadOnly != azb.ReadOnly {
			t.Errorf("unexpected (*azblob.Cache).ReadOnly value got %v expected %v", azb.ReadOnly, tc.expectReadOnly)
			return
		}
	}

	tests := map[string]tcase{
		"static creds": {
			config: map[string]interface{}{
				"container_url":   os.Getenv("AZ_CONTAINER_URL"),
				"az_account_name": os.Getenv("AZ_ACCOUNT_NAME"),
				"az_shared_key":   os.Getenv("AZ_SHARED_KEY"),
			},
			expectReadOnly: false,
			err: nil,
		},
		"anon creds": {
			config: map[string]interface{}{
				"container_url":   os.Getenv("AZ_CONTAINER_PUB_URL"),
			},
			expectReadOnly: true,
			err: nil,
		},
		"invalid value for max_zoom": {
			config: map[string]interface{}{
				"container_url":   os.Getenv("AZ_CONTAINER_PUB_URL"),
				"max_zoom": "foo",
			},
			err: fmt.Errorf("max_zoom value needs to be of type uint. Value is of type string"),
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
	if os.Getenv("RUN_AZBLOB_TESTS") != "yes" {
		return
	}

	type tcase struct {
		config   map[string]interface{}
		key      cache.Key
		expected []byte
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		fc, err := azblob.New(tc.config)
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
				"container_url":   os.Getenv("AZ_CONTAINER_URL"),
				"az_account_name": os.Getenv("AZ_ACCOUNT_NAME"),
				"az_shared_key":   os.Getenv("AZ_SHARED_KEY"),
			},
			key: cache.Key{
				MapName: "test-map",
				Z:       0,
				X:       1,
				Y:       2,
			},
			expected: []byte("\x41\x74\x6c\x61\x73\x20\x54\x65\x6c\x61\x6d\x6f\x6e"),
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
	if os.Getenv("RUN_AZBLOB_TESTS") != "yes" {
		return
	}

	type tcase struct {
		config      map[string]interface{}
		key         cache.Key
		bytes       []byte
		expectedHit bool
	}

	fn := func(t *testing.T, tc tcase) {
		// This test must be run in series otherwise
		// there is a race condition in the
		// initialization routine (the same test file must
		// be created and destroyed)

		fc, err := azblob.New(tc.config)
		if err != nil {
			t.Errorf("error initializing %v", err)
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
				t.Errorf("error cleaning %v", err)
				return
			}
		}
	}

	tests := map[string]tcase{
		"over max zoom": {
			config: map[string]interface{}{
				"container_url":   os.Getenv("AZ_CONTAINER_URL"),
				"az_account_name": os.Getenv("AZ_ACCOUNT_NAME"),
				"az_shared_key":   os.Getenv("AZ_SHARED_KEY"),
				"max_zoom": uint(10),
			},
			key: cache.Key{
				Z: 11,
				X: 1,
				Y: 1,
			},
			bytes:       []byte("\x66\x6f\x6f"),
			expectedHit: false,
		},
		"under max zoom": {
			config: map[string]interface{}{
				"container_url":   os.Getenv("AZ_CONTAINER_URL"),
				"az_account_name": os.Getenv("AZ_ACCOUNT_NAME"),
				"az_shared_key":   os.Getenv("AZ_SHARED_KEY"),
				"max_zoom": uint(10),
			},
			key: cache.Key{
				Z: 9,
				X: 1,
				Y: 1,
			},
			bytes:       []byte("\x66\x6f\x6f"),
			expectedHit: true,
		},
		"equals max zoom": {
			config: map[string]interface{}{
				"container_url":   os.Getenv("AZ_CONTAINER_URL"),
				"az_account_name": os.Getenv("AZ_ACCOUNT_NAME"),
				"az_shared_key":   os.Getenv("AZ_SHARED_KEY"),
				"max_zoom": uint(10),
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
