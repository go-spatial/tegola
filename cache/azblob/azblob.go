package azblob

import (
	"path/filepath"
	"bytes"
	"net/http"
	"io/ioutil"
	"net/url"
	"context"
	"encoding/binary"
	"fmt"
	"strconv"

	"github.com/Azure/azure-storage-blob-go/2016-05-31/azblob"

	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola"
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

const testMsg = "\x41\x74\x6c\x61\x73\x20\x54\x65\x6c\x61\x6d\x6f\x6e"

func init() {
	cache.Register(CacheType, New)
}

func New(config dict.Dicter) (cache.Interface, error) {
	azCache := Cache{}

	// the config map's underlying value is int
	defaultMaxZoom := uint(tegola.MaxZ)
	maxZoom, err := config.Uint(ConfigKeyMaxZoom, &defaultMaxZoom)
	if err != nil {
		return nil, err
	}

	azCache.MaxZoom = maxZoom

	// basepath
	basePath := ""
	basePath, err = config.String(ConfigKeyBasepath, &basePath)
	if err != nil {
		return nil, err
	}

	azCache.Basepath = basePath

	readOnly := false
	readOnly, err = config.Bool(ConfigKeyReadOnly, &readOnly)
	if err != nil {
		return nil, err
	}

	azCache.ReadOnly = readOnly

	// credentials
	acctName := ""
	acctName, err = config.String(ConfigKeyAzureAccountName, &acctName)
	if err != nil {
		return nil, err
	}

	acctKey := ""
	acctKey, err = config.String(ConfigKeyAzureSharedKey, &acctKey)
	if err != nil {

	}

	var cred azblob.Credential

	if acctName+acctKey == "" {
		cred = azblob.NewAnonymousCredential()
		azCache.ReadOnly = true
	} else {
		if acctName == "" || acctKey == "" {
			return nil, fmt.Errorf("both %s and %s must be present", ConfigKeyAzureAccountName, ConfigKeyAzureSharedKey)
		}

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

	// seperate test for read only caches
	if azCache.ReadOnly {
		// read test file
		_, _, err := azCache.Get(&key)
		if err != nil {
			e := cache.ErrGettingFromCache{
				Err:       err,
				CacheType: CacheType,
			}

			return nil, e
		}
	} else {
		// write test file
		err = azCache.Set(&key, []byte(testMsg))
		if err != nil {
			e := cache.ErrSettingToCache{
				Err:       err,
				CacheType: CacheType,
			}

			return nil, e
		}

		// read test file
		byt, hit, err := azCache.Get(&key)
		if err != nil {
			e := cache.ErrGettingFromCache{
				Err:       err,
				CacheType: CacheType,
			}

			return nil, e
		}
		if !hit {
			//return an error?
			panic("no hit on writable cache")
		}

		// test response of writable cache
		if string(byt) != testMsg {
			e := cache.ErrGettingFromCache{
				Err:       fmt.Errorf("incrrect test response %s != %s", string(byt), testMsg),
				CacheType: CacheType,
			}

			return nil, e
		}

		err = azCache.Purge(&key)
		if err != nil {
			e := cache.ErrPurgingCache{
				Err:       err,
				CacheType: CacheType,
			}

			return nil, e
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

func roundUp512(n int) int32 {
	return int32(n + (512 - n%512))
}

func (azb *Cache) Set(key *cache.Key, val []byte) error {
	if key.Z > azb.MaxZoom || azb.ReadOnly {
		return nil
	}

	blob := azb.makeBlob(key).ToPageBlobURL()
	ctx := context.Background()

	httpHeaders := azblob.BlobHTTPHeaders{
		ContentType: "application/x-protobuf",
	}

	// must send things in multiples of 512 byte pages
	msgLen := len(val)
	blobLen := roundUp512(msgLen + 8)

	// allocate blob
	blobSlice := make([]byte, blobLen)
	// encode the length of the blob
	binary.BigEndian.PutUint64(blobSlice[:8], uint64(msgLen))
	copy(blobSlice[8:], val)

	_, err := blob.Create(ctx, int64(blobLen), 0, httpHeaders, azblob.Metadata{}, azblob.BlobAccessConditions{})
	if err != nil {
		println("\n\n ERR CREATE \n\n")
		return err
	}

	pageRange := azblob.PageRange{
		Start: 0,
		End:   blobLen - 1,
	}

	_, err = blob.PutPages(ctx, pageRange, bytes.NewReader(blobSlice), azblob.BlobAccessConditions{})
	if err != nil {
		println("\n\n ERR PUT \n\n")
		return err
	}

	return nil
}

func (azb *Cache) Get(key *cache.Key) ([]byte, bool, error) {
	if key.Z > azb.MaxZoom {
		return nil, false, nil
	}

	blob := azb.makeBlob(key)
	ctx := context.Background()

	res, err := blob.GetBlob(ctx, azblob.BlobRange{}, azblob.BlobAccessConditions{}, false)
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
	defer res.Body().Close()

	blobSlice, err := ioutil.ReadAll(res.Body())
	if err != nil {
		return nil, false, err
	}

	// get the encoded message length
	msgLen := binary.BigEndian.Uint64(blobSlice[:8])

	// check for out of bounds
	if msgLen > uint64(len(blobSlice)) {
		return nil, false, fmt.Errorf("azblob: length section does not match message length")
	}

	return blobSlice[8:][:msgLen], true, nil
}

func (azb *Cache) Purge(key *cache.Key) error {
	if azb.ReadOnly {
		return nil
	}

	blob := azb.makeBlob(key)
	ctx := context.Background()

	_, err := blob.Delete(ctx, azblob.DeleteSnapshotsOptionNone,
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
