package gcs

import (
	"context"
	"errors"
	"io"
	"path/filepath"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/log"

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
	cache.Register(CacheType, New)
}

func New(config dict.Dicter) (cache.Interface, error) {
	var err error

	gcsCache := GCSCache{}

	gcsCache.BucketName, err = config.String(ConfigKeyBucketName, nil)
	if err != nil {
		return nil, ErrMissingBucket
	}

	gcsCache.Basepath, err = config.String(ConfigKeyBasepath, nil)
	defaultMaxZoom := uint(tegola.MaxZ)
	gcsCache.MaxZoom, err = config.Uint(ConfigKeyMaxZoom, &defaultMaxZoom)

	gcsCache.Ctx = context.Background()
	client, err := storage.NewClient(gcsCache.Ctx)
	if err != nil {
		log.Fatal(err)
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

	// write gzip encoded test file
	if err := gcsCache.Set(&key, testData); err != nil {
		e := cache.ErrSettingToCache{
			CacheType: CacheType,
			Err:       err,
		}

		return nil, e
	}

	// read the test file
	_, hit, err := gcsCache.Get(&key)
	if err != nil {
		e := cache.ErrGettingFromCache{
			CacheType: CacheType,
			Err:       err,
		}

		return nil, e
	}
	if !hit {
		// return an error?
	}

	// purge the test file
	if err := gcsCache.Purge(&key); err != nil {
		e := cache.ErrPurgingCache{
			CacheType: CacheType,
			Err:       err,
		}

		return nil, e
	}

	return &gcsCache, nil
}

type GCSCache struct {

	// Context
	Ctx context.Context

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
}

func (gcsCache *GCSCache) Get(key *cache.Key) ([]byte, bool, error) {
	k := filepath.Join(gcsCache.Basepath, key.String())
	obj := gcsCache.Bucket.Object(k)

	r, err := obj.NewReader(gcsCache.Ctx)
	if err != nil {
		return nil, false, nil
	}
	defer r.Close()

	val, err := io.ReadAll(r)
	if err != nil {
		return nil, false, err
	}

	log.Infof("GET %s: %d bytes\n", k, len(val))

	return val, true, nil
}

func (gcsCache *GCSCache) Set(key *cache.Key, val []byte) error {
	k := filepath.Join(gcsCache.Basepath, key.String())
	obj := gcsCache.Bucket.Object(k)

	// check for maxzoom
	if key.Z > gcsCache.MaxZoom {
		return nil
	}

	w := obj.NewWriter(gcsCache.Ctx)
	if _, err := w.Write(val); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	log.Infof("SET %s: %d bytes\n", k, len(val))

	return nil
}

func (gcsCache *GCSCache) Purge(key *cache.Key) error {
	k := filepath.Join(gcsCache.Basepath, key.String())
	obj := gcsCache.Bucket.Object(k)

	if err := obj.Delete(gcsCache.Ctx); err != nil {
		return err
	}

	log.Infof("PURGE %s\n", k)

	return nil
}
