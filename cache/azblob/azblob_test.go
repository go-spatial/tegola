package azblob_test

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/cache/azblob"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/ttools"
	"math/rand"
)

const TESTENV = "RUN_AZBLOB_TESTS"

func TestNew(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	type tcase struct {
		config         dict.Dict
		expectReadOnly bool
		err            error
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
			config: dict.Dict{
				"container_url":   os.Getenv("AZ_CONTAINER_URL"),
				"az_account_name": os.Getenv("AZ_ACCOUNT_NAME"),
				"az_shared_key":   os.Getenv("AZ_SHARED_KEY"),
			},
			expectReadOnly: false,
			err:            nil,
		},
		"anon creds": {
			config: dict.Dict{
				"container_url": os.Getenv("AZ_CONTAINER_PUB_URL"),
			},
			expectReadOnly: true,
			err:            nil,
		},
		"invalid value for max_zoom": {
			config: dict.Dict{
				"container_url": os.Getenv("AZ_CONTAINER_PUB_URL"),
				"max_zoom":      "foo",
			},
			err: fmt.Errorf(`config: value mapped to "max_zoom" is string not uint`),
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
	ttools.ShouldSkip(t, TESTENV)

	type tcase struct {
		config   dict.Dict
		key      cache.Key
		expected []byte
	}

	fn := func(t *testing.T, tc tcase) {

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
			config: dict.Dict{
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
		"get set purge large": {
			config: dict.Dict{
				"container_url":   os.Getenv("AZ_CONTAINER_URL"),
				"az_account_name": os.Getenv("AZ_ACCOUNT_NAME"),
				"az_shared_key":   os.Getenv("AZ_SHARED_KEY"),
			},
			key: cache.Key{
				MapName: "test-map",
				Z:       3,
				X:       1,
				Y:       2,
			},
			expected: randBytes(azblob.BlobReqMaxLen * 2.5),
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func randBytes(l int) []byte {
	ret := make([]byte, l)
	rand.Read(ret)

	return ret
}

func TestSetOverwrite(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	type tcase struct {
		config   dict.Dict
		key      cache.Key
		bytes1   []byte
		bytes2   []byte
		expected []byte
	}

	fn := func(t *testing.T, tc tcase) {
		// This test must be run in series otherwise
		// there is a race condition in the
		// initialization routine (the same test file must
		// be created and destroyed)

		fc, err := azblob.New(tc.config)
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
			config: dict.Dict{
				"container_url":   os.Getenv("AZ_CONTAINER_URL"),
				"az_account_name": os.Getenv("AZ_ACCOUNT_NAME"),
				"az_shared_key":   os.Getenv("AZ_SHARED_KEY"),
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

func TestMaxZoom(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	type tcase struct {
		config      dict.Dict
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
			config: dict.Dict{
				"container_url":   os.Getenv("AZ_CONTAINER_URL"),
				"az_account_name": os.Getenv("AZ_ACCOUNT_NAME"),
				"az_shared_key":   os.Getenv("AZ_SHARED_KEY"),
				"max_zoom":        uint(10),
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
			config: dict.Dict{
				"container_url":   os.Getenv("AZ_CONTAINER_URL"),
				"az_account_name": os.Getenv("AZ_ACCOUNT_NAME"),
				"az_shared_key":   os.Getenv("AZ_SHARED_KEY"),
				"max_zoom":        uint(10),
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
			config: dict.Dict{
				"container_url":   os.Getenv("AZ_CONTAINER_URL"),
				"az_account_name": os.Getenv("AZ_ACCOUNT_NAME"),
				"az_shared_key":   os.Getenv("AZ_SHARED_KEY"),
				"max_zoom":        uint(10),
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
