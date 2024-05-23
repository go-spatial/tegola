# S3Cache

s3cache is an abstraction on top of Amazon Web Services (AWS) Simple Storage Service (S3) which implements the tegola cache interface. To use it, add the following minimum config to your tegola config file:

```toml
[cache]
type="s3"
bucket="tegola-test-data"
```

## Properties
The s3cache config supports the following properties:

- `bucket` (string): [Required] the name of the S3 bucket to use.
- `basepath` (string): [Optional] a path prefix added to all cache operations inside of the S3 bucket. helpful so a bucket does not need to be dedicated to only this cache.
- `region` (string): [Optional] the region the bucket is in. defaults to 'us-east-1'
- `aws_access_key_id` (string): [Optional] the AWS access key id to use.
- `aws_secret_access_key` (string): [Optional] the AWS secret access key to use.
- `max_zoom` (int): [Optional] the max zoom the cache should cache to. After this zoom, Set() calls will return before doing work.
- `endpoint` (string): [Optional] the endpoint where the S3 compliant backend is located. only necessary for non-AWS deployments. defaults to ''.
- `access_control_list` (string): [Optional] the S3 access control to set on the file when putting the file. defaults to ''.
- `cache_control` (string): [Optional] the HTTP cache control header to set on the file when putting the file. defaults to ''.
- `content_type` (string): [Optional] the http MIME-type set on the file when putting the file. defaults to 'application/vnd.mapbox-vector-tile'.
- `force_path_style` (bool): [Optional] use path-style addressing instead of virtual hosted-style addressing (i.e. http://s3.amazonaws.com/BUCKET/KEY instead of http://BUCKET.s3.amazonaws.com/KEY)
- `req_signing_host` (string): [Optional] force AWS request signing to use a different Host value, useful when `endpoint` is set to a a local proxy/sidecar.

## Credential chain
If the `aws_access_key_id` and `aws_secret_access_key` are not set, then the [credential provider chain](http://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html) will be used. The provider chain supports multiple methods for passing credentials, one of which is setting environment variables. For example:

```bash
$ export AWS_REGION=us-west-2
$ export AWS_ACCESS_KEY_ID=YOUR_AKID
$ export AWS_SECRET_ACCESS_KEY=YOUR_SECRET_KEY
```

## Testing
Testing is designed to work against a live S3 bucket. To run the s3 cache tests, the following environment variables need to be set:

```bash
$ export RUN_S3_TESTS=yes
$ export AWS_TEST_BUCKET=YOUR_TEST_BUCKET_NAME
$ export AWS_REGION=TEST_BUCKET_REGION
$ export AWS_ACCESS_KEY_ID=YOUR_AKID
$ export AWS_SECRET_ACCESS_KEY=YOUR_SECRET_KEY
```

## Use a local proxy or sidecar (in k8s)
If `endpoint` is set to a local reverse proxy (like `http://localhost:1234`), then AWS request signing will not work: the real S3 endpoint will return a HTTP 403 error saying:

```
SignatureDoesNotMatch: The request signature we calculated does not match the signature you provided. Check your key and signing method.
```

To make it work, the `req_signing_host` is a special parameter that forces Tegola to use a different HTTP Host header value when the AWS sdk signs the request to be sent to the real S3 endpoint. It needs to be set to the Host header (so no http:// prefixes etc..) of the real S3 endpoint (behind the reverse proxy for example).
