# AzBlobCache

azblob cache is an abstraction on top of Azure Blob Storage which implements the tegola cache interface. To use it, add the following minimum config to your tegola config file:

```toml
[cache]
type="azblob"
container_url="https://your-account.blob.core.windows.net/container-name"
az_account_name="your-account-name"
az_shared_key="your-shared-key
```

## Properties
The azblob config supports the following properties:

- `container_url` (string): [Required] the name of the S3 bucket to use.
- `basepath` (string): [Optional] a path prefix added to all cache operations inside of the S3 bucket. helpful so a bucket does not need to be dedicated to only this cache.
- `az_account_name` (string): [Optional] the storage account to use. `az_shared_key` must be specified with this property. If this is not set, Tegola will attempt an anonymous connection (requires a public blob conatiner) and treat the cache as read only.
- `az_shared_key` (string): [Optional] the storage account key to use.
- `max_zoom` (int): [Optional] the max zoom the cache should cache to. After this zoom, Set() calls will return before doing work.
- `read_only` (bool): [Optional] tegola will not write cache missed tiles into the cache. This setting is implicitly set to `true` if no account credentials are given.

## Testing
Testing is designed to work against a live Azure blob storage account. To run the azblob cache tests, the following environment variables need to be set:

```bash
$ export RUN_AZBLOB_TESTS=yes
$ export AZ_CONTAINER_URL='https://your-account.blob.core.windows.net/container-name'
$ export AZ_ACCOUNT_NAME='your-account'
$ export AZ_SHARED_KEY='your-key'
```

## Caveats
Due to the nature of azblob storage, the first 8 bytes of every cached file is a big-endian binary uint denoting the actual length of the tile.