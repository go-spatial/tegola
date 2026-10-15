package file_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/cache/file"
	"github.com/alexeydott/tegola/dict"
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

			if tc.expected != nil {
				tc.expected.Basepath, err = filepath.Abs(tc.expected.Basepath)
				if err != nil {
					t.Fatal(err)
				}
			}
			if !reflect.DeepEqual(tc.expected, output) {
				t.Errorf("expected %+v got %+v", tc.expected, output)
				return
			}
		}
	}

	tests := map[string]tcase{
		"valid basepath": {
			config: map[string]any{
				"basepath": "testfiles/tegola-cache",
			},
			expected: &file.Cache{
				Basepath: "testfiles/tegola-cache",
				MaxZoom:  tegola.MaxZ,
			},
			err: nil,
		},
		"valid basepath and max zoom": {
			config: map[string]any{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": uint(9),
			},
			expected: &file.Cache{
				Basepath: "testfiles/tegola-cache",
				MaxZoom:  9,
			},
			err: nil,
		},
		"valid basepath, max zoom and ttl": {
			config: map[string]any{
				"basepath": "testfiles/tegola-cache",
				"max_zoom": uint(9),
				"ttl":      9,
			},
			expected: &file.Cache{
				Basepath:   "testfiles/tegola-cache",
				MaxZoom:    9,
				Expiration: time.Duration(9) * time.Second,
			},
			err: nil,
		},
		"missing basepath": {
			config:   map[string]any{},
			expected: nil,
			err:      file.ErrMissingBasepath,
		},
		"invalid zoom": {
			config: map[string]any{
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

	ctx := t.Context()
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
			config: map[string]any{
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

	ctx := t.Context()
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
			config: map[string]any{
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

	ctx := t.Context()
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
			config: map[string]any{
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
			config: map[string]any{
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
			config: map[string]any{
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

func TestExpiration(t *testing.T) {
	type tcase struct {
		config   dict.Dict
		key      cache.Key
		expected []byte
	}

	ctx := t.Context()
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

			_, hit, err := fc.Get(ctx, &tc.key)
			if err != nil {
				t.Errorf("read failed. err: %v", err)
				return
			}
			if !hit {
				t.Errorf("read failed. should not have expired yet")
				return
			}

			time.Sleep(1 * time.Second)

			_, hit, err = fc.Get(ctx, &tc.key)
			if err != nil {
				t.Errorf("read failed. err: %v", err)
				return
			}
			if hit {
				t.Errorf("read succeeded. should have expired and report a miss")
				return
			}

			basePath, err := tc.config.String(file.ConfigKeyBasepath, nil)
			if err != nil {
				t.Fatal("basepath must exist in test config")
			}
			path := filepath.Join(basePath, tc.key.String())
			_, oErr := os.Open(path)
			if oErr != nil {
				if !os.IsNotExist(oErr) {
					t.Errorf("cache filed should no longer exist, when expired")
					return
				}
			}
		}
	}

	tests := map[string]tcase{
		"get set purge": {
			config: map[string]any{
				"basepath": "testfiles/tegola-cache",
				"ttl":      1,
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

func TestExpirationPurgeErrorIsReturned(t *testing.T) {
	basepath := t.TempDir()
	fc, err := file.New(dict.Dict{
		file.ConfigKeyBasepath: basepath,
		file.ConfigKeyTTL:      1,
	})
	if err != nil {
		t.Fatalf("file.New() error = %v", err)
	}

	key := cache.Key{Z: 0, X: 1, Y: 2}
	if err := fc.Set(context.Background(), &key, []byte("tile")); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	path := filepath.Join(basepath, key.String())
	expiredAt := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, expiredAt, expiredAt); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, hit, err := fc.Get(ctx, &key)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Get() error = %v, want context.Canceled", err)
	}
	if hit {
		t.Fatal("Get() reported a hit for an expired tile")
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expired file should remain when purge was canceled: %v", err)
	}

	_, hit, err = fc.Get(context.Background(), &key)
	if err != nil {
		t.Fatalf("Get() cleanup error = %v", err)
	}
	if hit {
		t.Fatal("Get() reported a hit after expired file cleanup")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired file still exists after cleanup, stat error = %v", err)
	}
}

// TestSetConcurrentSameKeyUsesUniqueTempFiles guards against concurrent Set
// calls for the same key corrupting each other through a shared temp file.
func TestSetConcurrentSameKeyUsesUniqueTempFiles(t *testing.T) {
	basepath := t.TempDir()
	fc, err := file.New(dict.Dict{
		file.ConfigKeyBasepath: basepath,
	})
	if err != nil {
		t.Fatalf("file.New() error = %v", err)
	}

	key := cache.Key{Z: 0, X: 1, Y: 2}

	const workers = 8
	payloads := make(map[string]bool, workers)
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		payload := fmt.Sprintf("tile-payload-%d", i)
		payloads[payload] = true

		wg.Add(1)
		go func(val []byte) {
			defer wg.Done()
			if err := fc.Set(context.Background(), &key, val); err != nil {
				errs <- err
			}
		}([]byte(payload))
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent Set() error = %v", err)
	}

	// the surviving value must be exactly one of the written payloads
	got, hit, err := fc.Get(context.Background(), &key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !hit {
		t.Fatal("Get() reported a miss after concurrent Set() calls")
	}
	if !payloads[string(got)] {
		t.Errorf("Get() returned %q, which is none of the written payloads", string(got))
	}

	// no temporary files may be left behind
	tmpDir := filepath.Dir(filepath.Join(basepath, key.String()))
	leftovers, err := filepath.Glob(filepath.Join(tmpDir, "*-tmp-*"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v", leftovers)
	}
}

// TestPurgeConcurrentIsIdempotent guards against racing purges: concurrent
// Purge calls for one key must all succeed even when the file disappears
// between the existence check and the removal.
func TestPurgeConcurrentIsIdempotent(t *testing.T) {
	basepath := t.TempDir()
	fc, err := file.New(dict.Dict{
		file.ConfigKeyBasepath: basepath,
	})
	if err != nil {
		t.Fatalf("file.New() error = %v", err)
	}

	key := cache.Key{Z: 0, X: 1, Y: 2}
	if err := fc.Set(context.Background(), &key, []byte("tile")); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fc.Purge(context.Background(), &key); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent Purge() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(basepath, key.String())); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("tile should be gone after purge, stat error = %v", err)
	}
}

func TestRelativeCacheRootIsFixedAtConstruction(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	c, err := file.New(dict.Dict{"basepath": "tiles"})
	if err != nil {
		t.Fatal(err)
	}
	key := &cache.Key{MapName: "map", Z: 0, X: 0, Y: 0}
	if err = c.Set(context.Background(), key, []byte("seeded")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	value, hit, err := c.Get(context.Background(), key)
	if err != nil || !hit || string(value) != "seeded" {
		t.Fatalf("cache moved with cwd: hit=%v value=%q err=%v", hit, value, err)
	}
}
