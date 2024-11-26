package azblob

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/Azure/azure-storage-blob-go/2017-07-29/azblob"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/dict"
)

const CacheType = "azblob"

const (
	ConfigKeyBasepath         = "basepath"
	ConfigKeyMaxZoom          = "max_zoom"
	ConfigKeyReadOnly         = "read_only"
	ConfigKeyContainerUrl     = "container_url"
	ConfigKeyAzureAccountName = "az_account_name"
	ConfigKeyAzureSharedKey   = "az_shared_key"
)

const (
	DefaultBasepath    = ""
	DefaultAccountName = ""
	DefaultAccountKey  = ""
)

const (
	BlobHeaderLen = 8       // bytes
	BlobReqMaxLen = 4194304 // ~4MB
)

const testMsg = "\x41\x74\x6c\x61\x73\x20\x54\x65\x6c\x61\x6d\x6f\x6e"

func init() {
	cache.Register(CacheType, New)
}

func New(config dict.Dicter) (cache.Interface, error) {
	var err error
	azCache := Cache{}

	// the config map's underlying value is int
	maxZoom := uint(tegola.MaxZ)
	azCache.MaxZoom, err = config.Uint(ConfigKeyMaxZoom, &maxZoom)
	if err != nil {
		return nil, err
	}

	// basepath
	basePath := DefaultBasepath
	azCache.Basepath, err = config.String(ConfigKeyBasepath, &basePath)
	if err != nil {
		return nil, err
	}

	readOnly := false
	azCache.ReadOnly, err = config.Bool(ConfigKeyReadOnly, &readOnly)
	if err != nil {
		return nil, err
	}

	// credentials
	acctName := DefaultAccountName
	acctName, err = config.String(ConfigKeyAzureAccountName, &acctName)
	if err != nil {
		return nil, err
	}

	acctKey := DefaultAccountKey
	acctKey, err = config.String(ConfigKeyAzureSharedKey, &acctKey)
	if err != nil {
		return nil, err
	}

	var cred azblob.Credential

	if acctName+acctKey == "" {
		cred = azblob.NewAnonymousCredential()
		azCache.ReadOnly = true
	} else if acctName == "" || acctKey == "" {
		return nil, fmt.Errorf("both %s and %s must be present", ConfigKeyAzureAccountName, ConfigKeyAzureSharedKey)
	} else {
		cred = azblob.NewSharedKeyCredential(acctName, acctKey)
	}

	pipelineOpts := azblob.PipelineOptions{
		Telemetry: azblob.TelemetryOptions{
			Value: "tegola-cache",
		},
	}

	pipeline := azblob.NewPipeline(cred, pipelineOpts)

	// container
	uStr, err := config.String(ConfigKeyContainerUrl, nil)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(uStr)
	if err != nil {
		return nil, err
	}

	container := azblob.NewContainerURL(*u, pipeline)

	azCache.Container = container

	// in order to confirm we have the correct permissions on the container create a small file
	// and test a PUT, GET and DELETE to the container
	key := cache.Key{
		MapName:   "tegola-test-map",
		LayerName: "test-layer",
		Z:         0,
		X:         0,
		Y:         0,
	}

	ctx := context.Background()

	// seperate test for read only caches
	if azCache.ReadOnly {
		// read test file
		_, _, err := azCache.Get(ctx, &key)
		if err != nil {
			return nil, cache.ErrGettingFromCache{
				Err:       err,
				CacheType: CacheType,
			}
		}
	} else {
		// write test file
		err = azCache.Set(ctx, &key, []byte(testMsg))
		if err != nil {
			return nil, cache.ErrSettingToCache{
				Err:       err,
				CacheType: CacheType,
			}
		}

		// read test file
		byt, hit, err := azCache.Get(ctx, &key)
		if err != nil {
			return nil, cache.ErrGettingFromCache{
				Err:       err,
				CacheType: CacheType,
			}
		}
		if !hit {
			return nil, cache.ErrGettingFromCache{
				Err:       fmt.Errorf("no hit during test for key %s", key.String()),
				CacheType: CacheType,
			}
		}

		// test response of writable cache
		if string(byt) != testMsg {
			return nil, cache.ErrGettingFromCache{
				Err:       fmt.Errorf("incorrect test response %s != %s", string(byt), testMsg),
				CacheType: CacheType,
			}
		}

		err = azCache.Purge(ctx, &key)
		if err != nil {
			return nil, cache.ErrPurgingCache{
				Err:       err,
				CacheType: CacheType,
			}
		}
	}

	return &azCache, nil
}

type Cache struct {
	Basepath  string
	MaxZoom   uint
	ReadOnly  bool
	Container azblob.ContainerURL
}

func (azb *Cache) Set(ctx context.Context, key *cache.Key, val []byte) error {
	if key.Z > azb.MaxZoom || azb.ReadOnly {
		return nil
	}

	httpHeaders := azblob.BlobHTTPHeaders{
		ContentType: "application/x-protobuf",
	}

	res, err := azb.makeBlob(key).
		ToBlockBlobURL().
		Upload(ctx, bytes.NewReader(val), httpHeaders, azblob.Metadata{}, azblob.BlobAccessConditions{})

	if err != nil {
		return err
	}
	// response body needs to be explicitly closed or
	// the socket will stay open
	res.Response().Body.Close()

	return nil
}

func (azb *Cache) Get(ctx context.Context, key *cache.Key) ([]byte, bool, error) {
	if key.Z > azb.MaxZoom {
		return nil, false, nil
	}

	res, err := azb.makeBlob(key).
		ToBlockBlobURL().
		Download(ctx, 0, 0, azblob.BlobAccessConditions{}, false)

	if err != nil {
		// check if 404
		resErr, ok := err.(azblob.ResponseError)
		if ok {
			if resErr.Response().StatusCode == http.StatusNotFound {
				return nil, false, nil
			}
		}

		return nil, false, err
	}
	body := res.Body(azblob.RetryReaderOptions{})
	defer body.Close()

	blobSlice, err := io.ReadAll(body)
	if err != nil {
		return nil, false, err
	}

	return blobSlice, true, nil
}

func (azb *Cache) Purge(ctx context.Context, key *cache.Key) error {
	if azb.ReadOnly {
		return nil
	}

	_, err := azb.makeBlob(key).
		Delete(ctx, azblob.DeleteSnapshotsOptionNone,
			azblob.BlobAccessConditions{})

	if err != nil {
		return err
	}

	return nil
}

func (azb *Cache) makeBlob(key *cache.Key) azblob.BlobURL {
	k := filepath.Join(azb.Basepath, key.String())

	return azb.Container.NewBlobURL(k)
}
