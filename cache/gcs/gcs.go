package gcs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path"

	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/log"

	"cloud.google.com/go/storage"
)

const CacheType = "gcs"

var (
	ErrMissingBucket = errors.New("cache_gcs: missing required param 'bucket'")
)

const (
	// required
	ConfigKeyBucketName = "bucket"

	// optional
	ConfigKeyBasepath = "basepath"
	ConfigKeyMaxZoom  = "max_zoom"
)

// testData is used during New() to confirm the ability to write, read and purge the cache
var testData = []byte{0x1f, 0x8b, 0x8, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xff, 0x2a, 0xce, 0xcc, 0x49, 0x2c, 0x6, 0x4, 0x0, 0x0, 0xff, 0xff, 0xaf, 0x9d, 0x59, 0xca, 0x5, 0x0, 0x0, 0x0}

func init() {
	_ = cache.Register(CacheType, New)
}

func New(config dict.Dicter) (cache.Interface, error) {
	var err error

	gcsCache := GCSCache{}

	gcsCache.BucketName, err = config.String(ConfigKeyBucketName, nil)
	if err != nil {
		return nil, ErrMissingBucket
	}

	gcsCache.Basepath, err = config.String(ConfigKeyBasepath, nil)
	if err != nil {
		return nil, err
	}

	defaultMaxZoom := uint(tegola.MaxZ)

	gcsCache.MaxZoom, err = config.Uint(ConfigKeyMaxZoom, &defaultMaxZoom)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	gcsCache.Client = client
	gcsCache.Bucket = client.Bucket(gcsCache.BucketName)

	// in order to confirm we have the correct permissions on the bucket create a small file
	// and test a PUT, GET and DELETE to the bucket
	key := cache.Key{
		MapName:   "tegola-test-map",
		LayerName: "test-layer",
		Z:         0,
		X:         0,
		Y:         0,
	}

	if err := selfTestCache(ctx, &gcsCache, &key, testData); err != nil {
		return nil, err
	}

	return &gcsCache, nil
}

// selfTestCache confirms the cache is usable end to end: the test value must be
// written, read back as an actual cache hit with identical bytes, and purged.
// It runs once when the cache is created.
func selfTestCache(ctx context.Context, c cache.Interface, key *cache.Key, data []byte) error {
	// write gzip encoded test file
	if err := c.Set(ctx, key, data); err != nil {
		return cache.ErrSettingToCache{
			CacheType: CacheType,
			Err:       err,
		}
	}

	// read the test file
	got, hit, err := c.Get(ctx, key)
	if err != nil {
		return cache.ErrGettingFromCache{
			CacheType: CacheType,
			Err:       err,
		}
	}
	if !hit {
		return cache.ErrGettingFromCache{
			CacheType: CacheType,
			Err:       errors.New("cache self-test read did not hit"),
		}
	}
	if !bytes.Equal(got, data) {
		return cache.ErrGettingFromCache{
			CacheType: CacheType,
			Err:       errors.New("cache self-test read returned different data"),
		}
	}

	// purge the test file
	if err := c.Purge(ctx, key); err != nil {
		return cache.ErrPurgingCache{
			CacheType: CacheType,
			Err:       err,
		}
	}

	return nil
}

type GCSCache struct {
	// Bucket is the name of the GCS bucket to operate on
	BucketName string

	// Basepath is a path prefix added to all cache operations inside of the GCS bucket
	// helpful so a bucket does not need to be dedicated to only this cache
	Basepath string

	// MaxZoom determines the max zoom the cache to persist. Beyond this
	// zoom, cache Set() calls will be ignored. This is useful if the cache
	// should not be leveraged for higher zooms when data changes often.
	MaxZoom uint

	// client holds a reference to the storage client. it's expected the client
	// has an active session and read, write, delete permissions have been checked
	Client *storage.Client

	// bucket holds a reference to the bucket handle.
	Bucket *storage.BucketHandle

	// newReader opens the object identified by key and returns a reader over its
	// contents. When nil, readers are opened from Bucket. It exists as a field so
	// tests can substitute a fake backend (to exercise cache misses and transient
	// backend failures) without a live GCS connection.
	newReader func(ctx context.Context, key string) (io.ReadCloser, error)

	// deleteObject removes the object identified by key. When nil, objects are
	// deleted via Bucket. It exists as a field so tests can exercise the purge
	// paths (missing object, backend failure) without a live GCS connection.
	deleteObject func(ctx context.Context, key string) error
}

// openReader opens the object named k and returns a reader over its contents. A
// missing object is reported as storage.ErrObjectNotExist (mapped to a clean
// cache miss by Get); any other error is a backend failure and is surfaced to
// callers rather than being swallowed as a miss.
func (gcsCache *GCSCache) openReader(ctx context.Context, k string) (io.ReadCloser, error) {
	if gcsCache.newReader != nil {
		return gcsCache.newReader(ctx, k)
	}
	return gcsCache.Bucket.Object(k).NewReader(ctx)
}

// removeObject deletes the object named k. Deleting an object that does not
// exist reports storage.ErrObjectNotExist (the client maps both HTTP 404 and
// gRPC NotFound onto it); any other error is a backend failure and is surfaced
// to callers.
func (gcsCache *GCSCache) removeObject(ctx context.Context, k string) error {
	if gcsCache.deleteObject != nil {
		return gcsCache.deleteObject(ctx, k)
	}
	return gcsCache.Bucket.Object(k).Delete(ctx)
}

func (gcsCache *GCSCache) Get(ctx context.Context, key *cache.Key) ([]byte, bool, error) {
	// object keys always use forward slashes regardless of the OS
	// separator (P6-33)
	k := path.Join(gcsCache.Basepath, key.String())

	r, err := gcsCache.openReader(ctx, k)
	switch {
	case errors.Is(err, storage.ErrObjectNotExist):
		// the object is not in the bucket: a clean cache miss.
		return nil, false, nil
	case err != nil:
		// a backend/read failure is not a miss: surface the error so callers can
		// distinguish "nothing cached" from "cache backend unavailable".
		// (ported from upstream go-spatial/tegola#938)
		return nil, false, err
	}
	defer func() { _ = r.Close() }()

	val, err := io.ReadAll(r)
	if err != nil {
		return nil, false, err
	}

	log.Infof("GET %s: %d bytes\n", k, len(val))

	return val, true, nil
}

func (gcsCache *GCSCache) Set(ctx context.Context, key *cache.Key, val []byte) error {
	// object keys always use forward slashes regardless of the OS
	// separator (P6-33)
	k := path.Join(gcsCache.Basepath, key.String())
	obj := gcsCache.Bucket.Object(k)

	// check for maxzoom
	if key.Z > gcsCache.MaxZoom {
		return nil
	}

	w := obj.NewWriter(ctx)
	if _, err := w.Write(val); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	log.Infof("SET %s: %d bytes\n", k, len(val))

	return nil
}

func (gcsCache *GCSCache) Purge(ctx context.Context, key *cache.Key) error {
	// object keys always use forward slashes regardless of the OS
	// separator (P6-33)
	k := path.Join(gcsCache.Basepath, key.String())

	if err := gcsCache.removeObject(ctx, k); err != nil {
		// purging an object that is already gone is a no-op success so Purge
		// stays idempotent (P5-13)
		if !errors.Is(err, storage.ErrObjectNotExist) {
			return err
		}
	}

	log.Infof("PURGE %s\n", k)

	return nil
}
