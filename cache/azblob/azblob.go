package azblob

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/Azure/azure-storage-blob-go/2016-05-31/azblob"

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
	BlobHeaderLen = 8 // bytes
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

	// seperate test for read only caches
	if azCache.ReadOnly {
		// read test file
		_, _, err := azCache.Get(&key)
		if err != nil {
			return nil, cache.ErrGettingFromCache{
				Err:       err,
				CacheType: CacheType,
			}
		}
	} else {
		// write test file
		err = azCache.Set(&key, []byte(testMsg))
		if err != nil {
			return nil, cache.ErrSettingToCache{
				Err:       err,
				CacheType: CacheType,
			}
		}

		// read test file
		byt, hit, err := azCache.Get(&key)
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

		err = azCache.Purge(&key)
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

func padBy512(n int) int32 {
	if n <= 0 {
		return 512
	}

	pad := n % 512
	if pad != 0 {
		pad = 512 - pad
	}

	return int32(n + pad)
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
	blobLen := padBy512(msgLen + BlobHeaderLen)

	// allocate blob
	blobSlice := make([]byte, blobLen)
	// encode the length of the blob
	binary.BigEndian.PutUint64(blobSlice[:BlobHeaderLen], uint64(msgLen))
	copy(blobSlice[BlobHeaderLen:], val)

	res, err := blob.Create(ctx, int64(blobLen), 0, httpHeaders, azblob.Metadata{}, azblob.BlobAccessConditions{})
	if err != nil {
		return err
	}

	// response body needs to be explicitly closed or
	// the socket will stay open
	res.Response().Body.Close()

	pageRange := azblob.PageRange{
		Start: 0,
		End:   blobLen - 1,
	}

	for ok := true; ok ; ok = len(blobSlice) > 0 {
		l := min(BlobReqMaxLen, len(blobSlice))

		_, err = blob.PutPages(ctx, pageRange, bytes.NewReader(blobSlice[:l]), azblob.BlobAccessConditions{})
		if err != nil {
			return err
		}

		blobSlice = blobSlice[l:]
	}

	return nil
}

func min(x, y int) int {
	if x < y {return x}
	return y
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
	msgLen := binary.BigEndian.Uint64(blobSlice[:BlobHeaderLen])

	// check for out of bounds
	if msgLen > uint64(len(blobSlice)-BlobHeaderLen) {
		return nil, false, fmt.Errorf("azblob: length section does not match message length")
	}

	return blobSlice[BlobHeaderLen:][:msgLen], true, nil
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
